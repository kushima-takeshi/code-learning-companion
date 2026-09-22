package main

import (
	"context"
	"fmt"
)

func sum(ctx context.Context, values []int, result chan<- int) {
	total := 0
	for _, value := range values {
		select {
		case <-ctx.Done():
			return
		default:
		}
		total += value
	}
	select {
	case result <- total:
	case <-ctx.Done():
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan int)
	go sum(ctx, []int{20, 30, 10}, result)
	fmt.Println("合計:", <-result)
}
