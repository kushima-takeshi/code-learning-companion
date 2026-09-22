package main

import "fmt"

func main() {
	title := "Go入門"
	var minutes int
	minutes = 25
	const target = 60
	fmt.Printf("%s: %d分\n", title, minutes)
	if minutes >= target {
		fmt.Println("目標達成")
	} else {
		fmt.Println("学習中")
	}
	total := 0
	for day := 1; day <= 3; day++ {
		total += 20
	}
	fmt.Printf("合計: %d分\n", total)
	fmt.Printf("平均: %.1f分\n", float64(total)/3)
}
