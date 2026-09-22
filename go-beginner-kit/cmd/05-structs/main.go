package main

import "fmt"

type Record struct {
	Title   string
	Minutes int
}

func (r Record) Summary() string {
	return fmt.Sprintf("%s: %d分", r.Title, r.Minutes)
}

func (r *Record) AddMinutes(minutes int) {
	r.Minutes += minutes
}

func changeCopy(r Record) {
	r.Minutes = 999
}

func main() {
	r := Record{Title: "Go入門", Minutes: 20}
	changeCopy(r)
	fmt.Println(r.Summary())
	r.AddMinutes(10)
	fmt.Println(r.Summary())
	p := &r
	p.Minutes = 40
	fmt.Println(r.Summary())
}
