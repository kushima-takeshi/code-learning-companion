package main

import (
	"fmt"
	"strconv"
)

func parseMinutes(text string) (int, error) {
	minutes, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("minutes must be an integer: %w", err)
	}
	if minutes <= 0 {
		return 0, fmt.Errorf("minutes must be positive")
	}
	return minutes, nil
}

func main() {
	for _, text := range []string{"25", "0", "abc"} {
		minutes, err := parseMinutes(text)
		if err != nil {
			fmt.Printf("入力 %q: エラー\n", text)
			continue
		}
		fmt.Printf("入力 %q: %d分\n", text, minutes)
	}
}
