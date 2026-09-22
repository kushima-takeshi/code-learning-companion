package answers

import "testing"

func TestLevel(t *testing.T) {
	for _, tt := range []struct{ input, want int }{{-1, 0}, {0, 0}, {1, 1}, {59, 1}, {60, 2}} {
		if got := Level(tt.input); got != tt.want {
			t.Errorf("Level(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestAverage(t *testing.T) {
	got, err := Average([]int{10, 11})
	if err != nil || got != 10.5 {
		t.Fatalf("Average = %v, %v", got, err)
	}
	if _, err := Average(nil); err == nil {
		t.Fatal("empty input must fail")
	}
}

func TestTotals(t *testing.T) {
	input := []Record{{" Go ", 20}, {"SQL", 10}, {"Go", 30}}
	got := Totals(input)
	if len(got) != 2 || got["Go"] != 50 || got["SQL"] != 10 || input[0].Title != " Go " {
		t.Fatalf("Totals = %#v, input = %#v", got, input)
	}
}

func TestSetMinutes(t *testing.T) {
	r := Record{Title: "Go", Minutes: 25}
	if err := r.SetMinutes(0); err == nil || r.Minutes != 25 {
		t.Fatal("invalid input changed record")
	}
	if err := r.SetMinutes(30); err != nil || r.Minutes != 30 {
		t.Fatal("valid input did not update record")
	}
}

type fakeReader struct{}

func (fakeReader) List() []Record {
	return []Record{{Title: "Go", Minutes: 20}, {Title: "SQL", Minutes: 30}}
}

func TestTotalFrom(t *testing.T) {
	if got := TotalFrom(fakeReader{}); got != 50 {
		t.Fatalf("TotalFrom = %d, want 50", got)
	}
}
