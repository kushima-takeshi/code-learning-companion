package learningrecord

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/config"
)

const reviewMigration = "000008_learning_record_reviews.up.sql"

func TestFirstReviewTiming(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	db := newIntegrationDB(t, reviewMigration)
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("SET TIME ZONE 'America/New_York'"); err != nil {
		t.Fatal(err)
	}
	owner := listUser(t, db, 1)
	// The clock moves back one hour during these 72 hours.
	createdAt := time.Date(2026, 10, 31, 16, 35, 12, 123456000, location)
	deadline := time.Date(2026, 11, 3, 15, 35, 12, 123456000, location)
	id := uuid.NewString()
	insertReviewRecord(t, db, id, owner, createdAt)
	service := New(db)
	service.now = func() time.Time { return deadline }
	got, err := service.listDueReviews(t.Context(), owner)
	if err != nil || len(got) != 1 || got[0].ID != id || got[0].Stage != 1 || !got[0].DueAt.Equal(deadline) {
		t.Fatalf("due reviews = %+v, err = %v, want stage 1 due at %s", got, err, deadline)
	}
}

func newReviewHTTP(t *testing.T, db *sql.DB, clock func() time.Time) http.Handler {
	t.Helper()
	service := New(db)
	service.now = clock
	login := auth.NewHandler(config.Config{}, auth.NewStore(db))
	mux := http.NewServeMux()
	if err := NewHandler(service).RegisterWebRoutes(mux, login.RequireAPIAuth, "https://app.example"); err != nil {
		t.Fatal(err)
	}
	return mux
}

func insertReviewRecord(t *testing.T, db *sql.DB, id, userID string, createdAt time.Time) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO learning_records (id, user_id, request_id, theme, questions, created_at)
		VALUES ($1, $2, $3, 'Go', '[]', $4)
	`, id, userID, uuid.NewString(), createdAt)
	if err != nil {
		t.Fatal(err)
	}
}

type dueReviewsResponse struct {
	Records []dueReview `json:"records"`
}

const dueReviewsPath = "/web/reviews/due"

func TestWebDueReviewsHTTPIntegration(t *testing.T) {
	db := newIntegrationDB(t, reviewMigration)
	owner, other, empty := listUser(t, db, 1), listUser(t, db, 2), listUser(t, db, 3)
	now := time.Date(2026, 11, 27, 7, 35, 12, 123456000, time.UTC)
	mux := newReviewHTTP(t, db, func() time.Time { return now })

	atBoundary := now.Add(-72 * time.Hour)
	const (
		lowerID   = "00000000-0000-4000-8000-000000000001"
		higherID  = "00000000-0000-4000-8000-000000000002"
		overdueID = "00000000-0000-4000-8000-000000000003"
	)
	insertReviewRecord(t, db, lowerID, owner, atBoundary)
	insertReviewRecord(t, db, higherID, owner, atBoundary)
	insertReviewRecord(t, db, overdueID, owner, atBoundary.Add(-time.Hour))
	insertReviewRecord(t, db, uuid.NewString(), owner, atBoundary.Add(time.Microsecond))
	insertReviewRecord(t, db, uuid.NewString(), other, atBoundary)
	stageTwoID := uuid.NewString()
	stageTwoDue := now.Add(-30 * time.Minute)
	insertReviewRecord(t, db, stageTwoID, owner, stageTwoDue.Add(-336*time.Hour))
	if _, err := db.Exec(`
		INSERT INTO learning_record_reviews (learning_record_id, user_id, stage, result, saved_at)
		VALUES ($1, $2, 1, 'できた', $3)
	`, stageTwoID, owner, now); err != nil {
		t.Fatal(err)
	}

	checkListError(t, listRequest(mux, http.MethodGet, dueReviewsPath, "", nil), http.StatusUnauthorized, "unauthorized")

	expires := time.Now().Add(time.Hour)
	cookie := learningSession(t, db, owner, expires)
	got := decodeWebRecordJSON[dueReviewsResponse](t, listRequest(mux, http.MethodGet, dueReviewsPath, "", cookie))
	want := []dueReview{
		{ID: overdueID, Stage: 1, DueAt: now.Add(-time.Hour)},
		{ID: stageTwoID, Stage: 2, DueAt: stageTwoDue},
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
	got = decodeWebRecordJSON[dueReviewsResponse](t, listRequest(mux, http.MethodGet, dueReviewsPath, "", emptyCookie))
	if got.Records == nil || len(got.Records) != 0 {
		t.Fatalf("empty records = %#v, want []", got.Records)
	}
}
