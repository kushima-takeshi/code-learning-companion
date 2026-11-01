package learningrecord

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/config"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/connectiontoken"
)

func newListHTTP(t *testing.T) (*sql.DB, http.Handler, *connectiontoken.Service) {
	t.Helper()
	db := newIntegrationDB(t)
	body, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000005_connection_tokens.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(body)); err != nil {
		t.Fatal(err)
	}
	tokens := connectiontoken.New(db)
	mux := http.NewServeMux()
	NewHandler(New(db)).RegisterRoutes(mux, tokens)
	login := auth.NewHandler(config.Config{}, auth.NewStore(db))
	if err := connectiontoken.NewHandler(tokens).RegisterRoutes(mux, login.RequireAuth, "https://app.example"); err != nil {
		t.Fatal(err)
	}
	return db, mux, tokens
}

func listUser(t *testing.T, db *sql.DB, githubID int) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := db.Exec(`INSERT INTO users (id, github_user_id, github_login) VALUES ($1, $2, $3)`, id, githubID, id); err != nil {
		t.Fatal(err)
	}
	return id
}

func listRequest(h http.Handler, method, path, secret string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if secret != "" {
		req.Header.Set("Authorization", "bEaReR "+secret)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}

func TestLearningListHTTPIntegration(t *testing.T) {
	db, h, tokens := newListHTTP(t)
	owner, other, empty := listUser(t, db, 1), listUser(t, db, 2), listUser(t, db, 3)
	ctx := context.Background()
	issue := func(id string) connectiontoken.IssuedToken {
		t.Helper()
		issued, err := tokens.Issue(ctx, id, "AIエージェント")
		if err != nil {
			t.Fatal(err)
		}
		return issued
	}
	ownerToken, otherToken, emptyToken := issue(owner), issue(other), issue(empty)
	// The cookie belongs to a different user: Bearer authentication must win.
	sessionSecret, sessionHash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(db).CreateSession(ctx, other, sessionHash, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: "session", Value: sessionSecret}

	service := New(db)
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	var saved []Record
	for i := 0; i < 101; i++ {
		input := saveTestInput()
		input.RequestID, input.Theme = uuid.NewString(), fmt.Sprintf("theme-%03d", i)
		record, err := service.Save(ctx, owner, input)
		if err != nil {
			t.Fatal(err)
		}
		record.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		if _, err := db.Exec(`UPDATE learning_records SET created_at=$1 WHERE id=$2`, record.CreatedAt, record.ID); err != nil {
			t.Fatal(err)
		}
		saved = append(saved, record)
	}
	otherInput := saveTestInput()
	otherInput.Theme = "other-private-theme"
	otherRecord, err := service.Save(ctx, other, otherInput)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, query, secret string
		wantCount, first    int
		more                bool
	}{
		{"default", "", ownerToken.Secret, 20, 100, true},
		{"maximum", "?limit=100", ownerToken.Secret, 100, 100, true},
		{"last", "?limit=100&offset=100", ownerToken.Secret, 1, 0, false},
		{"exact end", "?limit=1&offset=100", ownerToken.Secret, 1, 0, false},
		{"past end", "?offset=101", ownerToken.Secret, 0, 0, false},
		{"maximum int offset", "?offset=" + strconv.Itoa(int(^uint(0)>>1)), ownerToken.Secret, 0, 0, false},
		{"empty owner", "", emptyToken.Secret, 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := listRequest(h, "GET", "/learning-records"+tc.query, tc.secret, cookie)
			if res.Code != 200 || res.Header().Get("Cache-Control") != "no-store" || res.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("list: %d %v %s", res.Code, res.Header(), res.Body.String())
			}
			var page ListPage
			if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}
			if page.Records == nil || len(page.Records) != tc.wantCount || page.HasMore != tc.more {
				t.Fatalf("page: %+v", page)
			}
			for i, summary := range page.Records {
				want := saved[tc.first-i]
				if summary.ID != want.ID || summary.Theme != want.Theme || !summary.CreatedAt.Equal(want.CreatedAt) {
					t.Fatalf("summary: %+v, want %+v", summary, want)
				}
			}
			for _, forbidden := range []string{owner, other, otherRecord.ID, otherRecord.Theme, "user_id", "request_id", "questions", "answer", "feedback", "secret", "hash", ownerToken.Secret} {
				if strings.Contains(res.Body.String(), forbidden) {
					t.Fatalf("private data leaked: %s", forbidden)
				}
			}
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(res.Body.Bytes(), &envelope); err != nil || len(envelope) != 2 {
				t.Fatalf("response fields: %s %v", res.Body.String(), err)
			}
			var summaries []map[string]json.RawMessage
			if err := json.Unmarshal(envelope["records"], &summaries); err != nil {
				t.Fatal(err)
			}
			for _, summary := range summaries {
				if len(summary) != 3 || summary["id"] == nil || summary["theme"] == nil || summary["created_at"] == nil {
					t.Fatalf("summary fields: %v", summary)
				}
			}
		})
	}
	res := listRequest(h, "GET", "/learning-records", otherToken.Secret, nil)
	var otherPage ListPage
	if err := json.Unmarshal(res.Body.Bytes(), &otherPage); err != nil || res.Code != 200 || len(otherPage.Records) != 1 || otherPage.Records[0].ID != otherRecord.ID {
		t.Fatalf("other user's own page: %s %v", res.Body.String(), err)
	}
	if strconv.IntSize > 32 {
		res := listRequest(h, "GET", "/learning-records?offset=2147483648", ownerToken.Secret, nil)
		if res.Code != 200 || strings.TrimSpace(res.Body.String()) != `{"records":[],"has_more":false}` {
			t.Fatalf("offset above signed 32-bit range must be accepted: %d %s", res.Code, res.Body.String())
		}
	}
	for _, query := range []string{"limit=0", "limit=101", "offset=-1", "offset=" + strconv.FormatUint(uint64(^uint(0)>>1)+1, 10), "limit=20&limit=1", "user_id=" + other, "cursor=x", "limit=%zz"} {
		checkListError(t, listRequest(h, "GET", "/learning-records?"+query, ownerToken.Secret, nil), 400, "invalid_pagination")
	}
	checkListError(t, listRequest(h, "GET", "/learning-records", "", cookie), 401, "unauthorized")
	// A connection token alone never obtains management privileges.
	for _, endpoint := range []struct{ method, path string }{
		{"GET", "/connection-tokens"}, {"POST", "/connection-tokens"},
		{"POST", "/connection-tokens/" + ownerToken.Token.ID + "/revoke"},
	} {
		checkListError(t, listRequest(h, endpoint.method, endpoint.path, ownerToken.Secret, nil), 401, "unauthorized")
	}
	for _, method := range []string{"PUT", "DELETE", "PATCH"} {
		res := listRequest(h, method, "/learning-records", ownerToken.Secret, nil)
		if res.Code != http.StatusMethodNotAllowed {
			t.Fatalf("method %s: %d", method, res.Code)
		}
	}
	if res := listRequest(h, "DELETE", "/learning-records/"+saved[0].ID, ownerToken.Secret, nil); res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("no deletion route: %d", res.Code)
	}
	var expires time.Time
	if err := db.QueryRow(`SELECT expires_at FROM connection_tokens WHERE id=$1`, ownerToken.Token.ID).Scan(&expires); err != nil || !expires.Equal(ownerToken.Token.ExpiresAt) {
		t.Fatalf("reading must not extend expiry: %v %v", expires, err)
	}
}

func TestLearningListAuthenticationIntegration(t *testing.T) {
	db, h, tokens := newListHTTP(t)
	owner := listUser(t, db, 1)
	ctx := context.Background()
	active, err := tokens.Issue(ctx, owner, "active")
	if err != nil {
		t.Fatal(err)
	}
	expired, err := tokens.Issue(ctx, owner, "expired")
	if err != nil {
		t.Fatal(err)
	}
	revoked, err := tokens.Issue(ctx, owner, "revoked")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE connection_tokens SET created_at=$1, expires_at=$2 WHERE id=$3`, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour), expired.Token.ID); err != nil {
		t.Fatal(err)
	}
	if err := tokens.Revoke(ctx, owner, revoked.Token.ID); err != nil {
		t.Fatal(err)
	}
	// Missing learning table proves authentication failures never reach the list query.
	if _, err := db.Exec(`DROP TABLE learning_records`); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"", "malformed", "clc_" + strings.Repeat("a", 64), expired.Secret, revoked.Secret} {
		res := listRequest(h, "GET", "/learning-records?limit=bad", secret, nil)
		checkListError(t, res, 401, "unauthorized")
		if res.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Fatal("missing Bearer challenge")
		}
	}
	checkListError(t, listRequest(h, "GET", "/learning-records", active.Secret, nil), 500, "internal_error")
	if _, err := db.Exec(`DROP TABLE connection_tokens`); err != nil {
		t.Fatal(err)
	}
	res := listRequest(h, "GET", "/learning-records", active.Secret, nil)
	checkListError(t, res, 500, "internal_error")
	if strings.Contains(res.Body.String(), active.Secret) || res.Header().Get("WWW-Authenticate") != "" {
		t.Fatal("DB failure must not expose secrets or be treated as invalid credentials")
	}
}
