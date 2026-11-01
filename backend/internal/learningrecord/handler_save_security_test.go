package learningrecord

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
)

type observedBody struct{ read bool }

func (b *observedBody) Read([]byte) (int, error) {
	b.read = true
	return 0, errors.New("body read failure")
}

func TestLearningSaveSecurityHTTPIntegration(t *testing.T) {
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
	session, hash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(db).CreateSession(ctx, owner, hash, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	for _, headers := range [][]string{nil, {"Bearer malformed"}, {"Bearer clc_" + strings.Repeat("a", 64)}, {"Bearer " + expired.Secret}, {"Bearer " + revoked.Secret}, {"Bearer " + active.Secret, "Bearer " + active.Secret}, {"Basic " + active.Secret}} {
		body := &observedBody{}
		req := httptest.NewRequest("POST", "/learning-records", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", "session="+session)
		for _, value := range headers {
			req.Header.Add("Authorization", value)
		}
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		checkListError(t, res, 401, "unauthorized")
		if body.read || res.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Fatal("authentication must precede body reading")
		}
	}
	// A read error is a client input error, not a database failure.
	body := &observedBody{}
	req := httptest.NewRequest("POST", "/learning-records", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+active.Secret)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	checkListError(t, res, 400, "invalid_json")
	if !body.read {
		t.Fatal("expected an attempted read")
	}
	if _, err := db.Exec(`DROP TABLE learning_records`); err != nil {
		t.Fatal(err)
	}
	checkListError(t, recordRequest(h, "POST", "/learning-records", active.Secret, "application/json", inputJSON(t, validSaveInputForTest(1)), nil), 500, "internal_error")
	if _, err := db.Exec(`DROP TABLE connection_tokens`); err != nil {
		t.Fatal(err)
	}
	res = recordRequest(h, "POST", "/learning-records", active.Secret, "application/json", "{", nil)
	checkListError(t, res, 500, "internal_error")
	if res.Header().Get("WWW-Authenticate") != "" {
		t.Fatal("database failure is not invalid credentials")
	}
}

func TestSaveHandlerRequiresTokenContext(t *testing.T) {
	res := httptest.NewRecorder()
	NewHandler(nil).Save(res, httptest.NewRequest("POST", "/learning-records", nil))
	checkListError(t, res, 401, "unauthorized")
	if res.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatal("missing Bearer challenge")
	}
}
