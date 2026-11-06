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

func TestReconnectHTTPUnavailableTargetIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	ctx := context.Background()
	owner := createUser(t, db)
	s := New(db)
	h := managementHTTP(t, db)
	cookie := webSession(t, db, owner)
	counts := func(oldID string) (tokens, ops int) {
		t.Helper()
		if err := db.QueryRow(`SELECT count(*) FROM connection_tokens WHERE user_id=$1`, owner).Scan(&tokens); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT count(*) FROM connection_reconnections WHERE old_token_id=$1`, oldID).Scan(&ops); err != nil {
			t.Fatal(err)
		}
		return tokens, ops
	}
	rejectHTTP := func(t *testing.T, oldID string) {
		t.Helper()
		tokens, ops := counts(oldID)
		res := managementRequest(h, "POST", "/connection-tokens/"+oldID+"/reconnections", "", cookie)
		checkHTTPError(t, res, 409, "token_unavailable")
		if strings.Contains(res.Body.String(), "clc_") || strings.Contains(res.Body.String(), "secret") {
			t.Fatalf("rejected start leaked a secret: %s", res.Body.String())
		}
		if gotTokens, gotOps := counts(oldID); gotTokens != tokens || gotOps != ops {
			t.Fatalf("created rows: tokens %d->%d ops %d->%d", tokens, gotTokens, ops, gotOps)
		}
	}

	t.Run("期限切れは409 token_unavailableで行を増やさない", func(t *testing.T) {
		issued, err := s.Issue(ctx, owner, "期限切れ")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE connection_tokens SET created_at=now()-interval '91 days', expires_at=now()-interval '1 day' WHERE id=$1`, issued.Token.ID); err != nil {
			t.Fatal(err)
		}
		rejectHTTP(t, issued.Token.ID)
	})
	t.Run("失効済みは409 token_unavailableで行を増やさない", func(t *testing.T) {
		issued, err := s.Issue(ctx, owner, "失効")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Revoke(ctx, owner, issued.Token.ID); err != nil {
			t.Fatal(err)
		}
		rejectHTTP(t, issued.Token.ID)
	})
	t.Run("進行中の再送は旧トークンが期限切れでも200で同じ操作を返す", func(t *testing.T) {
		issued, err := s.Issue(ctx, owner, "進行中")
		if err != nil {
			t.Fatal(err)
		}
		path := "/connection-tokens/" + issued.Token.ID + "/reconnections"
		first := readReconnectHTTP(t, managementRequest(h, "POST", path, "", cookie), 201)
		if _, err := db.Exec(`UPDATE connection_tokens SET created_at=now()-interval '91 days', expires_at=now()-interval '1 second' WHERE id=$1`, issued.Token.ID); err != nil {
			t.Fatal(err)
		}
		tokens, ops := counts(issued.Token.ID)
		again := readReconnectHTTP(t, managementRequest(h, "POST", path, "", cookie), 200)
		if again.Secret != "" || again.Reconnection.ID != first.Reconnection.ID || again.Reconnection.State != "pending" {
			t.Fatalf("in-progress retry: %+v", again)
		}
		if gotTokens, gotOps := counts(issued.Token.ID); gotTokens != tokens || gotOps != ops {
			t.Fatalf("retry created rows: tokens %d->%d ops %d->%d", tokens, gotTokens, ops, gotOps)
		}
	})
}
