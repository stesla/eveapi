package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
)

const (
	crestTQ         = "https://crest-tq.eveonline.com"
	categoryListURL = crestTQ + "/inventory/categories/"
	groupListURL    = crestTQ + "/inventory/groups/"
	typeListURL     = crestTQ + "/inventory/types/"
)

var (
	categoryFilter = flag.String("category", "", "")
	groupFilter    = flag.String("group", "", "")
)

func main() {
	flag.Parse()

	out := csv.NewWriter(os.Stdout)

	items := GetInventoryList(*categoryFilter, *groupFilter)

	for item := range items {
		if item.Err != nil {
			log.Fatalln(item.Err)
		}
		out.Write([]string{item.IdStr, item.Name})
	}
	out.Flush()
}

type inventoryListItem struct {
	HRef  string `json:"href"`
	Id    uint64 `json:"id"`
	IdStr string `json:"id_str"`
	Name  string `json:"name"`

	Err error
}

type inventoryList struct {
	PageCount  uint64              `json:"pageCount"`
	TotalCount uint64              `json:"totalCount"`
	Items      []inventoryListItem `json:"items"`
	Next       nextHRef            `json:"next"`

	err error
}

type nextHRef struct {
	HRef string `json:"href"`
}

func GetInventoryList(category, group string) <-chan inventoryListItem {
	ch := make(chan inventoryListItem)
	if category != "" && group != "" {
		go fetchItemsInCategoryAndGroup(categoryListURL, category, group, ch)
	} else if category == "" {
		go fetchItemsInGroup(groupListURL, group, ch)
	} else if group == "" {
		go fetchAllItemsInCategory(categoryListURL, category, ch)
	} else {
		go fetchAllItems(typeListURL, ch)
	}
	return ch
}

func fetchAllItems(url string, ch chan<- inventoryListItem) {
	il := &inventoryList{Next: nextHRef{url}}
	for il.NextPage() {
		if err := il.Err(); err != nil {
			ch <- inventoryListItem{Err: err}
			break
		}
		for _, item := range il.Items {
			ch <- item
		}
	}
	close(ch)
}

func fetchAllItemsInCategory(url string, category string, ch chan<- inventoryListItem) {
	lch := make(chan inventoryListItem)
	go fetchAllItems(url, lch)
loop:
	for item := range lch {
		if item.Err != nil {
			ch <- item
			break loop
		}
		if strings.EqualFold(category, item.Name) {
			category, err := fetchCategory(item.HRef)
			if err != nil {
				ch <- inventoryListItem{Err: err}
				break loop
			}
			if category.Published {
				for _, item := range category.Items {
					group, err := fetchGroup(item.HRef)
					if err != nil {
						ch <- inventoryListItem{Err: err}
						break loop
					}
					if group.Published {
						for _, item := range group.Items {
							ch <- item
						}
					}
				}
			}
		}
	}
	close(ch)
}

func fetchItemsInCategoryAndGroup(url string, category, group string, ch chan<- inventoryListItem) {
	lch := make(chan inventoryListItem)
	go fetchAllItems(url, lch)
loop:
	for catItem := range lch {
		if catItem.Err != nil {
			ch <- catItem
			break loop
		}
		if strings.EqualFold(category, catItem.Name) {
			category, err := fetchCategory(catItem.HRef)
			if err != nil {
				ch <- inventoryListItem{Err: err}
				break loop
			}
			if category.Published {
				for _, groupItem := range category.Items {
					if strings.EqualFold(group, groupItem.Name) {
						group, err := fetchGroup(groupItem.HRef)
						if err != nil {
							ch <- inventoryListItem{Err: err}
							break loop
						}
						if group.Published {
							for _, item := range group.Items {
								ch <- item
							}
						}
					}
				}
			}
		}
	}
	close(ch)
}

func fetchItemsInGroup(url string, group string, ch chan<- inventoryListItem) {
	lch := make(chan inventoryListItem)
	go fetchAllItems(url, lch)
loop:
	for item := range lch {
		if item.Err != nil {
			ch <- item
			break loop
		}
		if strings.EqualFold(group, item.Name) {
			group, err := fetchGroup(item.HRef)
			if err != nil {
				ch <- inventoryListItem{Err: err}
				break loop
			}
			if group.Published {
				for _, item := range group.Items {
					ch <- item
				}
			}
		}
	}
	close(ch)
}

func (inv *inventoryList) fetch(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(inv)
}

func (inv *inventoryList) Err() error {
	return inv.err
}

func (inv *inventoryList) NextPage() (result bool) {
	href := inv.Next.HRef
	if result = href != ""; !result {
		return
	}
	*inv = inventoryList{}
	inv.err = inv.fetch(href)
	return
}

type InventoryGroup struct {
	Published bool                `json:"published"`
	Items     []inventoryListItem `json:"types"`
}

func fetchGroup(url string) (result InventoryGroup, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&result)
	return
}

type InventoryCategory struct {
	Published bool                `json:"published"`
	Items     []inventoryListItem `json:"groups"`
}

func fetchCategory(url string) (result InventoryCategory, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&result)
	return
}
