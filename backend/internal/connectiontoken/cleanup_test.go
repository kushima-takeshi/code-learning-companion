package connectiontoken

import (
	"context"
	"database/sql"
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
