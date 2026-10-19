package primport

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

type queryRows struct {
	values        [][]driver.Value
	index         int
	terminalError error
	closed        bool
}

func (r *queryRows) Columns() []string {
	return []string{"id", "html_url", "owner", "repo", "pr_number", "title", "author_login", "state", "created_at"}
}
func (r *queryRows) Close() error { r.closed = true; return nil }
func (r *queryRows) Next(dest []driver.Value) error {
	if r.index == len(r.values) {
		if r.terminalError != nil {
			return r.terminalError
		}
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
func summaryValues(s SnapshotSummary) []driver.Value {
	return []driver.Value{s.ID, s.HTMLURL, s.Owner, s.Repo, int64(s.PRNumber), s.Title, s.AuthorLogin, s.State, s.CreatedAt}
}

func TestListUsesUserFilterAndReturnsSummaries(t *testing.T) {
	s, fetcher, store, db := testService(t)
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	want := []SnapshotSummary{
		{ID: "new-id", HTMLURL: "https://github.com/owner/new/pull/9", Owner: "owner", Repo: "new", PRNumber: 9, Title: "New PR", AuthorLogin: "alice", State: "open", CreatedAt: now},
		{ID: "old-id", HTMLURL: "https://github.com/other/old/pull/3", Owner: "other", Repo: "old", PRNumber: 3, Title: "Old PR", AuthorLogin: "bob", State: "closed", CreatedAt: now.Add(-time.Hour)},
	}
	calls := 0
	var returnedRows []*queryRows
	db.query = func(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
		calls++
		normalized := strings.Join(strings.Fields(query), " ")
		expected := "SELECT id, html_url, owner, repo, pr_number, title, author_login, state, created_at FROM pr_snapshots WHERE user_id = $1 ORDER BY created_at DESC"
		if normalized != expected {
			t.Errorf("unexpected SQL: %s", normalized)
		}
		if len(args) != 1 || args[0].Value != []string{"user-a", "user-b"}[calls-1] {
			t.Fatalf("wrong user arguments: %+v", args)
		}
		rows := &queryRows{values: [][]driver.Value{summaryValues(want[0]), summaryValues(want[1])}}
		returnedRows = append(returnedRows, rows)
		return rows, nil
	}
	for _, userID := range []string{"user-a", "user-b"} {
		got, err := s.List(context.Background(), userID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("summaries=%+v, want %+v", got, want)
		}
	}
	if calls != 2 {
		t.Fatalf("query calls=%d", calls)
	}
	for _, rows := range returnedRows {
		if !rows.closed {
			t.Fatal("rows not closed")
		}
	}
	if fetcher.calls != 0 || len(store.puts) != 0 || len(store.deletes) != 0 || db.begins != 0 {
		t.Fatal("List caused unrelated I/O")
	}
}

func TestListEmptyReturnsJSONArray(t *testing.T) {
	s, _, _, db := testService(t)
	rows := &queryRows{}
	db.query = func(context.Context, string, []driver.NamedValue) (driver.Rows, error) { return rows, nil }
	got, err := s.List(context.Background(), "user")
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || string(body) != "[]" {
		t.Fatalf("empty list JSON=%s", body)
	}
	if !rows.closed {
		t.Fatal("rows not closed")
	}
}

func TestListQueryFailure(t *testing.T) {
	s, _, _, db := testService(t)
	db.query = func(context.Context, string, []driver.NamedValue) (driver.Rows, error) { return nil, errInjected }
	got, err := s.List(context.Background(), "user")
	if got != nil || !errors.Is(err, errInjected) {
		t.Fatalf("result=%+v error=%v", got, err)
	}
}

func TestListScanFailureClosesRows(t *testing.T) {
	s, _, _, db := testService(t)
	values := summaryValues(SnapshotSummary{ID: "id", CreatedAt: time.Now()})
	values[4] = "not-an-integer"
	rows := &queryRows{values: [][]driver.Value{values}}
	db.query = func(context.Context, string, []driver.NamedValue) (driver.Rows, error) { return rows, nil }
	got, err := s.List(context.Background(), "user")
	if got != nil || err == nil || !strings.Contains(err.Error(), "scan PR snapshot") {
		t.Fatalf("result=%+v error=%v", got, err)
	}
	if !rows.closed {
		t.Fatal("rows not closed after Scan error")
	}
}

func TestListReadFailureDiscardsPartialResults(t *testing.T) {
	s, _, _, db := testService(t)
	rows := &queryRows{values: [][]driver.Value{summaryValues(SnapshotSummary{ID: "first", CreatedAt: time.Now()})}, terminalError: errInjected}
	db.query = func(context.Context, string, []driver.NamedValue) (driver.Rows, error) { return rows, nil }
	got, err := s.List(context.Background(), "user")
	if got != nil || !errors.Is(err, errInjected) {
		t.Fatalf("result=%+v error=%v", got, err)
	}
	if !rows.closed {
		t.Fatal("rows not closed after read error")
	}
}

func TestGetUsesSnapshotAndUserFilter(t *testing.T) {
	s, fetcher, store, db := testService(t)
	want := SnapshotSummary{
		ID: "snapshot-id", HTMLURL: "https://github.com/owner/repo/pull/7",
		Owner: "owner", Repo: "repo", PRNumber: 7, Title: "PR title",
		AuthorLogin: "author", State: "open",
		CreatedAt: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
	}
	rows := &queryRows{values: [][]driver.Value{summaryValues(want)}}
	calls := 0
	db.query = func(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
		calls++
		expected := "SELECT id, html_url, owner, repo, pr_number, title, author_login, state, created_at FROM pr_snapshots WHERE id = $1 AND user_id = $2"
		if normalized := strings.Join(strings.Fields(query), " "); normalized != expected {
			t.Errorf("unexpected SQL: %s", normalized)
		}
		if len(args) != 2 || args[0].Value != want.ID || args[1].Value != "owner-user-id" {
			t.Fatalf("wrong snapshot/user arguments: %+v", args)
		}
		return rows, nil
	}
	got, err := s.Get(context.Background(), "owner-user-id", want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || !reflect.DeepEqual(*got, want) {
		t.Fatalf("snapshot=%+v, want %+v", got, want)
	}
	if calls != 1 || !rows.closed {
		t.Fatalf("calls=%d, rows closed=%t", calls, rows.closed)
	}
	if fetcher.calls != 0 || len(store.puts) != 0 || len(store.deletes) != 0 || db.begins != 0 {
		t.Fatal("Get caused unrelated I/O")
	}
}

func TestGetNoRowsReturnsNotFound(t *testing.T) {
	s, _, _, db := testService(t)
	rows := &queryRows{}
	db.query = func(context.Context, string, []driver.NamedValue) (driver.Rows, error) { return rows, nil }
	got, err := s.Get(context.Background(), "user", "missing-snapshot")
	if got != nil || !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("result=%+v error=%v", got, err)
	}
	if !rows.closed {
		t.Fatal("rows not closed")
	}
}

func TestGetPassesRequestingUserForExistingSnapshot(t *testing.T) {
	s, _, _, db := testService(t)
	rows := &queryRows{}
	db.query = func(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
		if !strings.Contains(strings.Join(strings.Fields(query), " "), "WHERE id = $1 AND user_id = $2") {
			t.Fatal("missing owner restriction")
		}
		if len(args) != 2 || args[0].Value != "existing-snapshot" || args[1].Value != "other-user" {
			t.Fatalf("wrong access scope: %+v", args)
		}
		return rows, nil
	}
	got, err := s.Get(context.Background(), "other-user", "existing-snapshot")
	if got != nil || !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("result=%+v error=%v", got, err)
	}
	if !rows.closed {
		t.Fatal("rows not closed")
	}
}

func TestGetQueryFailureIsNotNotFound(t *testing.T) {
	s, _, _, db := testService(t)
	db.query = func(context.Context, string, []driver.NamedValue) (driver.Rows, error) { return nil, errInjected }
	got, err := s.Get(context.Background(), "user", "snapshot")
	if got != nil || !errors.Is(err, errInjected) || errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("result=%+v error=%v", got, err)
	}
}

func TestGetScanFailureClosesRows(t *testing.T) {
	s, _, _, db := testService(t)
	values := summaryValues(SnapshotSummary{ID: "snapshot", CreatedAt: time.Now()})
	values[4] = "not-an-integer"
	rows := &queryRows{values: [][]driver.Value{values}}
	db.query = func(context.Context, string, []driver.NamedValue) (driver.Rows, error) { return rows, nil }
	got, err := s.Get(context.Background(), "user", "snapshot")
	if got != nil || err == nil || errors.Is(err, ErrSnapshotNotFound) || !strings.Contains(err.Error(), "get PR snapshot") {
		t.Fatalf("result=%+v error=%v", got, err)
	}
	if !rows.closed {
		t.Fatal("rows not closed after scan failure")
	}
}

func TestGetReadFailureIsNotNotFound(t *testing.T) {
	s, _, _, db := testService(t)
	rows := &queryRows{terminalError: errInjected}
	db.query = func(context.Context, string, []driver.NamedValue) (driver.Rows, error) { return rows, nil }
	got, err := s.Get(context.Background(), "user", "snapshot")
	if got != nil || !errors.Is(err, errInjected) || errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("result=%+v error=%v", got, err)
	}
	if !rows.closed {
		t.Fatal("rows not closed after read failure")
	}
}
