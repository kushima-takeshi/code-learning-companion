package connectiontoken

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCleanupUnusableTokens(t *testing.T) {
	db := newIntegrationDB(t)
	applyMigration(t, db, "000004_learning_records.up.sql")
	owner := createUser(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	now := time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)
	cutoff := now.Add(-UnusableRetention)
	justUnder := cutoff.Add(time.Microsecond)
	justOver := cutoff.Add(-time.Microsecond)

	keptUnder := insertToken(t, db, owner, 1, justUnder.Add(-time.Hour), justUnder, nil)
	deletedOver := insertToken(t, db, owner, 2, justOver.Add(-time.Hour), justOver, nil)
	keptExact := insertToken(t, db, owner, 3, cutoff.Add(-time.Hour), cutoff, nil)
	future := now.Add(60 * 24 * time.Hour)
	revokedAt := justOver
	deletedRevoked := insertToken(t, db, owner, 4, revokedAt.Add(-time.Hour), future, &revokedAt)
	keptUsable := insertToken(t, db, owner, 5, now, now.Add(Lifetime), nil)

	// Completed replacement: the 24h mark is already past 30 days, but the new
	// token's own expiry is not. The old token is unusable from revoked_at.
	reconnectCreated := justOver.Add(-ReconnectLifetime)
	keptReplacement := insertToken(t, db, owner, 6, reconnectCreated, reconnectCreated.Add(Lifetime), nil)
	confirmedAt := reconnectCreated
	completedAt := reconnectCreated.Add(time.Hour)
	insertReconnect(t, db, owner, deletedRevoked, keptReplacement, "completed", reconnectCreated, &confirmedAt, &completedAt)

	// Pending past 24h. The new token's own expiry is still in the future.
	pendingCreated := justOver.Add(-ReconnectLifetime)
	deletedPendingNew := insertToken(t, db, owner, 7, pendingCreated, pendingCreated.Add(Lifetime), nil)
	insertReconnect(t, db, owner, keptUsable, deletedPendingNew, "pending", pendingCreated, nil, nil)

	if _, err := db.ExecContext(ctx, `
		INSERT INTO learning_records (user_id, request_id, theme, questions)
		VALUES ($1, $2, '保持', '[]'::jsonb)
	`, owner, uuid.NewString()); err != nil {
		t.Fatal(err)
	}

	service := New(db)
	got, err := service.CleanupUnusableTokens(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tokens != 3 || got.Reconnections != 2 {
		t.Fatalf("削除件数: tokens=%d reconnections=%d", got.Tokens, got.Reconnections)
	}
	for _, id := range []string{keptUnder, keptExact, keptUsable, keptReplacement} {
		if !rowExists(t, db, "connection_tokens", id) {
			t.Fatalf("残すトークンが消えた: %s", id)
		}
	}
	for _, id := range []string{deletedOver, deletedRevoked, deletedPendingNew} {
		if rowExists(t, db, "connection_tokens", id) {
			t.Fatalf("消すトークンが残った: %s", id)
		}
	}
	if countRows(t, db, "connection_reconnections") != 0 {
		t.Fatal("対象に紐づく再接続行が残った")
	}
	if countRows(t, db, "learning_records") != 1 || countRows(t, db, "users") != 1 {
		t.Fatal("学習記録かユーザーを消した")
	}

	again, err := service.CleanupUnusableTokens(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if again.Tokens != 0 || again.Reconnections != 0 {
		t.Fatalf("2回目は何も消さない: %+v", again)
	}
	if countRows(t, db, "connection_tokens") != 4 || countRows(t, db, "learning_records") != 1 {
		t.Fatal("2回目で残件が変わった")
	}
}

func TestCleanupDeletesUnfinishedReplacementWithOldToken(t *testing.T) {
	db := newIntegrationDB(t)
	owner := createUser(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	now := time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)
	cutoff := now.Add(-UnusableRetention)
	oldExpires := cutoff.Add(-time.Microsecond)
	started := oldExpires.Add(-time.Hour)
	if window := started.Add(ReconnectLifetime); !window.After(cutoff) || !window.Before(now) {
		t.Fatal("24時間の起点が30日以内で、かつ今より前であることを前提にする")
	}

	old := insertToken(t, db, owner, 1, started.Add(-time.Hour), oldExpires, nil)
	pendingSecret := "clc_" + strings.Repeat("ab", 32)
	pendingNew := insertTokenSecret(t, db, owner, pendingSecret, started, started.Add(Lifetime))
	insertReconnect(t, db, owner, old, pendingNew, "pending", started, nil, nil)

	completedOldExpires := cutoff.Add(-time.Hour)
	completedStart := completedOldExpires.Add(-time.Hour)
	completedOld := insertToken(t, db, owner, 2, completedStart.Add(-time.Hour), completedOldExpires, nil)
	keptSecret := "clc_" + strings.Repeat("cd", 32)
	keptNew := insertTokenSecret(t, db, owner, keptSecret, completedStart, completedStart.Add(Lifetime))
	confirmedAt := completedStart
	completedAt := completedStart.Add(time.Hour)
	insertReconnect(t, db, owner, completedOld, keptNew, "completed", completedStart, &confirmedAt, &completedAt)

	service := New(db)
	service.now = func() time.Time { return now }
	if _, err := service.CleanupUnusableTokens(ctx, now); err != nil {
		t.Fatal(err)
	}
	if rowExists(t, db, "connection_tokens", old) || rowExists(t, db, "connection_tokens", pendingNew) || rowExists(t, db, "connection_tokens", completedOld) {
		t.Fatal("期限を過ぎた旧トークンか、未完了の置き換え先が残った")
	}
	if !rowExists(t, db, "connection_tokens", keptNew) {
		t.Fatal("完了した置き換え先を消した")
	}
	if _, err := service.Authenticate(ctx, pendingSecret); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("消した置き換え先は認証できない: %v", err)
	}
	got, err := service.Authenticate(ctx, keptSecret)
	if err != nil || got.ID != keptNew {
		t.Fatalf("完了した置き換え先は使える: %+v %v", got, err)
	}
}

func insertToken(t *testing.T, db *sql.DB, userID string, mark byte, created, expires time.Time, revoked *time.Time) string {
	t.Helper()
	hash := make([]byte, 32)
	hash[0] = mark
	var id string
	err := db.QueryRow(`
		INSERT INTO connection_tokens (user_id, name, token_hash, created_at, expires_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, userID, "接続", hash, created, expires, revoked).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertTokenSecret(t *testing.T, db *sql.DB, userID, secret string, created, expires time.Time) string {
	t.Helper()
	hash := sha256.Sum256([]byte(secret))
	var id string
	err := db.QueryRow(`
		INSERT INTO connection_tokens (user_id, name, token_hash, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, userID, "接続", hash[:], created, expires).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertReconnect(t *testing.T, db *sql.DB, userID, oldID, newID, state string, created time.Time, confirmed, completed *time.Time) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO connection_reconnections
			(user_id, old_token_id, new_token_id, state, created_at, expires_at, confirmed_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, userID, oldID, newID, state, created, created.Add(ReconnectLifetime), confirmed, completed)
	if err != nil {
		t.Fatal(err)
	}
}

func rowExists(t *testing.T, db *sql.DB, table, id string) bool {
	t.Helper()
	var exists bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM `+table+` WHERE id=$1)`, id).Scan(&exists)
	if err != nil {
		t.Fatal(err)
	}
	return exists
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
