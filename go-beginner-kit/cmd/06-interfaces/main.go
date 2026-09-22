package main

import "fmt"

type Record struct {
	Title string
}

// 呼び出す側が、必要な操作だけを定義する。
type RecordReader interface {
	List() []Record
}

type MemoryReader struct {
	records []Record
}

func (m MemoryReader) List() []Record {
	return append([]Record(nil), m.records...)
}

func printTitles(reader RecordReader) {
	for _, record := range reader.List() {
		fmt.Println(record.Title)
	}
}

func main() {
	reader := MemoryReader{records: []Record{{Title: "Go入門"}, {Title: "SQL復習"}}}
	printTitles(reader)
}
