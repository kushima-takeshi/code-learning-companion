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

func TestManagementRenameIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	owner, other := createUser(t, db), createUser(t, db)
	h := managementHTTP(t, db)
	cookie := webSession(t, db, owner)
	s := New(db)
	issued, err := s.Issue(context.Background(), owner, "codex")
	if err != nil {
		t.Fatal(err)
	}
	kept, err := s.Issue(context.Background(), owner, "残す接続")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := s.Issue(context.Background(), other, "別ユーザー")
	if err != nil {
		t.Fatal(err)
	}

	path := "/connection-tokens/" + strings.ToUpper(issued.Token.ID) + "/rename?user_id=" + other
	res := managementRequest(h, "POST", path, `{"name":"  cursor  "}`, cookie)
	if res.Code != http.StatusOK || res.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("名前変更: %d %s", res.Code, res.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(res.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["secret"]; ok || strings.Contains(res.Body.String(), issued.Secret) {
		t.Fatal("名前変更の応答に原本を含めない")
	}
	var got struct {
		Token tokenResponse `json:"token"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Token.ID != issued.Token.ID || got.Token.Name != "cursor" || got.Token.RevokedAt != nil || !got.Token.CreatedAt.Equal(issued.Token.CreatedAt) || !got.Token.ExpiresAt.Equal(issued.Token.ExpiresAt) {
		t.Fatalf("メタデータは表示名だけ変わる: %+v", got.Token)
	}
	for _, forbidden := range []string{issued.Secret, "token_hash", "user_id", foreign.Token.ID} {
		if strings.Contains(res.Body.String(), forbidden) {
			t.Fatalf("内部情報を返さない: %s", forbidden)
		}
	}
	authenticated, err := s.Authenticate(context.Background(), issued.Secret)
	if err != nil || authenticated.Name != "cursor" {
		t.Fatalf("原本はそのまま照合できる: %+v %v", authenticated, err)
	}
	var keptName, foreignName string
	if err := db.QueryRow(`SELECT name FROM connection_tokens WHERE id=$1`, kept.Token.ID).Scan(&keptName); err != nil || keptName != "残す接続" {
		t.Fatalf("別の接続には影響しない: %s %v", keptName, err)
	}
	if err := db.QueryRow(`SELECT name FROM connection_tokens WHERE id=$1`, foreign.Token.ID).Scan(&foreignName); err != nil || foreignName != "別ユーザー" {
		t.Fatalf("他人の接続名を変えない: %s %v", foreignName, err)
	}

	res = managementRequest(h, "POST", "/connection-tokens/"+issued.Token.ID+"/rename", `{"name":"cursor"}`, cookie)
	if res.Code != 200 {
		t.Fatalf("同じ名前への変更: %d %s", res.Code, res.Body.String())
	}
	charsetReq := httptest.NewRequest("POST", "/connection-tokens/"+issued.Token.ID+"/rename", strings.NewReader(`{"name":"cursor"}`))
	charsetReq.Header.Set("Content-Type", "application/json; charset=utf-8")
	charsetReq.Header.Set("Origin", webOrigin)
	charsetReq.AddCookie(cookie)
	charsetRes := httptest.NewRecorder()
	h.ServeHTTP(charsetRes, charsetReq)
	if charsetRes.Code != 200 {
		t.Fatalf("charset付きJSON: %d %s", charsetRes.Code, charsetRes.Body.String())
	}
	for _, id := range []string{foreign.Token.ID, uuid.NewString()} {
		checkHTTPError(t, managementRequest(h, "POST", "/connection-tokens/"+id+"/rename", `{"name":"乗っ取り"}`, cookie), 404, "token_not_found")
	}
	checkHTTPError(t, managementRequest(h, "POST", "/connection-tokens/not-a-uuid/rename", `{"name":"x"}`, cookie), 400, "invalid_token_id")
	if err := db.QueryRow(`SELECT name FROM connection_tokens WHERE id=$1`, foreign.Token.ID).Scan(&foreignName); err != nil || foreignName != "別ユーザー" {
		t.Fatalf("404のあと他人の名前が残る: %s %v", foreignName, err)
	}

	revoked, err := s.Issue(context.Background(), owner, "失効済み")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Revoke(context.Background(), owner, revoked.Token.ID); err != nil {
		t.Fatal(err)
	}
	var revokedAt time.Time
	if err := db.QueryRow(`SELECT revoked_at FROM connection_tokens WHERE id=$1`, revoked.Token.ID).Scan(&revokedAt); err != nil {
		t.Fatal(err)
	}
	res = managementRequest(h, "POST", "/connection-tokens/"+revoked.Token.ID+"/rename", `{"name":"失効後の名前"}`, cookie)
	if res.Code != 200 {
		t.Fatalf("失効済みの名前変更: %d %s", res.Code, res.Body.String())
	}
	var again time.Time
	var revokedName string
	if err := db.QueryRow(`SELECT name, revoked_at FROM connection_tokens WHERE id=$1`, revoked.Token.ID).Scan(&revokedName, &again); err != nil || revokedName != "失効後の名前" || !again.Equal(revokedAt) {
		t.Fatalf("失効日時を動かさない: %s %v %v", revokedName, again, err)
	}

	past := New(db)
	past.now = func() time.Time { return time.Now().Add(-Lifetime - time.Hour) }
	expired, err := past.Issue(context.Background(), owner, "期限切れ")
	if err != nil {
		t.Fatal(err)
	}
	res = managementRequest(h, "POST", "/connection-tokens/"+expired.Token.ID+"/rename", `{"name":"期限後の名前"}`, cookie)
	if res.Code != 200 {
		t.Fatalf("期限切れの名前変更: %d %s", res.Code, res.Body.String())
	}
	var expiredName string
	var expiresAt time.Time
	if err := db.QueryRow(`SELECT name, expires_at FROM connection_tokens WHERE id=$1`, expired.Token.ID).Scan(&expiredName, &expiresAt); err != nil || expiredName != "期限後の名前" || !expiresAt.Equal(expired.Token.ExpiresAt) {
		t.Fatalf("期限を動かさない: %s %v %v", expiredName, expiresAt, err)
	}
}

func TestManagementRenameInvalidIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	owner := createUser(t, db)
	h := managementHTTP(t, db)
	cookie := webSession(t, db, owner)
	issued, err := New(db).Issue(context.Background(), owner, "codex")
	if err != nil {
		t.Fatal(err)
	}
	path := "/connection-tokens/" + issued.Token.ID + "/rename"
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
		{"所有者偽装", `{"name":"cursor","user_id":"other"}`, "invalid_json", 400},
		{"末尾JSON", `{"name":"cursor"} {}`, "invalid_json", 400},
		{"大きな要求", `{"name":"` + strings.Repeat("a", 4096) + `"}`, "request_too_large", 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checkHTTPError(t, managementRequest(h, "POST", path, tc.body, cookie), tc.status, tc.code)
		})
	}
	req := httptest.NewRequest("POST", path, strings.NewReader(`{"name":"cursor"}`))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Origin", webOrigin)
	req.AddCookie(cookie)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	checkHTTPError(t, res, 415, "unsupported_media_type")
	var name string
	if err := db.QueryRow(`SELECT name FROM connection_tokens WHERE id=$1`, issued.Token.ID).Scan(&name); err != nil || name != "codex" {
		t.Fatalf("不正入力では名前を変えない: %s %v", name, err)
	}
}

func TestManagementRenameBodyLimitIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	owner := createUser(t, db)
	h := managementHTTP(t, db)
	cookie := webSession(t, db, owner)
	issued, err := New(db).Issue(context.Background(), owner, "codex")
	if err != nil {
		t.Fatal(err)
	}
	path := "/connection-tokens/" + issued.Token.ID + "/rename"
	body := `{"name":"cursor"}`
	body += strings.Repeat(" ", 4096-len(body))
	res := managementRequest(h, "POST", path, body, cookie)
	if res.Code != 200 {
		t.Fatalf("4096バイトは受け付ける: %d %s", res.Code, res.Body.String())
	}
	checkHTTPError(t, managementRequest(h, "POST", path, body+" ", cookie), 413, "request_too_large")
	var name string
	if err := db.QueryRow(`SELECT name FROM connection_tokens WHERE id=$1`, issued.Token.ID).Scan(&name); err != nil || name != "cursor" {
		t.Fatalf("超過分で名前を戻さない: %s %v", name, err)
	}
}
