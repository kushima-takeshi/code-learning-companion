package learningrecord

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestListRejectsInvalidPaginationBeforeDB(t *testing.T) {
	for _, tc := range []struct{ limit, offset int }{{0, 0}, {-1, 0}, {101, 0}, {20, -1}} {
		page, err := New(nil).List(context.Background(), uuid.NewString(), tc.limit, tc.offset)
		if !errors.Is(err, ErrInvalidPagination) || !reflect.DeepEqual(page, ListPage{}) {
			t.Fatalf("invalid pagination: page=%+v err=%v", page, err)
		}
	}
}

func TestListIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	ctx := context.Background()
	owner, other := uuid.NewString(), uuid.NewString()
	for i, id := range []string{owner, other} {
		if _, err := db.ExecContext(ctx, `INSERT INTO users (id, github_user_id, github_login) VALUES ($1, $2, $3)`, id, i+1, id); err != nil {
			t.Fatal(err)
		}
	}
	service := New(db)
	page, err := service.List(ctx, owner, 20, 0)
	if err != nil || page.Records == nil || len(page.Records) != 0 || page.HasMore {
		t.Fatalf("empty page: %+v %v", page, err)
	}
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	// Insert IDs out of sort order and include another owner's newer record.
	ids := []string{
		"00000000-0000-4000-8000-000000000002",
		"00000000-0000-4000-8000-000000000003",
		"00000000-0000-4000-8000-000000000001",
	}
	for i, id := range ids {
		created := base
		if i == 2 {
			created = base.Add(-time.Hour)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO learning_records (id, user_id, request_id, theme, questions, created_at) VALUES ($1, $2, $3, $4, '[]', $5)`, id, owner, uuid.NewString(), "theme-"+id, created); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO learning_records (user_id, request_id, theme, questions, created_at) VALUES ($1, $2, 'other-private', '[]', $3)`, other, uuid.NewString(), base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	page, err = service.List(ctx, owner, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []RecordSummary{
		{ID: ids[1], Theme: "theme-" + ids[1], CreatedAt: base},
		{ID: ids[0], Theme: "theme-" + ids[0], CreatedAt: base},
	}
	if len(page.Records) != len(want) || !page.HasMore {
		t.Fatalf("first page: %+v, want %+v and has_more", page, want)
	}
	for i, record := range page.Records {
		if record.ID != want[i].ID || record.Theme != want[i].Theme || !record.CreatedAt.Equal(want[i].CreatedAt) {
			t.Fatalf("summary %d: %+v, want %+v", i, record, want[i])
		}
	}
	page, err = service.List(ctx, owner, 2, 2)
	if err != nil || len(page.Records) != 1 || page.Records[0].ID != ids[2] || page.HasMore {
		t.Fatalf("last page: %+v %v", page, err)
	}
	for _, tc := range []struct{ limit, offset int }{{3, 0}, {100, 0}, {20, 3}, {20, MaxListOffset}} {
		page, err := service.List(ctx, owner, tc.limit, tc.offset)
		if err != nil || page.HasMore || page.Records == nil {
			t.Fatalf("boundary %+v: %+v %v", tc, page, err)
		}
		if tc.offset >= 3 && len(page.Records) != 0 {
			t.Fatalf("past end: %+v", page)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	page, err = service.List(ctx, owner, 20, 0)
	if err == nil || !reflect.DeepEqual(page, ListPage{}) {
		t.Fatalf("DB failure must not return partial data: %+v %v", page, err)
	}
}
