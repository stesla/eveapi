package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"regexp"
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

	var err error
	var categoryRE, groupRE *regexp.Regexp

	if *categoryFilter != "" {
		categoryRE, err = regexp.Compile(*categoryFilter)
		if err != nil {
			log.Fatalln(err)
		}
	}

	if *groupFilter != "" {
		groupRE, err = regexp.Compile(*groupFilter)
		if err != nil {
			log.Fatalln(err)
		}
	}

	out := csv.NewWriter(os.Stdout)

	inv := GetInventoryList(categoryRE, groupRE)

loop:
	for {
		select {
		case item, open := <-inv.Items:
			if !open {
				break loop
			}
			out.Write([]string{item.IdStr, item.Name})
		case err := <-inv.Err:
			log.Fatalln(err)
		}
	}
	out.Flush()
}

type inventoryResult struct {
	Items <-chan inventoryListItem
	Err   <-chan error
}

type inventoryListItem struct {
	HRef  string `json:"href"`
	Id    uint64 `json:"id"`
	IdStr string `json:"id_str"`
	Name  string `json:"name"`
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

func GetInventoryList(categoryRE, groupRE *regexp.Regexp) (ir *inventoryResult) {
	ich := make(chan inventoryListItem)
	errch := make(chan error, 1)
	ir = &inventoryResult{Items: ich, Err: errch}
	if categoryRE != nil && groupRE != nil {
		go fetchItemsInCategoryAndGroup(categoryListURL, categoryRE, groupRE, ich, errch)
	} else if categoryRE == nil {
		go fetchItemsInGroup(groupListURL, groupRE, ich, errch)
	} else if groupRE == nil {
		go fetchAllItemsInCategory(categoryListURL, categoryRE, ich, errch)
	} else {
		go fetchAllItems(typeListURL, ich, errch)
	}
	return
}

func fetchAllItems(url string, ich chan<- inventoryListItem, errch chan<- error) {
	il := &inventoryList{Next: nextHRef{url}}
loop:
	for il.NextPage() {
		if err := il.Err(); err != nil {
			errch <- err
			break loop
		}
		for _, item := range il.Items {
			ich <- item
		}
	}
	close(ich)
}

func fetchAllItemsInCategory(url string, categoryRE *regexp.Regexp, ich chan<- inventoryListItem, errch chan<- error) {
	lch := make(chan inventoryListItem)
	lerrch := make(chan error, 1)
	go fetchAllItems(url, lch, lerrch)
loop:
	for {
		select {
		case item, open := <-lch:
			if !open {
				break loop
			}
			if categoryRE.MatchString(item.Name) {
				category, err := fetchCategory(item.HRef)
				if err != nil {
					errch <- err
					break loop
				}
				if category.Published {
					for _, item := range category.Items {
						group, err := fetchGroup(item.HRef)
						if err != nil {
							errch <- err
							break loop
						}
						if group.Published {
							for _, item := range group.Items {
								ich <- item
							}
						}
					}
				}
			}
		case err := <-lerrch:
			errch <- err
			break loop
		}
	}
	close(ich)
}

func fetchItemsInCategoryAndGroup(url string, categoryRE, groupRE *regexp.Regexp, ich chan<- inventoryListItem, errch chan<- error) {
	lch := make(chan inventoryListItem)
	lerrch := make(chan error, 1)
	go fetchAllItems(url, lch, lerrch)
loop:
	for {
		select {
		case item, open := <-lch:
			if !open {
				break loop
			}
			if categoryRE.MatchString(item.Name) {
				category, err := fetchCategory(item.HRef)
				if err != nil {
					errch <- err
					break loop
				}
				if category.Published {
					for _, item := range category.Items {
						if groupRE.MatchString(item.Name) {
							group, err := fetchGroup(item.HRef)
							if err != nil {
								errch <- err
								break loop
							}
							if group.Published {
								for _, item := range group.Items {
									ich <- item
								}
							}
						}
					}
				}
			}
		case err := <-lerrch:
			errch <- err
			break loop
		}
	}
	close(ich)
}

func fetchItemsInGroup(url string, groupRE *regexp.Regexp, ich chan<- inventoryListItem, errch chan<- error) {
	gch := make(chan inventoryListItem)
	gerrch := make(chan error, 1)
	go fetchAllItems(url, gch, gerrch)
loop:
	for {
		select {
		case item, open := <-gch:
			if !open {
				break loop
			}
			if groupRE.MatchString(item.Name) {
				group, err := fetchGroup(item.HRef)
				if err != nil {
					errch <- err
					break loop
				}
				if group.Published {
					for _, item := range group.Items {
						ich <- item
					}
				}
			}
		case err := <-gerrch:
			errch <- err
			break loop
		}
	}
	close(ich)
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
