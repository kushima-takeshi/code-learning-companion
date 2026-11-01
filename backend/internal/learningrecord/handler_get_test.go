package learningrecord

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
)

func TestLearningGetHTTPIntegration(t *testing.T) {
	db, h, tokens := newListHTTP(t)
	owner, other := listUser(t, db, 1), listUser(t, db, 2)
	ctx := context.Background()
	token, err := tokens.Issue(ctx, owner, "owner")
	if err != nil {
		t.Fatal(err)
	}
	otherToken, err := tokens.Issue(ctx, other, "other")
	if err != nil {
		t.Fatal(err)
	}
	session, hash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(db).CreateSession(ctx, other, hash, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: "session", Value: session}
	input := validSaveInputForTest(1)
	saved := recordRequest(h, "POST", "/learning-records", token.Secret, "application/json", inputJSON(t, input), nil)
	id := checkRecordResponse(t, saved, input)
	for _, requestedID := range []string{id, strings.ToUpper(id)} {
		res := recordRequest(h, "GET", "/learning-records/"+requestedID, token.Secret, "", "", cookie)
		if checkRecordResponse(t, res, input) != id || res.Body.String() != saved.Body.String() {
			t.Fatal("detail differs from saved content")
		}
	}
	otherInput := validSaveInputForTest(10)
	otherInput.Theme = "other private theme"
	otherSaved := recordRequest(h, "POST", "/learning-records", otherToken.Secret, "application/json", inputJSON(t, otherInput), nil)
	otherID := checkRecordResponse(t, otherSaved, otherInput)
	checkRecordResponse(t, recordRequest(h, "GET", "/learning-records/"+otherID, otherToken.Secret, "", "", nil), otherInput)
	missing := recordRequest(h, "GET", "/learning-records/"+uuid.NewString(), token.Secret, "", "", nil)
	forbidden := recordRequest(h, "GET", "/learning-records/"+otherID, token.Secret, "", "", cookie)
	for _, res := range []*httptest.ResponseRecorder{missing, forbidden} {
		checkListError(t, res, 404, "record_not_found")
	}
	if forbidden.Body.String() != missing.Body.String() {
		t.Fatal("other-owner and missing records must have identical errors")
	}
	checkListError(t, recordRequest(h, "GET", "/learning-records/"+id, otherToken.Secret, "", "", nil), 404, "record_not_found")
	checkListError(t, recordRequest(h, "GET", "/learning-records/invalid", token.Secret, "", "", nil), 400, "invalid_record_id")
	checkListError(t, recordRequest(h, "GET", "/learning-records/"+id, "", "", "", cookie), 401, "unauthorized")
	// Query parameters cannot change the owner selected by Bearer authentication.
	checkListError(t, recordRequest(h, "GET", "/learning-records/"+otherID+"?user_id="+other, token.Secret, "", "", nil), 404, "record_not_found")
	head := recordRequest(h, "HEAD", "/learning-records/"+id, token.Secret, "", "", nil)
	if head.Code != 200 || head.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("HEAD: %d %v", head.Code, head.Header())
	}
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		res := recordRequest(h, method, "/learning-records/"+id, token.Secret, "", "", nil)
		if res.Code != http.StatusMethodNotAllowed {
			t.Fatalf("unregistered method %s: %d", method, res.Code)
		}
	}
	var expires time.Time
	if err := db.QueryRow(`SELECT expires_at FROM connection_tokens WHERE id=$1`, token.Token.ID).Scan(&expires); err != nil || !expires.Equal(token.Token.ExpiresAt) {
		t.Fatal("detail changed expiry", err)
	}
}

func TestLearningGetSecurityHTTPIntegration(t *testing.T) {
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
	if _, err := db.Exec(`DROP TABLE learning_records`); err != nil {
		t.Fatal(err)
	}
	for _, headers := range [][]string{nil, {"Bearer malformed"}, {"Bearer clc_" + strings.Repeat("a", 64)}, {"Bearer " + expired.Secret}, {"Bearer " + revoked.Secret}, {"Bearer " + active.Secret, "Bearer " + active.Secret}} {
		for _, method := range []string{"GET", "HEAD"} {
			req := httptest.NewRequest(method, "/learning-records/invalid", nil)
			for _, value := range headers {
				req.Header.Add("Authorization", value)
			}
			res := httptest.NewRecorder()
			h.ServeHTTP(res, req)
			checkListError(t, res, 401, "unauthorized")
			if res.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing Bearer challenge")
			}
		}
	}
	checkListError(t, recordRequest(h, "GET", "/learning-records/"+uuid.NewString(), active.Secret, "", "", nil), 500, "internal_error")
	if _, err := db.Exec(`DROP TABLE connection_tokens`); err != nil {
		t.Fatal(err)
	}
	res := recordRequest(h, "GET", "/learning-records/invalid", active.Secret, "", "", nil)
	checkListError(t, res, 500, "internal_error")
	if res.Header().Get("WWW-Authenticate") != "" {
		t.Fatal("database failure is not invalid credentials")
	}
}

func TestGetHandlerRequiresTokenContext(t *testing.T) {
	res := httptest.NewRecorder()
	NewHandler(nil).Get(res, httptest.NewRequest("GET", "/learning-records/invalid", nil))
	checkListError(t, res, 401, "unauthorized")
	if res.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatal("missing Bearer challenge")
	}
}
