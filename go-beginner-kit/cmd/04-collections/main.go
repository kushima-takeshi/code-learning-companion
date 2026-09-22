package main

import "fmt"

func main() {
	minutes := []int{20, 30}
	minutes = append(minutes, 10)
	total := 0
	for _, value := range minutes {
		total += value
	}
	fmt.Println("合計:", total)
	byTopic := map[string]int{"Go": 60, "SQL": 0}
	value, ok := byTopic["SQL"]
	fmt.Println("SQL:", value, ok)
	value, ok = byTopic["Java"]
	fmt.Println("Java:", value, ok)

	// スライスを代入しても、要素を保存する配列は共有される。
	a := []int{10, 20}
	b := a
	b[0] = 99
	fmt.Println("共有:", a)
	c := append([]int(nil), a...)
	c[0] = 1
	fmt.Println("コピー:", a, c)
}
