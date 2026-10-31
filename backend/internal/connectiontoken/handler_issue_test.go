package connectiontoken

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/config"
)

const webOrigin = "http://localhost:5173"

func managementHTTP(t *testing.T, db *sql.DB) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	login := auth.NewHandler(config.Config{}, auth.NewStore(db))
	if err := NewHandler(New(db)).RegisterRoutes(mux, login.RequireAuth, webOrigin); err != nil {
		t.Fatal(err)
	}
	return mux
}

func webSession(t *testing.T, db *sql.DB, owner string) *http.Cookie {
	t.Helper()
	secret, hash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(db).CreateSession(context.Background(), owner, hash, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: "session", Value: secret}
}

func managementRequest(h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", webOrigin)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}

func checkHTTPError(t *testing.T, res *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("JSONエラー応答: %v (%s)", err, res.Body.String())
	}
	if res.Code != status || body["error"] != code {
		t.Fatalf("status=%d body=%s; want %d %s", res.Code, res.Body.String(), status, code)
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("管理応答はキャッシュしない")
	}
}

func TestManagementIssueIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	owner := createUser(t, db)
	h := managementHTTP(t, db)
	cookie := webSession(t, db, owner)
	res := managementRequest(h, "POST", "/connection-tokens", `{"name":"  Codex  "}`, cookie)
	if res.Code != http.StatusCreated {
		t.Fatalf("発行: %d %s", res.Code, res.Body.String())
	}
	var got struct {
		Token struct {
			ID        string     `json:"id"`
			Name      string     `json:"name"`
			CreatedAt time.Time  `json:"created_at"`
			ExpiresAt time.Time  `json:"expires_at"`
			RevokedAt *time.Time `json:"revoked_at"`
		} `json:"token"`
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Token.ID == "" || got.Token.Name != "Codex" || got.Token.ExpiresAt.Sub(got.Token.CreatedAt) != Lifetime || got.Token.RevokedAt != nil {
		t.Fatalf("発行メタデータ: %+v", got.Token)
	}
	token, err := New(db).Authenticate(context.Background(), got.Secret)
	if err != nil || token.UserID != owner || token.ID != got.Token.ID {
		t.Fatalf("原本で本人を照合: %+v %v", token, err)
	}
	if strings.Contains(res.Body.String(), "user_id") || strings.Contains(res.Body.String(), "hash") {
		t.Fatal("内部情報をHTTPへ出さない")
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("原本をキャッシュしない")
	}
}

func TestManagementIssueInvalidIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	owner := createUser(t, db)
	h := managementHTTP(t, db)
	cookie := webSession(t, db, owner)
	for _, tc := range []struct {
		name, body, code string
		status           int
	}{
		{"空", "", "invalid_json", 400},
		{"構文", "{", "invalid_json", 400},
		{"型", `{"name":1}`, "invalid_json", 400},
		{"空名", `{"name":"  "}`, "invalid_name", 400},
		{"名前なし", `{}`, "invalid_name", 400},
		{"null", `null`, "invalid_json", 400},
		{"所有者偽装", `{"name":"Codex","user_id":"other"}`, "invalid_json", 400},
		{"末尾JSON", `{"name":"Codex"} {}`, "invalid_json", 400},
		{"大きな要求", `{"name":"` + strings.Repeat("a", 4096) + `"}`, "request_too_large", 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checkHTTPError(t, managementRequest(h, "POST", "/connection-tokens", tc.body, cookie), tc.status, tc.code)
		})
	}
	req := httptest.NewRequest("POST", "/connection-tokens", strings.NewReader(`{"name":"Codex"}`))
	req.Header.Set("Content-Type", "text/plain")
	req.AddCookie(cookie)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	checkHTTPError(t, res, 415, "unsupported_media_type")
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM connection_tokens`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("不正入力では発行しない: %d %v", count, err)
	}
}
