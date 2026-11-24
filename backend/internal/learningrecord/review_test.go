package learningrecord

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/config"
)

func TestFirstReviewTiming(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	// The clock moves back one hour during these 72 hours.
	createdAt := time.Date(2026, 10, 31, 16, 35, 12, 123456789, location)
	deadline := time.Date(2026, 11, 3, 15, 35, 12, 123456789, location)
	if got := firstReviewDueAt(createdAt); !got.Equal(deadline) {
		t.Fatalf("due at = %s, want %s", got, deadline)
	}
}

func TestWebDueReviewsHTTPIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	owner, other, empty := listUser(t, db, 1), listUser(t, db, 2), listUser(t, db, 3)
	now := time.Date(2026, 11, 27, 7, 35, 12, 123456000, time.UTC)
	service := New(db)
	service.now = func() time.Time { return now }
	login := auth.NewHandler(config.Config{}, auth.NewStore(db))
	mux := http.NewServeMux()
	if err := NewHandler(service).RegisterWebRoutes(mux, login.RequireAPIAuth, "https://app.example"); err != nil {
		t.Fatal(err)
	}

	insert := func(id, userID string, createdAt time.Time) {
		t.Helper()
		_, err := db.Exec(`
			INSERT INTO learning_records (id, user_id, request_id, theme, questions, created_at)
			VALUES ($1, $2, $3, 'Go', '[]', $4)
		`, id, userID, uuid.NewString(), createdAt)
		if err != nil {
			t.Fatal(err)
		}
	}
	atBoundary := now.Add(-72 * time.Hour)
	const (
		lowerID   = "00000000-0000-4000-8000-000000000001"
		higherID  = "00000000-0000-4000-8000-000000000002"
		overdueID = "00000000-0000-4000-8000-000000000003"
	)
	insert(lowerID, owner, atBoundary)
	insert(higherID, owner, atBoundary)
	insert(overdueID, owner, atBoundary.Add(-time.Hour))
	insert(uuid.NewString(), owner, atBoundary.Add(time.Microsecond))
	insert(uuid.NewString(), other, atBoundary)

	const path = "/web/reviews/due"
	checkListError(t, listRequest(mux, http.MethodGet, path, "", nil), http.StatusUnauthorized, "unauthorized")

	type response struct {
		Records []dueReview `json:"records"`
	}
	expires := time.Now().Add(time.Hour)
	cookie := learningSession(t, db, owner, expires)
	got := decodeWebRecordJSON[response](t, listRequest(mux, http.MethodGet, path, "", cookie))
	want := []dueReview{
		{ID: overdueID, Stage: 1, DueAt: now.Add(-time.Hour)},
		{ID: lowerID, Stage: 1, DueAt: now},
		{ID: higherID, Stage: 1, DueAt: now},
	}
	if len(got.Records) != len(want) {
		t.Fatalf("records = %+v, want %+v", got.Records, want)
	}
	for i, record := range got.Records {
		if record.ID != want[i].ID || record.Stage != want[i].Stage || !record.DueAt.Equal(want[i].DueAt) {
			t.Errorf("records[%d] = %+v, want %+v", i, record, want[i])
		}
	}
	emptyCookie := learningSession(t, db, empty, expires)
	got = decodeWebRecordJSON[response](t, listRequest(mux, http.MethodGet, path, "", emptyCookie))
	if got.Records == nil || len(got.Records) != 0 {
		t.Fatalf("empty records = %#v, want []", got.Records)
	}
}
