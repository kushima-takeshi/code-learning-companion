package answers

import (
	"errors"
	"strings"
)

// E1: 0以下は0、1〜59は1、60以上は2。
func Level(minutes int) int {
	if minutes <= 0 {
		return 0
	}
	if minutes < 60 {
		return 1
	}
	return 2
}

// E2: 空のスライスでは平均を定義せず、errorを返す。
func Average(values []int) (float64, error) {
	if len(values) == 0 {
		return 0, errors.New("values must not be empty")
	}
	total := 0
	for _, value := range values {
		total += value
	}
	return float64(total) / float64(len(values)), nil
}

type Record struct {
	Title   string
	Minutes int
}

// E3: 空白除去後のタイトルごとに合計する。元データは変えない。
func Totals(records []Record) map[string]int {
	result := make(map[string]int)
	for _, record := range records {
		result[strings.TrimSpace(record.Title)] += record.Minutes
	}
	return result
}

// E4: 不正な値なら元のMinutesを維持する。
func (r *Record) SetMinutes(minutes int) error {
	if minutes < 1 || minutes > 1440 {
		return errors.New("minutes must be between 1 and 1440")
	}
	r.Minutes = minutes
	return nil
}

// E5: MemoryReaderなど、Listメソッドを持つ型を受け取る。
type Reader interface {
	List() []Record
}

func TotalFrom(reader Reader) int {
	total := 0
	for _, record := range reader.List() {
		total += record.Minutes
	}
	return total
}
