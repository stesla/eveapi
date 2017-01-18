package main

import (
	"encoding/csv"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

const (
	crestTQ     = "https://crest-tq.eveonline.com"
	typeListURL = crestTQ + "/inventory/types/"
)

func main() {
	out := csv.NewWriter(os.Stdout)

	inv := GetInventoryList(typeListURL)

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

func GetInventoryList(url string) (ir *inventoryResult) {
	ich := make(chan inventoryListItem)
	errch := make(chan error, 1)
	ir = &inventoryResult{Items: ich, Err: errch}
	il := &inventoryList{Next: nextHRef{url}}
	go func() {
		for il.NextPage() {
			if err := il.Err(); err != nil {
				errch <- err
				close(ich)
				return
			}
			for _, item := range il.Items {
				ich <- item
			}
		}
		close(ich)
	}()
	return
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
