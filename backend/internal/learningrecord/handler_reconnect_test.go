package learningrecord

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
)

func TestLearningListConfirmsReconnectHTTPIntegration(t *testing.T) {
	db, h, tokens := newListHTTP(t)
	ctx := context.Background()
	owner, other := listUser(t, db, 1), listUser(t, db, 2)
	old, err := tokens.Issue(ctx, owner, "AIエージェント")
	if err != nil {
		t.Fatal(err)
	}
	otherToken, err := tokens.Issue(ctx, other, "AIエージェント")
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := tokens.StartReconnect(ctx, owner, old.Token.ID)
	if err != nil {
		t.Fatal(err)
	}
	state := func(want string) {
		t.Helper()
		op, err := tokens.GetReconnect(ctx, owner, attempt.Operation.ID)
		if err != nil || op.State != want {
			t.Fatalf("state=%s want=%s err=%v", op.State, want, err)
		}
	}
	state("pending")
	// HEAD, invalid pagination and a different token must not confirm this operation.
	for _, tc := range []struct {
		method, path, secret string
		status               int
	}{
		{"HEAD", "/learning-records", attempt.Secret, 200},
		{"GET", "/learning-records?limit=bad", attempt.Secret, 400},
		{"GET", "/learning-records", old.Secret, 200},
		{"GET", "/learning-records", otherToken.Secret, 200},
	} {
		res := listRequest(h, tc.method, tc.path, tc.secret, nil)
		if res.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, res.Code, res.Body.String())
		}
		state("pending")
	}
	// POST and detail reads are not connection confirmation, even when they succeed.
	data := inputJSON(t, saveTestInput())
	saved := recordRequest(h, "POST", "/learning-records", attempt.Secret, "application/json", data, nil)
	if saved.Code != 200 {
		t.Fatalf("save: %d %s", saved.Code, saved.Body.String())
	}
	var record recordResponse
	if err := json.Unmarshal(saved.Body.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if res := listRequest(h, "GET", "/learning-records/"+record.ID, attempt.Secret, nil); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	state("pending")
	// Empty-list success also confirms, so use an owner with no saved records next.
	emptyOwner := listUser(t, db, 3)
	emptyOld, err := tokens.Issue(ctx, emptyOwner, "empty")
	if err != nil {
		t.Fatal(err)
	}
	empty, err := tokens.StartReconnect(ctx, emptyOwner, emptyOld.Token.ID)
	if err != nil {
		t.Fatal(err)
	}
	res := listRequest(h, "GET", "/learning-records", empty.Secret, nil)
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != `{"records":[],"has_more":false}` {
		t.Fatalf("empty confirmation: %d %s", res.Code, res.Body.String())
	}
	emptyState, err := tokens.GetReconnect(ctx, emptyOwner, empty.Operation.ID)
	if err != nil || emptyState.State != "confirmed" {
		t.Fatalf("empty state: %+v %v", emptyState, err)
	}
	// The supplied cookie belongs to someone else; Bearer identity still wins.
	session, hash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(db).CreateSession(ctx, other, hash, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	res = listRequest(h, "GET", "/learning-records", attempt.Secret, &http.Cookie{Name: "session", Value: session})
	if res.Code != 200 {
		t.Fatalf("confirmation: %d %s", res.Code, res.Body.String())
	}
	state("confirmed")
	// Confirmation never revokes the old connection itself.
	if _, err := tokens.Authenticate(ctx, old.Secret); err != nil {
		t.Fatal(err)
	}
	// Finish through the Web-authenticated route.
	ownerSession, ownerHash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(db).CreateSession(ctx, owner, ownerHash, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	path := "/connection-reconnections/" + attempt.Operation.ID + "/complete"
	checkListError(t, listRequest(h, "POST", path, attempt.Secret, nil), 401, "unauthorized")
	res = listRequest(h, "POST", path, "", &http.Cookie{Name: "session", Value: ownerSession})
	if res.Code != 200 {
		t.Fatalf("complete: %d %s", res.Code, res.Body.String())
	}
	state("completed")
	checkListError(t, listRequest(h, "GET", "/learning-records", old.Secret, nil), 401, "unauthorized")
	if res := listRequest(h, "GET", "/learning-records", attempt.Secret, nil); res.Code != 200 {
		t.Fatalf("completed token list: %d %s", res.Code, res.Body.String())
	}
	// No confirmation-only learning records were created.
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM learning_records`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("dummy records: %d %v", count, err)
	}
}

func TestLearningListReconnectFailureHTTPIntegration(t *testing.T) {
	db, h, tokens := newListHTTP(t)
	ctx := context.Background()
	owner := listUser(t, db, 1)
	old, err := tokens.Issue(ctx, owner, "PC")
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := tokens.StartReconnect(ctx, owner, old.Token.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Force failure only when the list handler tries to persist confirmation.
	if _, err := db.Exec(`CREATE FUNCTION reject_confirmation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'private DB failure'; END $$; CREATE TRIGGER reject_confirmation BEFORE UPDATE ON connection_reconnections FOR EACH ROW EXECUTE FUNCTION reject_confirmation()`); err != nil {
		t.Fatal(err)
	}
	checkListError(t, listRequest(h, "GET", "/learning-records", attempt.Secret, nil), 500, "internal_error")
	op, err := tokens.GetReconnect(ctx, owner, attempt.Operation.ID)
	if err != nil || op.State != "pending" {
		t.Fatalf("failed update: %+v %v", op, err)
	}
	if _, err := tokens.Authenticate(ctx, old.Secret); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TRIGGER reject_confirmation ON connection_reconnections; DROP TABLE learning_records`); err != nil {
		t.Fatal(err)
	}
	checkListError(t, listRequest(h, "GET", "/learning-records", attempt.Secret, nil), 500, "internal_error")
	op, err = tokens.GetReconnect(ctx, owner, attempt.Operation.ID)
	if err != nil || op.State != "pending" {
		t.Fatalf("list DB failure confirmed: %+v %v", op, err)
	}
}

func TestLearningReconnectDeadlineHTTPIntegration(t *testing.T) {
	db, h, tokens := newListHTTP(t)
	ctx := context.Background()
	owner := listUser(t, db, 1)
	old, err := tokens.Issue(ctx, owner, "PC")
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := tokens.StartReconnect(ctx, owner, old.Token.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Age the operation, preserving all DB constraints, without waiting in real time.
	if _, err := db.Exec(`UPDATE connection_reconnections SET created_at=created_at-interval '25 hours',expires_at=expires_at-interval '25 hours' WHERE id=$1`, attempt.Operation.ID); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, path string }{{"GET", "/learning-records"}, {"POST", "/learning-records"}, {"GET", "/learning-records/invalid"}} {
		checkListError(t, listRequest(h, tc.method, tc.path, attempt.Secret, nil), 401, "unauthorized")
	}
	if res := listRequest(h, "GET", "/learning-records", old.Secret, nil); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	// No browser or MCP runtime is involved in these registered-route HTTP tests.
}
