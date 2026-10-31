package connectiontoken

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
)

func TestManagementAuthenticationIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	owner := createUser(t, db)
	h := managementHTTP(t, db)
	issued, err := New(db).Issue(context.Background(), owner, "Codex")
	if err != nil {
		t.Fatal(err)
	}
	expired, hash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(db).CreateSession(context.Background(), owner, hash, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []struct{ method, path, body string }{
		{"POST", "/connection-tokens", `{"name":"Codex"}`},
		{"GET", "/connection-tokens", ""},
		{"POST", "/connection-tokens/" + issued.Token.ID + "/revoke", ""},
	} {
		for _, credentials := range []struct{ name, bearer, cookie string }{
			{name: "未ログイン"},
			{name: "Bearerだけ", bearer: issued.Secret},
			{name: "連携原本をCookieに指定", cookie: issued.Secret},
			{name: "無効セッション", cookie: "unknown"},
			{name: "期限切れセッション", cookie: expired},
		} {
			t.Run(endpoint.method+endpoint.path+credentials.name, func(t *testing.T) {
				req := httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader(endpoint.body))
				req.Header.Set("Content-Type", "application/json")
				if credentials.bearer != "" {
					req.Header.Set("Authorization", "Bearer "+credentials.bearer)
				}
				if credentials.cookie != "" {
					req.AddCookie(&http.Cookie{Name: "session", Value: credentials.cookie})
				}
				res := httptest.NewRecorder()
				h.ServeHTTP(res, req)
				checkHTTPError(t, res, 401, "unauthorized")
			})
		}
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM connection_tokens WHERE revoked_at IS NULL`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("拒否で発行・失効しない: %d %v", count, err)
	}
}

func TestManagementCrossOriginIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	owner := createUser(t, db)
	h := managementHTTP(t, db)
	cookie := webSession(t, db, owner)
	issued, err := New(db).Issue(context.Background(), owner, "Codex")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/connection-tokens", "/connection-tokens/" + issued.Token.ID + "/revoke"} {
		for _, tc := range []struct{ name, origin, fetchSite string }{
			{"別サイト", "https://evil.example", "cross-site"},
			{"Originのみ", "https://evil.example", ""},
			{"null Origin", "null", ""},
			{"同一siteの別origin", "https://evil.example", "same-site"},
		} {
			t.Run(path+tc.name, func(t *testing.T) {
				req := httptest.NewRequest("POST", path, strings.NewReader(`{"name":"Codex"}`))
				req.Header.Set("Origin", tc.origin)
				req.Header.Set("Sec-Fetch-Site", tc.fetchSite)
				req.Header.Set("Content-Type", "application/json")
				req.AddCookie(cookie)
				res := httptest.NewRecorder()
				h.ServeHTTP(res, req)
				checkHTTPError(t, res, 403, "cross_origin_forbidden")
			})
		}
	}
	// The configured frontend may be cross-site and must still be accepted.
	req := httptest.NewRequest("POST", "/connection-tokens", strings.NewReader(`{"name":"trusted"}`))
	req.Header.Set("Origin", webOrigin)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.AddCookie(cookie)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != 201 {
		t.Fatalf("設定済みfrontendを許可: %d %s", res.Code, res.Body.String())
	}
	if _, err := New(db).Authenticate(context.Background(), issued.Secret); err != nil {
		t.Fatal("拒否した要求で失効しない")
	}
}

func TestManagementDBFailureIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	owner := createUser(t, db)
	h := managementHTTP(t, db)
	cookie := webSession(t, db, owner)
	issued, err := New(db).Issue(context.Background(), owner, "Codex")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE connection_tokens`); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []struct{ method, path, body string }{
		{"POST", "/connection-tokens", `{"name":"Codex"}`},
		{"GET", "/connection-tokens", ""},
		{"POST", "/connection-tokens/" + issued.Token.ID + "/revoke", ""},
	} {
		res := managementRequest(h, endpoint.method, endpoint.path, endpoint.body, cookie)
		checkHTTPError(t, res, 500, "internal_error")
		if strings.Contains(res.Body.String(), "secret") || strings.Contains(res.Body.String(), "SQL") || strings.Contains(res.Body.String(), issued.Secret) {
			t.Fatal("DB障害や原本を公開しない")
		}
	}
}
