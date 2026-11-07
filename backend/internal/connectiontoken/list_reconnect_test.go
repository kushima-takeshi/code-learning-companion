package connectiontoken

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// reconnectListScenario builds one owner's tokens at a fixed clock.
// shown is oldest first. List order is the reverse.
type reconnectListScenario struct {
	service *Service
	owner   string
	base    time.Time
	shown   []string
	labels  map[string]string
	secrets []string

	pendingOldID       string
	pendingNewID       string
	pendingSecret      string
	confirmedOldID     string
	confirmedNewID     string
	confirmedSecret    string
	timeExpiredOldID   string
	timeExpiredNewID   string
	timeExpiredSecret  string
	supersededNewID    string
	restartOldID       string
	restartNewID       string
	restartSecret      string
	storedExpiredOldID string
	storedExpiredNewID string
	freshPendingID     string
	freshPendingSecret string
	completedOldID     string
	completedNewID     string
	completedSecret    string
	grandchildID       string
	revokedCompletedID string
	expiredCompletedID string
	usableID           string
	revokedNormalID    string
	expiredNormalID    string
	otherID            string
	otherNewID         string
}

func TestListOmitsIncompleteReconnectTokens(t *testing.T) {
	db := newIntegrationDB(t)
	sc := setupReconnectListScenario(t, db)
	ctx := context.Background()
	list, err := sc.service.List(ctx, sc.owner)
	if err != nil {
		t.Fatal(err)
	}
	got := tokenIDs(list)
	want := reversed(sc.shown)
	byID := make(map[string]Token, len(list))
	for _, token := range list {
		byID[token.ID] = token
	}

	t.Run("見せる鍵だけを新しい順に返す", func(t *testing.T) {
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("一覧:\n got %v\nwant %v", sc.labelIDs(got), sc.labelIDs(want))
		}
	})
	t.Run("始めて10時間のpendingは出さない", func(t *testing.T) {
		if _, ok := byID[sc.pendingNewID]; ok {
			t.Fatal("24時間以内のpendingの置き換え先を一覧に出さない")
		}
		if _, ok := byID[sc.pendingOldID]; !ok {
			t.Fatal("再接続中の旧トークンは一覧に残す")
		}
		if _, err := sc.service.Authenticate(ctx, sc.pendingSecret); err != nil {
			t.Fatalf("期限内の未完了は認証できる: %v", err)
		}
	})
	t.Run("期限内のconfirmedは出さない", func(t *testing.T) {
		if _, ok := byID[sc.confirmedNewID]; ok {
			t.Fatal("confirmedの置き換え先を一覧に出さない")
		}
		if _, ok := byID[sc.confirmedOldID]; !ok {
			t.Fatal("確認済みでも旧トークンは一覧に残す")
		}
		if _, err := sc.service.Authenticate(ctx, sc.confirmedSecret); err != nil {
			t.Fatalf("期限内のconfirmedは認証できる: %v", err)
		}
	})
	t.Run("24時間超過でpendingのままは出さない", func(t *testing.T) {
		if _, ok := byID[sc.timeExpiredNewID]; ok {
			t.Fatal("期限を過ぎても状態がpendingの置き換え先を出さない")
		}
		var revoked sql.NullTime
		if err := db.QueryRow(`SELECT revoked_at FROM connection_tokens WHERE id=$1`, sc.timeExpiredNewID).Scan(&revoked); err != nil {
			t.Fatal(err)
		}
		if revoked.Valid {
			t.Fatal("24時間超過だけでは失効日時は入らない")
		}
		if _, err := sc.service.Authenticate(ctx, sc.timeExpiredSecret); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("期限超過の未完了は認証を拒否する: %v", err)
		}
		if _, ok := byID[sc.timeExpiredOldID]; !ok {
			t.Fatal("旧トークンの期限と失効は変えず一覧に残す")
		}
	})
	t.Run("やり直しで捨てた鍵は出さない", func(t *testing.T) {
		if _, ok := byID[sc.supersededNewID]; ok {
			t.Fatal("supersededの置き換え先を一覧に出さない")
		}
		if _, ok := byID[sc.restartNewID]; ok {
			t.Fatal("やり直しで発行したpendingも一覧に出さない")
		}
		if _, ok := byID[sc.restartOldID]; !ok {
			t.Fatal("やり直し後も旧トークンは一覧に残す")
		}
		if _, err := sc.service.Authenticate(ctx, sc.restartSecret); err != nil {
			t.Fatalf("やり直し後のpendingは期限内なら認証できる: %v", err)
		}
	})
	t.Run("保存状態がexpiredの鍵は出さない", func(t *testing.T) {
		if _, ok := byID[sc.storedExpiredNewID]; ok {
			t.Fatal("stateがexpiredの置き換え先を一覧に出さない")
		}
		if _, ok := byID[sc.freshPendingID]; ok {
			t.Fatal("期限後の開始で発行したpendingも一覧に出さない")
		}
		if _, ok := byID[sc.storedExpiredOldID]; !ok {
			t.Fatal("期限後にやり直しても旧トークンは一覧に残す")
		}
		if _, err := sc.service.Authenticate(ctx, sc.freshPendingSecret); err != nil {
			t.Fatalf("期限後に発行したpendingは期限内なら認証できる: %v", err)
		}
	})
	t.Run("完了した置き換え先は通常の鍵として出す", func(t *testing.T) {
		usable, ok := byID[sc.completedNewID]
		if !ok || usable.RevokedAt != nil || !usable.ExpiresAt.After(sc.base) {
			t.Fatalf("完了済みで期限内・未失効の置き換え先を有効な形で出す: %+v %v", usable, ok)
		}
		revoked, ok := byID[sc.revokedCompletedID]
		if !ok || revoked.RevokedAt == nil || !revoked.ExpiresAt.After(sc.base) {
			t.Fatalf("完了後に失効した置き換え先も失効日時つきで出す: %+v %v", revoked, ok)
		}
		expired, ok := byID[sc.expiredCompletedID]
		if !ok || expired.RevokedAt != nil || !expired.ExpiresAt.Before(sc.base) {
			t.Fatalf("完了後に90日を過ぎた置き換え先も失効なしで出す: %+v %v", expired, ok)
		}
		if _, ok := byID[sc.completedOldID]; !ok {
			t.Fatal("完了で失効した旧トークンは一覧に残す")
		}
		if _, ok := byID[sc.grandchildID]; ok {
			t.Fatal("完了済みの鍵から始めた次のpendingは出さない")
		}
		if _, ok := byID[sc.usableID]; !ok {
			t.Fatal("再接続していない有効な鍵を出す")
		}
		if _, ok := byID[sc.revokedNormalID]; !ok {
			t.Fatal("再接続していない失効済みを出す")
		}
		if _, ok := byID[sc.expiredNormalID]; !ok {
			t.Fatal("再接続していない期限切れを出す")
		}
		if _, err := sc.service.Authenticate(ctx, sc.completedSecret); err != nil {
			t.Fatalf("完了済みの新しい鍵は通常の期限で認証できる: %v", err)
		}
		for _, id := range []string{sc.otherID, sc.otherNewID} {
			if _, ok := byID[id]; ok {
				t.Fatalf("他人の鍵を出さない: %s", sc.labels[id])
			}
		}
	})
	t.Run("一覧に出ない鍵も失効できる", func(t *testing.T) {
		if err := sc.service.Revoke(ctx, sc.owner, sc.restartNewID); err != nil {
			t.Fatal(err)
		}
		if _, err := sc.service.Authenticate(ctx, sc.restartSecret); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("未完了の置き換え先も失効すれば認証できない: %v", err)
		}
		again, err := sc.service.List(ctx, sc.owner)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := indexTokens(again)[sc.restartNewID]; ok {
			t.Fatal("失効しても未完了の置き換え先は一覧に出さない")
		}
		if !reflect.DeepEqual(tokenIDs(again), want) {
			t.Fatalf("失効で見せる鍵の並びを変えない:\n got %v\nwant %v", sc.labelIDs(tokenIDs(again)), sc.labelIDs(want))
		}
	})
}

func TestManagementListOmitsIncompleteReconnectTokens(t *testing.T) {
	db := newIntegrationDB(t)
	sc := setupReconnectListScenario(t, db)
	h := managementHTTP(t, db)
	cookie := webSession(t, db, sc.owner)
	res := managementRequest(h, "GET", "/connection-tokens", "", cookie)
	if res.Code != 200 {
		t.Fatalf("一覧: %d %s", res.Code, res.Body.String())
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("一覧をキャッシュしない")
	}
	var got []tokenResponse
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(got))
	byID := make(map[string]tokenResponse, len(got))
	for i, token := range got {
		ids[i] = token.ID
		byID[token.ID] = token
	}
	want := reversed(sc.shown)
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("HTTP一覧:\n got %v\nwant %v\nbody %s", sc.labelIDs(ids), sc.labelIDs(want), res.Body.String())
	}
	body := res.Body.String()
	for _, secret := range sc.secrets {
		if strings.Contains(body, secret) {
			t.Fatal("一覧に原本を含めない")
		}
	}
	for _, forbidden := range []string{sc.otherID, sc.otherNewID, "secret", "hash", "user_id"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("一覧に他人や内部情報を含めない: %s", forbidden)
		}
	}
	usable := byID[sc.completedNewID]
	if usable.RevokedAt != nil || !usable.ExpiresAt.After(sc.base) {
		t.Fatalf("完了済みの新しい鍵は有効な形: %+v", usable)
	}
	revoked := byID[sc.revokedCompletedID]
	if revoked.RevokedAt == nil || !revoked.ExpiresAt.After(sc.base) {
		t.Fatalf("完了後に失効した鍵は使えない形: %+v", revoked)
	}
	expired := byID[sc.expiredCompletedID]
	if expired.RevokedAt != nil || !expired.ExpiresAt.Before(sc.base) {
		t.Fatalf("完了後に期限切れの鍵は失効なしで期限だけ過ぎている: %+v", expired)
	}
	for _, id := range []string{sc.pendingNewID, sc.confirmedNewID, sc.timeExpiredNewID, sc.supersededNewID, sc.restartNewID, sc.storedExpiredNewID, sc.freshPendingID, sc.grandchildID} {
		if _, ok := byID[id]; ok {
			t.Fatalf("未完了の置き換え先をHTTP一覧に出さない: %s", sc.labels[id])
		}
	}
}

func setupReconnectListScenario(t *testing.T, db *sql.DB) reconnectListScenario {
	t.Helper()
	owner, other := createUser(t, db), createUser(t, db)
	ctx := context.Background()
	base := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	now := base
	service := New(db)
	service.now = func() time.Time { return now }
	sc := reconnectListScenario{
		service: service,
		owner:   owner,
		base:    base,
		labels:  map[string]string{},
	}
	show := func(id, label, secret string) {
		t.Helper()
		sc.labels[id] = label
		sc.shown = append(sc.shown, id)
		if secret != "" {
			sc.secrets = append(sc.secrets, secret)
		}
	}
	hide := func(id, label, secret string) {
		t.Helper()
		sc.labels[id] = label
		if secret != "" {
			sc.secrets = append(sc.secrets, secret)
		}
	}
	issue := func(userID, name string) IssuedToken {
		t.Helper()
		issued, err := service.Issue(ctx, userID, name)
		if err != nil {
			t.Fatal(err)
		}
		return issued
	}
	start := func(userID, oldID string) ReconnectResult {
		t.Helper()
		result, err := service.StartReconnect(ctx, userID, oldID)
		if err != nil {
			t.Fatal(err)
		}
		if result.Secret == "" || result.Operation.State != "pending" {
			t.Fatalf("新規開始: %+v", result.Operation)
		}
		return result
	}

	now = base.Add(-Lifetime - 2*time.Hour)
	expiredNormal := issue(owner, "期限切れ")
	sc.expiredNormalID = expiredNormal.Token.ID
	show(expiredNormal.Token.ID, "再接続していない期限切れ", expiredNormal.Secret)

	now = base.Add(-Lifetime - time.Hour)
	expiredCompletedOld := issue(owner, "完了後に期限切れの旧")
	show(expiredCompletedOld.Token.ID, "完了後に期限切れの旧", expiredCompletedOld.Secret)
	now = now.Add(time.Second)
	expiredCompleted := start(owner, expiredCompletedOld.Token.ID)
	if err := service.ConfirmReconnect(ctx, owner, expiredCompleted.Operation.NewTokenID); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if _, err := service.CompleteReconnect(ctx, owner, expiredCompleted.Operation.ID); err != nil {
		t.Fatal(err)
	}
	sc.expiredCompletedID = expiredCompleted.Operation.NewTokenID
	show(expiredCompleted.Operation.NewTokenID, "完了後に90日超過", expiredCompleted.Secret)

	now = base.Add(-30 * time.Hour)
	storedOld := issue(owner, "expired状態の旧")
	sc.storedExpiredOldID = storedOld.Token.ID
	show(storedOld.Token.ID, "expired状態の旧", storedOld.Secret)
	now = now.Add(time.Second)
	storedNew := start(owner, storedOld.Token.ID)
	now = base.Add(-5 * time.Hour)
	fresh := start(owner, storedOld.Token.ID)
	sc.storedExpiredNewID = storedNew.Operation.NewTokenID
	sc.freshPendingID = fresh.Operation.NewTokenID
	sc.freshPendingSecret = fresh.Secret
	hide(storedNew.Operation.NewTokenID, "保存状態expired", storedNew.Secret)
	hide(fresh.Operation.NewTokenID, "期限後の開始で発行したpending", fresh.Secret)
	mustReconnectState(t, db, storedNew.Operation.NewTokenID, "expired")
	mustReconnectState(t, db, fresh.Operation.NewTokenID, "pending")

	now = base.Add(-25 * time.Hour)
	timeOld := issue(owner, "24時間超過の旧")
	sc.timeExpiredOldID = timeOld.Token.ID
	show(timeOld.Token.ID, "24時間超過の旧", timeOld.Secret)
	now = now.Add(time.Second)
	timeNew := start(owner, timeOld.Token.ID)
	sc.timeExpiredNewID = timeNew.Operation.NewTokenID
	sc.timeExpiredSecret = timeNew.Secret
	hide(timeNew.Operation.NewTokenID, "24時間超過でpendingのまま", timeNew.Secret)
	mustReconnectState(t, db, timeNew.Operation.NewTokenID, "pending")
	var expires time.Time
	if err := db.QueryRow(`SELECT expires_at FROM connection_reconnections WHERE new_token_id=$1`, timeNew.Operation.NewTokenID).Scan(&expires); err != nil {
		t.Fatal(err)
	}
	if !expires.Before(base) && !expires.Equal(base) {
		t.Fatalf("24時間超過の例は観測時点で期限ちょうど以降: %v", expires)
	}

	now = base.Add(-10 * time.Hour)
	pendingOld := issue(owner, "10時間の旧")
	sc.pendingOldID = pendingOld.Token.ID
	show(pendingOld.Token.ID, "10時間の旧", pendingOld.Secret)
	now = now.Add(time.Second)
	pendingNew := start(owner, pendingOld.Token.ID)
	sc.pendingNewID = pendingNew.Operation.NewTokenID
	sc.pendingSecret = pendingNew.Secret
	hide(pendingNew.Operation.NewTokenID, "始めて10時間のpending", pendingNew.Secret)
	var created time.Time
	if err := db.QueryRow(`SELECT created_at, expires_at FROM connection_reconnections WHERE new_token_id=$1`, pendingNew.Operation.NewTokenID).Scan(&created, &expires); err != nil {
		t.Fatal(err)
	}
	if !created.Equal(base.Add(-10*time.Hour+time.Second)) || !expires.After(base) {
		t.Fatalf("始めて約10時間で24時間以内: created=%v expires=%v", created, expires)
	}

	now = base.Add(-9 * time.Hour)
	confirmedOld := issue(owner, "confirmedの旧")
	sc.confirmedOldID = confirmedOld.Token.ID
	show(confirmedOld.Token.ID, "confirmedの旧", confirmedOld.Secret)
	now = now.Add(time.Second)
	confirmedNew := start(owner, confirmedOld.Token.ID)
	if err := service.ConfirmReconnect(ctx, owner, confirmedNew.Operation.NewTokenID); err != nil {
		t.Fatal(err)
	}
	sc.confirmedNewID = confirmedNew.Operation.NewTokenID
	sc.confirmedSecret = confirmedNew.Secret
	hide(confirmedNew.Operation.NewTokenID, "期限内のconfirmed", confirmedNew.Secret)
	mustReconnectState(t, db, confirmedNew.Operation.NewTokenID, "confirmed")

	now = base.Add(-8 * time.Hour)
	restartOld := issue(owner, "やり直しの旧")
	sc.restartOldID = restartOld.Token.ID
	show(restartOld.Token.ID, "やり直しの旧", restartOld.Secret)
	now = now.Add(time.Second)
	abandoned := start(owner, restartOld.Token.ID)
	now = now.Add(time.Second)
	restarted, err := service.RestartReconnect(ctx, owner, abandoned.Operation.ID)
	if err != nil || restarted.Secret == "" || restarted.Operation.State != "pending" {
		t.Fatalf("やり直し: %+v %v", restarted.Operation, err)
	}
	sc.supersededNewID = abandoned.Operation.NewTokenID
	sc.restartNewID = restarted.Operation.NewTokenID
	sc.restartSecret = restarted.Secret
	hide(abandoned.Operation.NewTokenID, "やり直しで捨てたsuperseded", abandoned.Secret)
	hide(restarted.Operation.NewTokenID, "やり直しで発行したpending", restarted.Secret)
	mustReconnectState(t, db, abandoned.Operation.NewTokenID, "superseded")

	now = base.Add(-4 * time.Hour)
	completedOld := issue(owner, "完了の旧")
	sc.completedOldID = completedOld.Token.ID
	show(completedOld.Token.ID, "完了の旧", completedOld.Secret)
	now = now.Add(time.Second)
	completed := start(owner, completedOld.Token.ID)
	if err := service.ConfirmReconnect(ctx, owner, completed.Operation.NewTokenID); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if _, err := service.CompleteReconnect(ctx, owner, completed.Operation.ID); err != nil {
		t.Fatal(err)
	}
	sc.completedNewID = completed.Operation.NewTokenID
	sc.completedSecret = completed.Secret
	show(completed.Operation.NewTokenID, "完了した新しい鍵", completed.Secret)
	now = now.Add(time.Second)
	grandchild := start(owner, completed.Operation.NewTokenID)
	sc.grandchildID = grandchild.Operation.NewTokenID
	hide(grandchild.Operation.NewTokenID, "完了後に始めたpending", grandchild.Secret)
	mustReconnectState(t, db, completed.Operation.NewTokenID, "completed")

	now = base.Add(-3 * time.Hour)
	revokedCompletedOld := issue(owner, "完了後失効の旧")
	show(revokedCompletedOld.Token.ID, "完了後失効の旧", revokedCompletedOld.Secret)
	now = now.Add(time.Second)
	revokedCompleted := start(owner, revokedCompletedOld.Token.ID)
	if err := service.ConfirmReconnect(ctx, owner, revokedCompleted.Operation.NewTokenID); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if _, err := service.CompleteReconnect(ctx, owner, revokedCompleted.Operation.ID); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if err := service.Revoke(ctx, owner, revokedCompleted.Operation.NewTokenID); err != nil {
		t.Fatal(err)
	}
	sc.revokedCompletedID = revokedCompleted.Operation.NewTokenID
	show(revokedCompleted.Operation.NewTokenID, "完了後に失効", revokedCompleted.Secret)
	mustReconnectState(t, db, revokedCompleted.Operation.NewTokenID, "completed")

	now = base.Add(-2 * time.Hour)
	revokedNormal := issue(owner, "失効済み")
	if err := service.Revoke(ctx, owner, revokedNormal.Token.ID); err != nil {
		t.Fatal(err)
	}
	sc.revokedNormalID = revokedNormal.Token.ID
	show(revokedNormal.Token.ID, "再接続していない失効済み", revokedNormal.Secret)

	now = base.Add(-time.Hour)
	usable := issue(owner, "有効")
	sc.usableID = usable.Token.ID
	show(usable.Token.ID, "再接続していない有効", usable.Secret)

	now = base.Add(-30 * time.Minute)
	foreign := issue(other, "他人")
	now = now.Add(time.Second)
	foreignNew := start(other, foreign.Token.ID)
	sc.otherID = foreign.Token.ID
	sc.otherNewID = foreignNew.Operation.NewTokenID
	hide(foreign.Token.ID, "他人の鍵", foreign.Secret)
	hide(foreignNew.Operation.NewTokenID, "他人のpending", foreignNew.Secret)

	now = base
	return sc
}

func mustReconnectState(t *testing.T, db *sql.DB, newID, state string) {
	t.Helper()
	var got string
	if err := db.QueryRow(`SELECT state FROM connection_reconnections WHERE new_token_id=$1`, newID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != state {
		t.Fatalf("new_token %s の状態が %s、期待は %s", newID, got, state)
	}
}

func (sc reconnectListScenario) labelIDs(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		label := sc.labels[id]
		if label == "" {
			label = id
		}
		out[i] = label
	}
	return out
}

func tokenIDs(tokens []Token) []string {
	ids := make([]string, len(tokens))
	for i, token := range tokens {
		ids[i] = token.ID
	}
	return ids
}

func indexTokens(tokens []Token) map[string]Token {
	byID := make(map[string]Token, len(tokens))
	for _, token := range tokens {
		byID[token.ID] = token
	}
	return byID
}

func reversed(ids []string) []string {
	out := make([]string, len(ids))
	for i := range ids {
		out[i] = ids[len(ids)-1-i]
	}
	return out
}
