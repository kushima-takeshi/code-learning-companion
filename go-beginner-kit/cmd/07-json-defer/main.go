package main

import (
	"encoding/json"
	"fmt"
	"log"
)

type Record struct {
	Title   string `json:"title"`
	Minutes int    `json:"minutes"`
}

func main() {
	defer fmt.Println("最後に実行")
	defer fmt.Println("先に実行")
	r := Record{Title: "Go入門", Minutes: 25}
	data, err := json.Marshal(r)
	if err != nil {
		log.Print(err)
		return
	}
	fmt.Println(string(data))
	var decoded Record
	if err := json.Unmarshal(data, &decoded); err != nil {
		log.Print(err)
		return
	}
	fmt.Printf("復元: %s / %d分\n", decoded.Title, decoded.Minutes)
}
