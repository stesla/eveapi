package main

import (
	"encoding/csv"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

const CrestTQ = "https://crest-tq.eveonline.com"

func main() {
	out := csv.NewWriter(os.Stdout)

	inv := GetInventoryList(CrestTQ + "/inventory/types/")
	n := 0
	for inv.NextPage() {
		if err := inv.Err(); err != nil {
			log.Fatalln(err)
		}
		n += len(inv.Items)
		log.Println(n, inv.TotalCount)
		for _, item := range inv.Items {
			out.Write([]string{item.IdStr, item.Name})
		}
	}
	out.Flush()
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
