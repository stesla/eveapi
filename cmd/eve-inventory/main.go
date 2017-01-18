package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

const CrestTQ = "https://crest-tq.eveonline.com"

func main() {
	inv := GetInventoryList(CrestTQ + "/inventory/groups/")

	for inv.NextPage() {
		if err := inv.Err(); err != nil {
			log.Fatalln(err)
		}
		log.Println(len(inv.Items), inv.TotalCount)
		for _, item := range inv.Items {
			fmt.Println(item.Id, item.Name)
		}
	}
}

type inventoryListItem struct {
	HRef string `json:"href"`
	Id   uint64 `json:"id"`
	Name string `json:"name"`
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

func GetInventoryList(url string) *inventoryList {
	return &inventoryList{Next: nextHRef{url}}
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
