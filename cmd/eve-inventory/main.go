package main

import (
	"encoding/json"
	"log"
	"net/http"
)

const CrestTQ = "https://crest-tq.eveonline.com"

func main() {
	resp, err := http.Get(CrestTQ + "/inventory/categories/")
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	var inv InventoryList
	err = json.NewDecoder(resp.Body).Decode(&inv)
}

type InventoryListItem struct {
	HRef string `json:"href"`
	Id   uint64 `json:"id"`
	Name string `json:"name"`
}

type InventoryList struct {
	PageCount  uint64              `json:"pageCount"`
	TotalCount uint64              `json:"totalCount"`
	Items      []InventoryListItem `json:"items"`

	Next struct {
		HRef string `json:"href"`
	} `json:"next"`
}
