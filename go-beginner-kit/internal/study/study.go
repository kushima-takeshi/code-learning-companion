package study

import (
	"errors"
	"strings"
	"sync"
	"unicode/utf8"
)

var ErrNotFound = errors.New("record not found")

type Record struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Minutes int    `json:"minutes"`
}

func Validate(title string, minutes int) error {
	if strings.TrimSpace(title) == "" {
		return errors.New("title is required")
	}
	if utf8.RuneCountInString(title) > 100 {
		return errors.New("title must be at most 100 Unicode code points")
	}
	if minutes < 1 || minutes > 1440 {
		return errors.New("minutes must be between 1 and 1440")
	}
	return nil
}

// Storeはコピーせず、NewStoreが返すポインタを使う。
type Store struct {
	mu      sync.Mutex
	nextID  int
	records []Record
}

func NewStore() *Store {
	return &Store{nextID: 1, records: make([]Record, 0)}
}

func (s *Store) Add(title string, minutes int) (Record, error) {
	title = strings.TrimSpace(title)
	if err := Validate(title, minutes); err != nil {
		return Record{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := Record{ID: s.nextID, Title: title, Minutes: minutes}
	s.nextID++
	s.records = append(s.records, r)
	return r, nil
}

func (s *Store) List() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	// nilではなく空スライスを返すため、JSONはnullではなく[]になる。
	result := make([]Record, len(s.records))
	copy(result, s.records)
	return result
}

func (s *Store) Find(id int) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range s.records {
		if record.ID == id {
			return record, nil
		}
	}
	return Record{}, ErrNotFound
}
