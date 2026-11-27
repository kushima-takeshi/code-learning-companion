package learningrecord

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/config"
)

func TestWebLearningRoutes(t *testing.T) {
	records := NewHandler(nil)
	login := auth.NewHandler(config.Config{}, nil)
	mux := http.NewServeMux()
	if err := records.RegisterWebRoutes(mux, login.RequireAPIAuth, "https://app.example/path"); err == nil {
		t.Fatal("trusted origin must not contain a path")
	}
	if err := records.RegisterWebRoutes(mux, login.RequireAPIAuth, "https://app.example"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/web/learning-records?limit=0", "/web/learning-records/invalid"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			res := listRequest(mux, method, path, "", nil)
			checkListError(t, res, http.StatusUnauthorized, "unauthorized")
			if res.Header().Get("WWW-Authenticate") != "" {
				t.Fatal("Web API must not advertise Bearer authentication")
			}
		}
		if res := listRequest(mux, http.MethodPost, path, "", nil); res.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s: %d", path, res.Code)
		}
	}
}

func TestWebLearningRecordsHTTPIntegration(t *testing.T) {
	db, handler, tokens := newListHTTP(t, reviewMigration)
	owner, other := listUser(t, db, 1), listUser(t, db, 2)
	ctx := context.Background()
	cookie := learningSession(t, db, owner, time.Now().Add(time.Hour))
	otherToken, err := tokens.Issue(ctx, other, "other")
	if err != nil {
		t.Fatal(err)
	}
	save := func(userID, repository string) Record {
		t.Helper()
		input := validSaveInputForTest(1)
		input.RequestID, input.Repository = uuid.NewString(), repository
		record, err := New(db).Save(ctx, userID, input)
		if err != nil {
			t.Fatal(err)
		}
		return record
	}
	withRepository := save(owner, "owner/repo")
	withoutRepository := save(owner, "")
	otherRecord := save(other, "other/repo")
	records := map[string]Record{withRepository.ID: withRepository, withoutRepository.ID: withoutRepository}
	ids := []string{withRepository.ID, withoutRepository.ID}
	slices.Sort(ids)
	slices.Reverse(ids)
	// Force a tie to verify the ID sort, with another user's record ordered ahead of both.
	if _, err := db.Exec(`UPDATE learning_records SET created_at = CASE WHEN id = $1 THEN now() ELSE now() - interval '1 day' END`, otherRecord.ID); err != nil {
		t.Fatal(err)
	}
	const path = "/web/learning-records"
	get := func(path string) *httptest.ResponseRecorder {
		return listRequest(handler, http.MethodGet, path, otherToken.Secret, cookie)
	}
	t.Run("list_and_pagination", func(t *testing.T) {
		for _, tc := range []struct {
			query  string
			ids    []string
			more   bool
			bearer string
		}{
			{ids: ids},
			{query: "?limit=1", ids: ids[:1], more: true, bearer: otherToken.Secret},
			{query: "?limit=1&offset=1", ids: ids[1:]},
			{query: "?limit=100&offset=2", ids: []string{}},
		} {
			res := listRequest(handler, http.MethodGet, path+tc.query, tc.bearer, cookie)
			page := decodeWebRecordJSON[struct {
				Records []map[string]json.RawMessage `json:"records"`
				HasMore bool                         `json:"has_more"`
			}](t, res)
			gotIDs := make([]string, 0, len(page.Records))
			for _, summary := range page.Records {
				var id string
				if err := json.Unmarshal(summary["id"], &id); err != nil {
					t.Fatal(err)
				}
				gotIDs = append(gotIDs, id)
				checkWebRepository(t, summary, records[id].Repository)
			}
			if page.Records == nil || !slices.Equal(gotIDs, tc.ids) || page.HasMore != tc.more {
				t.Fatalf("%q: %s", tc.query, res.Body.String())
			}
		}
		for _, query := range []string{"limit=0", "limit=101", "offset=-1", "offset=%zz", "limit=1&limit=2", "user_id=" + other} {
			checkListError(t, get(path+"?"+query), http.StatusBadRequest, "invalid_pagination")
		}
	})
	t.Run("detail_and_ownership", func(t *testing.T) {
		for _, record := range records {
			res := listRequest(handler, http.MethodGet, path+"/"+strings.ToUpper(record.ID), "", cookie)
			body := decodeWebRecordJSON[recordResponse](t, res)
			if body.ID != record.ID || body.Theme != record.Theme || !reflect.DeepEqual(body.Questions, record.Questions) {
				t.Fatalf("unexpected detail: %s", res.Body.String())
			}
			checkWebRepository(t, decodeWebRecordJSON[map[string]json.RawMessage](t, res), record.Repository)
		}
		checkListError(t, get(path+"/invalid"), http.StatusBadRequest, "invalid_record_id")
		missing := get(path + "/" + uuid.NewString())
		forbidden := get(path + "/" + otherRecord.ID + "?user_id=" + other)
		checkListError(t, missing, http.StatusNotFound, "record_not_found")
		checkListError(t, forbidden, http.StatusNotFound, "record_not_found")
		if missing.Body.String() != forbidden.Body.String() {
			t.Fatal("missing and other user's records must have the same response")
		}
	})
	t.Run("bearer_routes_unchanged", func(t *testing.T) {
		for _, endpoint := range []struct{ method, path string }{
			{http.MethodGet, "/learning-records"},
			{http.MethodGet, "/learning-records/" + withRepository.ID},
			{http.MethodPost, "/learning-records"},
		} {
			res := listRequest(handler, endpoint.method, endpoint.path, "", cookie)
			checkListError(t, res, http.StatusUnauthorized, "unauthorized")
			if res.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing Bearer challenge")
			}
		}
		res := listRequest(handler, http.MethodGet, "/learning-records", otherToken.Secret, cookie)
		page := decodeWebRecordJSON[ListPage](t, res)
		if len(page.Records) != 1 || page.Records[0].ID != otherRecord.ID || page.HasMore {
			t.Fatalf("Bearer list must use token owner: %s", res.Body.String())
		}
	})
}

func TestWebLearningAuthenticationHTTPIntegration(t *testing.T) {
	db, handler, tokens := newListHTTP(t)
	owner := listUser(t, db, 1)
	token, err := tokens.Issue(context.Background(), owner, "owner")
	if err != nil {
		t.Fatal(err)
	}
	valid := learningSession(t, db, owner, time.Now().Add(time.Hour))
	expired := learningSession(t, db, owner, time.Now().Add(-time.Hour))
	invalid := &http.Cookie{Name: "session", Value: "invalid"}
	for _, path := range []string{"/web/learning-records?limit=0", "/web/learning-records/invalid"} {
		for _, tc := range []struct {
			bearer string
			cookie *http.Cookie
		}{
			{}, {bearer: token.Secret}, {cookie: invalid}, {cookie: expired}, {bearer: token.Secret, cookie: invalid},
		} {
			res := listRequest(handler, http.MethodGet, path, tc.bearer, tc.cookie)
			checkListError(t, res, http.StatusUnauthorized, "unauthorized")
			if res.Header().Get("WWW-Authenticate") != "" {
				t.Fatal("Web API must not advertise Bearer authentication")
			}
		}
	}
	if _, err := db.Exec(`DROP TABLE learning_records`); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/web/learning-records", "/web/learning-records/" + uuid.NewString()} {
		checkListError(t, listRequest(handler, http.MethodGet, path, "", valid), http.StatusInternalServerError, "internal_error")
	}
	if _, err := db.Exec(`DROP TABLE sessions`); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/web/learning-records", "/web/learning-records/invalid"} {
		checkListError(t, listRequest(handler, http.MethodGet, path, "", valid), http.StatusInternalServerError, "internal_error")
	}
}

func learningSession(t *testing.T, db *sql.DB, userID string, expiresAt time.Time) *http.Cookie {
	t.Helper()
	session, hash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(db).CreateSession(context.Background(), userID, hash, expiresAt); err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: "session", Value: session}
}

func decodeWebRecordJSON[T any](t *testing.T, res *httptest.ResponseRecorder) T {
	t.Helper()
	if res.Code != http.StatusOK || res.Header().Get("Content-Type") != "application/json" || res.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("response: %d %v %s", res.Code, res.Header(), res.Body.String())
	}
	var value T
	if err := json.Unmarshal(res.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func checkWebRepository(t *testing.T, object map[string]json.RawMessage, repository string) {
	t.Helper()
	want := ""
	if repository != "" {
		want = `"` + repository + `"`
	}
	if string(object["repository"]) != want {
		t.Fatalf("repository: %s, want %q", object["repository"], want)
	}
}
