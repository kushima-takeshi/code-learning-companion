package connectiontoken

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestManagementListIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	owner, other := createUser(t, db), createUser(t, db)
	h := managementHTTP(t, db)
	cookie := webSession(t, db, owner)
	res := managementRequest(h, "GET", "/connection-tokens", "", cookie)
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != "[]" {
		t.Fatalf("空一覧: %d %s", res.Code, res.Body.String())
	}
	s := New(db)
	s.now = func() time.Time { return time.Now().Add(-Lifetime - 48*time.Hour) }
	first, err := s.Issue(context.Background(), owner, "first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Issue(context.Background(), owner, "second")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := s.Issue(context.Background(), other, "other")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Revoke(context.Background(), owner, first.Token.ID); err != nil {
		t.Fatal(err)
	}

	res = managementRequest(h, "GET", "/connection-tokens", "", cookie)
	if res.Code != 200 {
		t.Fatalf("一覧: %d %s", res.Code, res.Body.String())
	}
	var got []tokenResponse
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != second.Token.ID || got[1].ID != first.Token.ID || got[1].RevokedAt == nil || !got[0].ExpiresAt.Before(time.Now()) {
		t.Fatalf("本人だけの新しい順・失効済み/期限切れも含む: %+v", got)
	}
	for _, forbidden := range []string{first.Secret, second.Secret, foreign.Secret, foreign.Token.ID, "secret", "hash", "user_id"} {
		if strings.Contains(res.Body.String(), forbidden) {
			t.Fatalf("一覧に原本・他人・内部情報を含めない: %s", forbidden)
		}
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("一覧をキャッシュしない")
	}
}

func TestManagementRevokeIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	owner, other := createUser(t, db), createUser(t, db)
	h := managementHTTP(t, db)
	cookie := webSession(t, db, owner)
	s := New(db)
	first, err := s.Issue(context.Background(), owner, "first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Issue(context.Background(), owner, "second")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := s.Issue(context.Background(), other, "other")
	if err != nil {
		t.Fatal(err)
	}
	path := "/connection-tokens/" + first.Token.ID + "/revoke"
	res := managementRequest(h, "POST", path, "", cookie)
	if res.Code != http.StatusNoContent || res.Body.Len() != 0 || res.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("失効: %d %s", res.Code, res.Body.String())
	}
	var at time.Time
	if err := db.QueryRow(`SELECT revoked_at FROM connection_tokens WHERE id=$1`, first.Token.ID).Scan(&at); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(context.Background(), first.Secret); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("失効後は照合不可: %v", err)
	}
	res = managementRequest(h, "POST", path, "", cookie)
	if res.Code != 204 {
		t.Fatalf("再失効: %d", res.Code)
	}
	var again time.Time
	if err := db.QueryRow(`SELECT revoked_at FROM connection_tokens WHERE id=$1`, first.Token.ID).Scan(&again); err != nil || !at.Equal(again) {
		t.Fatalf("再失効で日時を変えない: %v", err)
	}
	for _, id := range []string{foreign.Token.ID, uuid.NewString()} {
		checkHTTPError(t, managementRequest(h, "POST", "/connection-tokens/"+id+"/revoke", "", cookie), 404, "token_not_found")
	}
	checkHTTPError(t, managementRequest(h, "POST", "/connection-tokens/not-a-uuid/revoke", "", cookie), 400, "invalid_token_id")
	for _, secret := range []string{second.Secret, foreign.Secret} {
		if _, err := s.Authenticate(context.Background(), secret); err != nil {
			t.Fatalf("別の接続には影響しない: %v", err)
		}
	}
}
