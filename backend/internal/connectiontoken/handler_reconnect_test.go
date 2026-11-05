package connectiontoken

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type reconnectHTTPResult struct {
	Reconnection struct {
		ID         string    `json:"id"`
		OldTokenID string    `json:"old_token_id"`
		NewTokenID string    `json:"new_token_id"`
		State      string    `json:"state"`
		CreatedAt  time.Time `json:"created_at"`
		ExpiresAt  time.Time `json:"expires_at"`
	} `json:"reconnection"`
	Secret string `json:"secret"`
}

func readReconnectHTTP(t *testing.T, res *httptest.ResponseRecorder, status int) reconnectHTTPResult {
	t.Helper()
	var got reconnectHTTPResult
	if res.Code != status {
		t.Fatalf("reconnect HTTP: %d %s; want %d", res.Code, res.Body.String(), status)
	}
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if res.Header().Get("Cache-Control") != "no-store" || res.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("headers: %v", res.Header())
	}
	for _, hidden := range []string{"user_id", "hash"} {
		if strings.Contains(res.Body.String(), hidden) {
			t.Fatalf("leaked %s", hidden)
		}
	}
	return got
}
func TestReconnectHTTPRoutesIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	ctx := context.Background()
	owner, other := createUser(t, db), createUser(t, db)
	s := New(db)
	old, err := s.Issue(ctx, owner, "PC")
	if err != nil {
		t.Fatal(err)
	}
	h := managementHTTP(t, db)
	cookie, otherCookie := webSession(t, db, owner), webSession(t, db, other)
	startPath := "/connection-tokens/" + old.Token.ID + "/reconnections"
	first := readReconnectHTTP(t, managementRequest(h, "POST", startPath, "", cookie), 201)
	if first.Secret == "" || first.Reconnection.State != "pending" || first.Reconnection.OldTokenID != old.Token.ID || first.Reconnection.ExpiresAt.Sub(first.Reconnection.CreatedAt) != 24*time.Hour {
		t.Fatalf("start: %+v", first.Reconnection)
	}
	again := readReconnectHTTP(t, managementRequest(h, "POST", startPath, "", cookie), 200)
	if again.Secret != "" || again.Reconnection.ID != first.Reconnection.ID {
		t.Fatal("duplicate start issued a secret")
	}
	path := "/connection-reconnections/" + first.Reconnection.ID
	readReconnectHTTP(t, managementRequest(h, "GET", path, "", cookie), 200)
	checkHTTPError(t, managementRequest(h, "POST", path+"/complete", "", cookie), 409, "reconnection_conflict")
	for _, credentials := range []*http.Cookie{otherCookie} {
		checkHTTPError(t, managementRequest(h, "POST", startPath, "", credentials), 404, "token_not_found")
		for _, tc := range []struct{ method, suffix string }{{"GET", ""}, {"POST", "/complete"}, {"POST", "/restart"}} {
			checkHTTPError(t, managementRequest(h, tc.method, path+tc.suffix, "", credentials), 404, "reconnection_not_found")
		}
	}
	second := readReconnectHTTP(t, managementRequest(h, "POST", path+"/restart", "", cookie), 201)
	if second.Secret == "" || second.Reconnection.ID == first.Reconnection.ID {
		t.Fatal("restart failed")
	}
	checkHTTPError(t, managementRequest(h, "POST", path+"/complete", "", cookie), 409, "reconnection_conflict")
	checkHTTPError(t, managementRequest(h, "POST", path+"/restart", "", cookie), 409, "reconnection_conflict")
	if err := s.ConfirmReconnect(ctx, owner, second.Reconnection.NewTokenID); err != nil {
		t.Fatal(err)
	}
	secondPath := "/connection-reconnections/" + second.Reconnection.ID
	completed := readReconnectHTTP(t, managementRequest(h, "POST", secondPath+"/complete", "", cookie), 200)
	if completed.Reconnection.State != "completed" || completed.Secret != "" {
		t.Fatal("completion state")
	}
	readReconnectHTTP(t, managementRequest(h, "POST", secondPath+"/complete", "", cookie), 200)
	checkHTTPError(t, managementRequest(h, "POST", secondPath+"/restart", "", cookie), 409, "reconnection_conflict")
	for _, method := range []string{"GET", "HEAD"} {
		res := managementRequest(h, method, secondPath, "", cookie)
		if res.Code != 200 || strings.Contains(res.Body.String(), second.Secret) || strings.Contains(res.Body.String(), "secret") {
			t.Fatalf("status leak: %d %s", res.Code, res.Body.String())
		}
	}
	for _, id := range []string{uuid.NewString(), "invalid"} {
		code, status := "reconnection_not_found", 404
		if id == "invalid" {
			code, status = "invalid_reconnection_id", 400
		}
		checkHTTPError(t, managementRequest(h, "GET", "/connection-reconnections/"+id, "", cookie), status, code)
	}
	checkHTTPError(t, managementRequest(h, "POST", "/connection-tokens/invalid/reconnections", "", cookie), 400, "invalid_token_id")
}

func TestReconnectHTTPProtectionIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	ctx := context.Background()
	owner := createUser(t, db)
	s := New(db)
	old, err := s.Issue(ctx, owner, "PC")
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.StartReconnect(ctx, owner, old.Token.ID)
	if err != nil {
		t.Fatal(err)
	}
	h := managementHTTP(t, db)
	cookie := webSession(t, db, owner)
	for _, tc := range []struct{ method, path string }{
		{"POST", "/connection-tokens/" + old.Token.ID + "/reconnections"},
		{"GET", "/connection-reconnections/" + first.Operation.ID},
		{"POST", "/connection-reconnections/" + first.Operation.ID + "/complete"},
		{"POST", "/connection-reconnections/" + first.Operation.ID + "/restart"},
	} {
		// A Bearer token and an unauthenticated request both fail before reading the body or validating an ID.
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("bad body"))
		req.Header.Set("Authorization", "Bearer "+first.Secret)
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		checkHTTPError(t, res, 401, "unauthorized")
		if tc.method == "POST" {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.AddCookie(cookie)
			req.Header.Set("Origin", "https://evil.example")
			req.Header.Set("Sec-Fetch-Site", "cross-site")
			res := httptest.NewRecorder()
			h.ServeHTTP(res, req)
			checkHTTPError(t, res, 403, "cross_origin_forbidden")
			for _, body := range []string{`{"user_id":"other"}`, `null`, `{}`} {
				checkHTTPError(t, managementRequest(h, tc.method, tc.path, body, cookie), 400, "invalid_reconnection_request")
			}
			checkHTTPError(t, managementRequest(h, tc.method, tc.path, strings.Repeat("x", 4097), cookie), 413, "request_too_large")
		}
		checkHTTPError(t, managementRequest(h, tc.method, tc.path+"?user_id=other", "", cookie), 400, "invalid_reconnection_request")
	}
	// Web session storage failures retain the existing 500 distinction.
	if _, err := db.Exec(`DROP TABLE connection_reconnections`); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, path string }{
		{"POST", "/connection-tokens/" + old.Token.ID + "/reconnections"},
		{"GET", "/connection-reconnections/" + first.Operation.ID},
		{"POST", "/connection-reconnections/" + first.Operation.ID + "/complete"},
		{"POST", "/connection-reconnections/" + first.Operation.ID + "/restart"},
	} {
		checkHTTPError(t, managementRequest(h, tc.method, tc.path, "", cookie), 500, "internal_error")
	}
}
