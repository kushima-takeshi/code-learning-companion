package study

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		minutes int
		wantErr bool
	}{
		{"normal", "Go", 25, false},
		{"empty title", "", 25, true},
		{"blank title", " \t", 25, true},
		{"100 code points", strings.Repeat("あ", 100), 1, false},
		{"101 code points", strings.Repeat("あ", 101), 1, true},
		{"negative", "Go", -1, true},
		{"zero", "Go", 0, true},
		{"minimum", "Go", 1, false},
		{"maximum", "Go", 1440, false},
		{"too large", "Go", 1441, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.title, tt.minutes)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStore(t *testing.T) {
	s := NewStore()
	if got := s.List(); got == nil || len(got) != 0 {
		t.Fatalf("new List() = %#v", got)
	}
	if _, err := s.Add("", 10); err == nil {
		t.Fatal("empty title was accepted")
	}
	r, err := s.Add(" Go ", 25)
	if err != nil || r.ID != 1 || r.Title != "Go" {
		t.Fatalf("Add() = %#v, %v", r, err)
	}
	list := s.List()
	list[0].Title = "changed"
	found, err := s.Find(1)
	if err != nil || found.Title != "Go" {
		t.Fatalf("List leaked mutable storage: %#v, %v", found, err)
	}
	if _, err := s.Find(999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Find(999) error = %v", err)
	}
}

func TestConcurrentAdds(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Add("Go", 1); err != nil {
				t.Error(err)
			}
			_ = s.List()
		}()
	}
	wg.Wait()
	list := s.List()
	if len(list) != 30 {
		t.Fatalf("got %d records", len(list))
	}
	for i, record := range list {
		if record.ID != i+1 {
			t.Fatalf("record %d ID = %d", i, record.ID)
		}
	}
}
