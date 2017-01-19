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
	crestTQ      = "https://crest-tq.eveonline.com"
	groupListURL = crestTQ + "/inventory/groups/"
	typeListURL  = crestTQ + "/inventory/types/"
)

var (
	groupFilter = flag.String("group", "", "")
)

func main() {
	flag.Parse()

	var err error
	var groupRE *regexp.Regexp

	if *groupFilter != "" {
		groupRE, err = regexp.Compile(*groupFilter)
		if err != nil {
			log.Fatalln(err)
		}
	}

	out := csv.NewWriter(os.Stdout)

	inv := GetInventoryList(groupRE)

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

func GetInventoryList(groupRE *regexp.Regexp) (ir *inventoryResult) {
	ich := make(chan inventoryListItem)
	errch := make(chan error, 1)
	ir = &inventoryResult{Items: ich, Err: errch}
	if groupRE == nil {
		go fetchAllItems(typeListURL, ich, errch)
	} else {
		go fetchItemsInGroup(groupRE, ich, errch)
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

func fetchItemsInGroup(groupRE *regexp.Regexp, ich chan<- inventoryListItem, errch chan<- error) {
	gch := make(chan inventoryListItem)
	gerrch := make(chan error, 1)
	go fetchAllItems(groupListURL, gch, gerrch)
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
