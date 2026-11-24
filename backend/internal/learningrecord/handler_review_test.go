package learningrecord

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func postReview(ctx context.Context, handler http.Handler, id, body, origin string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/web/learning-records/"+id+"/reviews", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", origin)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func TestWebSaveReviewProgressHTTPIntegration(t *testing.T) {
	db := newIntegrationDB(t, reviewMigration)
	owner := listUser(t, db, 1)
	cookie := learningSession(t, db, owner, time.Now().Add(time.Hour))
	createdAt := time.Date(2026, 10, 31, 20, 35, 12, 123456000, time.UTC)
	now := createdAt
	handler := newReviewHTTP(t, db, func() time.Time { return now })
	deadlines := []time.Duration{72 * time.Hour, 336 * time.Hour, 720 * time.Hour}

	// Include each of the four results in every stage across four records.
	results := []string{"できた", "一部できた", "もう一度", "判定保留"}
	ids := make([]string, len(results))
	for i := range ids {
		ids[i] = uuid.NewString()
		insertReviewRecord(t, db, ids[i], owner, createdAt)
	}
	getDue := func() []dueReview {
		return decodeWebRecordJSON[dueReviewsResponse](t, listRequest(handler, http.MethodGet, dueReviewsPath, "", cookie)).Records
	}
	for i, delay := range deadlines {
		stage := i + 1
		deadline := createdAt.Add(delay)
		now = deadline.Add(-time.Microsecond)
		if got := getDue(); got == nil || len(got) != 0 {
			t.Fatalf("before stage %d: records = %+v, want []", stage, got)
		}
		now = deadline
		due := getDue()
		if len(due) != len(ids) {
			t.Fatalf("at stage %d: records = %+v, want %d records", stage, due, len(ids))
		}
		for _, record := range due {
			if record.Stage != stage || !record.DueAt.Equal(deadline) {
				t.Fatalf("at stage %d: record = %+v, want due_at %s", stage, record, deadline)
			}
		}

		// Save late to show subsequent deadlines ignore the actual save time.
		now = deadline.Add(time.Hour)
		for index, id := range ids {
			result := results[(index+i)%len(results)]
			body := fmt.Sprintf(`{"result":%q,"stage":%d}`, result, stage)
			res := postReview(t.Context(), handler, id, body, "https://app.example", cookie)
			var reply struct{ Stage int }
			if res.Code != http.StatusCreated || json.Unmarshal(res.Body.Bytes(), &reply) != nil || reply.Stage != stage {
				t.Fatalf("save stage %d = %d %s", stage, res.Code, res.Body)
			}
			if stage == 1 && index == 0 {
				checkListError(t, postReview(t.Context(), handler, id, body, "https://app.example", cookie), 409, "review_conflict")
			}
			var savedResult string
			var savedAt time.Time
			err := db.QueryRow(`
				SELECT result, saved_at FROM learning_record_reviews
				WHERE learning_record_id = $1 AND user_id = $2 AND stage = $3
			`, id, owner, stage).Scan(&savedResult, &savedAt)
			if err != nil || savedResult != result || !savedAt.Equal(now) {
				t.Fatalf("stored stage %d = %q at %s, err = %v", stage, savedResult, savedAt, err)
			}
		}
		if got := getDue(); got == nil || len(got) != 0 {
			t.Fatalf("after saving stage %d: records = %+v, want []", stage, got)
		}
	}
	now = createdAt.Add(1000 * time.Hour)
	if got := getDue(); got == nil || len(got) != 0 {
		t.Fatalf("after all deadlines: records = %+v, want []", got)
	}
	checkListError(t, postReview(t.Context(), handler, ids[0], `{"result":"できた","stage":3}`, "https://app.example", cookie), 409, "review_conflict")
}

func TestWebSaveReviewErrorsHTTPIntegration(t *testing.T) {
	db := newIntegrationDB(t, reviewMigration)
	owner, other := listUser(t, db, 1), listUser(t, db, 2)
	id := uuid.NewString()
	now := time.Now()
	insertReviewRecord(t, db, id, owner, now)
	cookie := learningSession(t, db, owner, now.Add(time.Hour))
	otherCookie := learningSession(t, db, other, now.Add(time.Hour))
	handler := newReviewHTTP(t, db, func() time.Time { return now })
	const valid = `{"result":"できた","stage":1}`
	for _, tc := range []struct {
		name, id, body, origin string
		cookie                 *http.Cookie
		status                 int
		code                   string
	}{
		{"no session", id, valid, "https://app.example", nil, 401, "unauthorized"},
		{"other owner", id, valid, "https://app.example", otherCookie, 404, "record_not_found"},
		{"missing record", uuid.NewString(), valid, "https://app.example", cookie, 404, "record_not_found"},
		{"invalid ID", "bad-id", valid, "https://app.example", cookie, 400, "invalid_record_id"},
		{"broken JSON", id, "{", "https://app.example", cookie, 400, "invalid_json"},
		{"extra JSON", id, valid + " {}", "https://app.example", cookie, 400, "invalid_json"},
		{"null JSON", id, "null", "https://app.example", cookie, 400, "invalid_json"},
		{"unknown field", id, `{"result":"できた","stage":1,"unexpected":true}`, "https://app.example", cookie, 400, "invalid_json"},
		{"unknown result", id, `{"result":"成功","stage":1}`, "https://app.example", cookie, 400, "invalid_result"},
		{"missing result", id, `{"stage":1}`, "https://app.example", cookie, 400, "invalid_result"},
		{"missing stage", id, `{"result":"できた"}`, "https://app.example", cookie, 400, "invalid_stage"},
		{"stage below range", id, `{"result":"できた","stage":0}`, "https://app.example", cookie, 400, "invalid_stage"},
		{"stage above range", id, `{"result":"できた","stage":4}`, "https://app.example", cookie, 400, "invalid_stage"},
		{"non-integer stage", id, `{"result":"できた","stage":1.5}`, "https://app.example", cookie, 400, "invalid_stage"},
		{"skips stage", id, `{"result":"できた","stage":2}`, "https://app.example", cookie, 409, "review_conflict"},
		{"cross-origin", id, valid, "https://other.example", cookie, 403, "cross_origin_forbidden"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := postReview(t.Context(), handler, tc.id, tc.body, tc.origin, tc.cookie)
			checkListError(t, res, tc.status, tc.code)
		})
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM learning_record_reviews").Scan(&count); err != nil || count != 0 {
		t.Fatalf("reviews after rejected requests = %d, err = %v", count, err)
	}
}

func TestWebSaveReviewConflictHTTPIntegration(t *testing.T) {
	db := newIntegrationDB(t, reviewMigration)
	owner := listUser(t, db, 1)
	cookie := learningSession(t, db, owner, time.Now().Add(time.Hour))
	id := uuid.NewString()
	now := time.Now()
	insertReviewRecord(t, db, id, owner, now)
	handler := newReviewHTTP(t, db, func() time.Time { return now })

	// Hold the winning insert uncommitted so the HTTP request selects the same stage.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var winnerPID int
	if err := tx.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&winnerPID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO learning_record_reviews (learning_record_id, user_id, stage, result, saved_at)
		VALUES ($1, $2, 1, 'もう一度', $3)
	`, id, owner, now); err != nil {
		t.Fatal(err)
	}

	response := make(chan *httptest.ResponseRecorder, 1)
	const stageOneBody = `{"result":"できた","stage":1}`
	go func() {
		response <- postReview(ctx, handler, id, stageOneBody, "https://app.example", cookie)
	}()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE datname = current_database() AND $1 = ANY(pg_blocking_pids(pid))
			)
		`, winnerPID).Scan(&blocked)
		if err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case res := <-response:
			t.Fatalf("save returned before stage 1 was committed: %d %s", res.Code, res.Body)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case res := <-response:
		checkListError(t, res, 409, "review_conflict")
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// Retrying the same stage after commit must not write the next stage.
	checkListError(t, postReview(ctx, handler, id, stageOneBody, "https://app.example", cookie), 409, "review_conflict")
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM learning_record_reviews").Scan(&count); err != nil || count != 1 {
		t.Fatalf("reviews after conflict and retry = %d, err = %v", count, err)
	}
	// A request specifying the next stage succeeds even before its deadline.
	res := postReview(ctx, handler, id, `{"result":"できた","stage":2}`, "https://app.example", cookie)
	if res.Code != http.StatusCreated || strings.TrimSpace(res.Body.String()) != `{"stage":2}` {
		t.Fatalf("new save after stage 1 = %d %s", res.Code, res.Body)
	}
}
