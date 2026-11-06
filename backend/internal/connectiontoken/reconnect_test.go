package connectiontoken

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestReconnectLifecycleIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	s := New(db)
	now := time.Date(2026, 11, 2, 3, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	ctx := context.Background()
	owner, other := createUser(t, db), createUser(t, db)
	old, err := s.Issue(ctx, owner, "AIエージェント")
	if err != nil {
		t.Fatal(err)
	}
	unrelated, err := s.Issue(ctx, owner, "AIエージェント")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{old.Token.ID, uuid.NewString()} {
		if _, err := s.StartReconnect(ctx, other, id); !errors.Is(err, ErrTokenNotFound) {
			t.Fatalf("owner isolation: %v", err)
		}
	}
	started, err := s.StartReconnect(ctx, owner, old.Token.ID)
	if err != nil {
		t.Fatal(err)
	}
	if started.Secret == "" || started.Operation.State != "pending" || !started.Operation.ExpiresAt.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("start: %+v", started.Operation)
	}
	duplicate, err := s.StartReconnect(ctx, owner, old.Token.ID)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.Secret != "" || duplicate.Operation.ID != started.Operation.ID {
		t.Fatal("duplicate start must return state without secret")
	}
	if _, err := s.StartReconnect(ctx, owner, started.Operation.NewTokenID); !errors.Is(err, ErrReconnectConflict) {
		t.Fatalf("provisional token cannot be replacement target: %v", err)
	}
	if _, err := s.GetReconnect(ctx, other, started.Operation.ID); !errors.Is(err, ErrReconnectNotFound) {
		t.Fatalf("status ownership: %v", err)
	}
	if _, err := s.CompleteReconnect(ctx, owner, started.Operation.ID); !errors.Is(err, ErrReconnectConflict) {
		t.Fatalf("unconfirmed completion: %v", err)
	}
	for _, secret := range []string{old.Secret, started.Secret, unrelated.Secret} {
		if _, err := s.Authenticate(ctx, secret); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ConfirmReconnect(ctx, owner, old.Token.ID); err != nil {
		t.Fatal(err)
	}
	op, err := s.GetReconnect(ctx, owner, started.Operation.ID)
	if err != nil || op.State != "pending" {
		t.Fatalf("old token must not confirm: %+v %v", op, err)
	}
	if err := s.ConfirmReconnect(ctx, owner, started.Operation.NewTokenID); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	completed, err := s.CompleteReconnect(ctx, owner, started.Operation.ID)
	if err != nil || completed.State != "completed" {
		t.Fatalf("completion: %+v %v", completed, err)
	}
	if _, err := s.Authenticate(ctx, old.Secret); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("old token still active: %v", err)
	}
	for _, secret := range []string{started.Secret, unrelated.Secret} {
		if _, err := s.Authenticate(ctx, secret); err != nil {
			t.Fatal(err)
		}
	}
	again, err := s.CompleteReconnect(ctx, owner, completed.ID)
	if err != nil || !again.CompletedAt.Equal(*completed.CompletedAt) {
		t.Fatalf("completion retry: %+v %v", again, err)
	}
	duplicate, err = s.StartReconnect(ctx, owner, old.Token.ID)
	if err != nil || duplicate.Operation.State != "completed" || duplicate.Secret != "" {
		t.Fatalf("completed start retry: %+v %v", duplicate, err)
	}
	now = now.Add(25 * time.Hour)
	if _, err := s.Authenticate(ctx, started.Secret); err != nil {
		t.Fatalf("completed token keeps 90 days: %v", err)
	}
	var expires time.Time
	if err := db.QueryRow(`SELECT expires_at FROM connection_tokens WHERE id=$1`, completed.NewTokenID).Scan(&expires); err != nil || !expires.Equal(started.Operation.CreatedAt.Add(Lifetime)) {
		t.Fatalf("90 day deadline changed: %v %v", expires, err)
	}
}

func TestReconnectRestartAndExpiryIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	s := New(db)
	ctx := context.Background()
	owner := createUser(t, db)
	now := time.Date(2026, 11, 2, 3, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	old, err := s.Issue(ctx, owner, "PC")
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.StartReconnect(ctx, owner, old.Token.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmReconnect(ctx, owner, first.Operation.NewTokenID); err != nil {
		t.Fatal(err)
	}
	second, err := s.RestartReconnect(ctx, owner, first.Operation.ID)
	if err != nil || second.Secret == "" {
		t.Fatalf("restart: %v", err)
	}
	if _, err := s.Authenticate(ctx, first.Secret); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("abandoned token active: %v", err)
	}
	if _, err := s.CompleteReconnect(ctx, owner, first.Operation.ID); !errors.Is(err, ErrReconnectConflict) {
		t.Fatalf("stale complete: %v", err)
	}
	if _, err := s.RestartReconnect(ctx, owner, first.Operation.ID); !errors.Is(err, ErrReconnectConflict) {
		t.Fatalf("stale restart: %v", err)
	}
	if err := s.ConfirmReconnect(ctx, owner, second.Operation.NewTokenID); err != nil {
		t.Fatal(err)
	}
	now = second.Operation.ExpiresAt.Add(-time.Microsecond)
	if _, err := s.Authenticate(ctx, second.Secret); err != nil {
		t.Fatal(err)
	}
	now = second.Operation.ExpiresAt
	if _, err := s.Authenticate(ctx, second.Secret); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("exact 24h expiry: %v", err)
	}
	op, err := s.GetReconnect(ctx, owner, second.Operation.ID)
	if err != nil || op.State != "expired" {
		t.Fatalf("expired status: %+v %v", op, err)
	}
	if _, err := s.CompleteReconnect(ctx, owner, second.Operation.ID); !errors.Is(err, ErrReconnectConflict) {
		t.Fatalf("expired completion: %v", err)
	}
	if err := s.ConfirmReconnect(ctx, owner, second.Operation.NewTokenID); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired confirmation: %v", err)
	}
	if _, err := s.Authenticate(ctx, old.Secret); err != nil {
		t.Fatalf("old must survive: %v", err)
	}
	third, err := s.StartReconnect(ctx, owner, old.Token.ID)
	if err != nil || third.Secret == "" || third.Operation.ID == second.Operation.ID {
		t.Fatalf("start after expiry: %v", err)
	}
	// A separately revoked replacement can never revoke the old token on completion.
	if err := s.ConfirmReconnect(ctx, owner, third.Operation.NewTokenID); err != nil {
		t.Fatal(err)
	}
	if err := s.Revoke(ctx, owner, third.Operation.NewTokenID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteReconnect(ctx, owner, third.Operation.ID); !errors.Is(err, ErrReconnectConflict) {
		t.Fatalf("revoked replacement: %v", err)
	}
	if _, err := s.Authenticate(ctx, old.Secret); err != nil {
		t.Fatal(err)
	}
}

func TestReconnectConcurrentIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	s := New(db)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	owner := createUser(t, db)
	old, err := s.Issue(ctx, owner, "PC")
	if err != nil {
		t.Fatal(err)
	}
	const n = 6
	results := make(chan ReconnectResult, n)
	errs := make(chan error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() { <-start; r, e := s.StartReconnect(ctx, owner, old.Token.ID); results <- r; errs <- e })
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var operation Reconnection
	secrets := 0
	for r := range results {
		if operation.ID != "" && operation.ID != r.Operation.ID {
			t.Fatal("multiple operations")
		}
		operation = r.Operation
		if r.Secret != "" {
			secrets++
		}
	}
	if secrets != 1 {
		t.Fatalf("secret issuance count: %d", secrets)
	}
	if err := s.ConfirmReconnect(ctx, owner, operation.NewTokenID); err != nil {
		t.Fatal(err)
	}
	// Completion and restart serialize. Exactly one wins, regardless of scheduling.
	barrier := make(chan struct{})
	done := make(chan error, 2)
	wg.Go(func() { <-barrier; _, e := s.CompleteReconnect(ctx, owner, operation.ID); done <- e })
	wg.Go(func() { <-barrier; _, e := s.RestartReconnect(ctx, owner, operation.ID); done <- e })
	close(barrier)
	wg.Wait()
	close(done)
	ok, conflicts := 0, 0
	for e := range done {
		if e == nil {
			ok++
		} else if errors.Is(e, ErrReconnectConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if ok != 1 || conflicts != 1 {
		t.Fatalf("race outcomes: success=%d conflict=%d", ok, conflicts)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM connection_reconnections WHERE old_token_id=$1 AND state IN ('pending','confirmed')`, old.Token.ID).Scan(&count); err != nil || count > 1 {
		t.Fatalf("pending operations: %d %v", count, err)
	}
}

func TestReconnectRollbackAndTargetIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	s := New(db)
	ctx := context.Background()
	owner, other := createUser(t, db), createUser(t, db)
	old, err := s.Issue(ctx, owner, "same name")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := s.Issue(ctx, other, "same name")
	if err != nil {
		t.Fatal(err)
	}
	// A live reconnection is required for the rollback and down-migration checks.
	// Expired and revoked targets are covered by TestReconnectUnavailableTargetIntegration.
	first, err := s.StartReconnect(ctx, owner, old.Token.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []func(context.Context, string, string) error{
		func(c context.Context, u, id string) error { _, e := s.GetReconnect(c, u, id); return e },
		func(c context.Context, u, id string) error { _, e := s.CompleteReconnect(c, u, id); return e },
		func(c context.Context, u, id string) error { _, e := s.RestartReconnect(c, u, id); return e },
	} {
		for _, id := range []string{first.Operation.ID, uuid.NewString()} {
			if err := action(ctx, other, id); !errors.Is(err, ErrReconnectNotFound) {
				t.Fatalf("foreign/missing operations: %v", err)
			}
		}
	}
	if err := s.ConfirmReconnect(ctx, other, first.Operation.NewTokenID); err != nil {
		t.Fatal(err)
	}
	status, err := s.GetReconnect(ctx, owner, first.Operation.ID)
	if err != nil || status.State != "pending" {
		t.Fatalf("wrong owner confirmed operation: %+v %v", status, err)
	}
	// Enforce same-owner relations in the database as well as in the service.
	if _, err := db.Exec(`UPDATE connection_reconnections SET new_token_id=$2 WHERE id=$1`, first.Operation.ID, foreign.Token.ID); err == nil {
		t.Fatal("cross-owner FK missing")
	}
	// An insert failure during restart must roll back both the revocation and state update.
	if _, err := db.Exec(`CREATE FUNCTION reject_reconnect() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected failure'; END $$; CREATE TRIGGER reject_reconnect BEFORE INSERT ON connection_reconnections FOR EACH ROW EXECUTE FUNCTION reject_reconnect()`); err != nil {
		t.Fatal(err)
	}
	failed, err := s.RestartReconnect(ctx, owner, first.Operation.ID)
	if err == nil || failed.Secret != "" || failed.Operation.ID != "" {
		t.Fatalf("failed restart returned result: %v", err)
	}
	if _, err := s.Authenticate(ctx, first.Secret); err != nil {
		t.Fatalf("rollback lost original replacement: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM connection_tokens WHERE user_id=$1`, owner).Scan(&count); err != nil || count != 2 {
		t.Fatalf("orphan issuance: %d %v", count, err)
	}
	if _, err := db.Exec(`DROP TRIGGER reject_reconnect ON connection_reconnections`); err != nil {
		t.Fatal(err)
	}
	// Once a target disappears, no other same-name connection may replace it.
	if _, err := s.StartReconnect(ctx, owner, uuid.NewString()); !errors.Is(err, ErrTokenNotFound) {
		t.Fatal(err)
	}
	// Down migration cannot turn a provisional token into an ordinary 90-day token.
	applyMigration(t, db, "000006_connection_reconnections.down.sql")
	applyMigration(t, db, "000006_connection_reconnections.up.sql")
	if _, err := s.Authenticate(ctx, first.Secret); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("rollback reactivated provisional token: %v", err)
	}
}

func TestReconnectCompletionRollbackIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	s := New(db)
	ctx := context.Background()
	owner := createUser(t, db)
	old, err := s.Issue(ctx, owner, "PC")
	if err != nil {
		t.Fatal(err)
	}
	pending, err := s.StartReconnect(ctx, owner, old.Token.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmReconnect(ctx, owner, pending.Operation.NewTokenID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE FUNCTION reject_completion() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.state='completed' THEN RAISE EXCEPTION 'injected failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_completion BEFORE UPDATE ON connection_reconnections FOR EACH ROW EXECUTE FUNCTION reject_completion()`); err != nil {
		t.Fatal(err)
	}
	result, err := s.CompleteReconnect(ctx, owner, pending.Operation.ID)
	if err == nil || result.ID != "" {
		t.Fatalf("failed completion: %+v %v", result, err)
	}
	for _, secret := range []string{old.Secret, pending.Secret} {
		if _, err := s.Authenticate(ctx, secret); err != nil {
			t.Fatalf("rollback revoked token: %v", err)
		}
	}
	op, err := s.GetReconnect(ctx, owner, pending.Operation.ID)
	if err != nil || op.State != "confirmed" {
		t.Fatalf("rollback state: %+v %v", op, err)
	}
	if _, err := db.Exec(`DROP TRIGGER reject_completion ON connection_reconnections`); err != nil {
		t.Fatal(err)
	}
	// Repeated completion requests all return the same operation and timestamp.
	const n = 6
	start := make(chan struct{})
	type outcome struct {
		op  Reconnection
		err error
	}
	out := make(chan outcome, n)
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() { <-start; r, e := s.CompleteReconnect(ctx, owner, pending.Operation.ID); out <- outcome{r, e} })
	}
	close(start)
	wg.Wait()
	close(out)
	var completedAt time.Time
	for r := range out {
		if r.err != nil || r.op.State != "completed" {
			t.Fatalf("concurrent completion: %+v %v", r.op, r.err)
		}
		if !completedAt.IsZero() && !r.op.CompletedAt.Equal(completedAt) {
			t.Fatal("completion time changed")
		}
		completedAt = *r.op.CompletedAt
	}
	// Account deletion still cascades through token and reconnection metadata.
	if _, err := db.Exec(`DELETE FROM users WHERE id=$1`, owner); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM connection_reconnections`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("owner cascade: %d %v", count, err)
	}
}

func TestReconnectUnavailableTargetIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	s := New(db)
	base := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	clock := base
	s.now = func() time.Time { return clock }
	ctx := context.Background()
	owner := createUser(t, db)

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
	rejectStart := func(t *testing.T, oldID string) {
		t.Helper()
		tokens, ops := counts(oldID)
		_, err := s.StartReconnect(ctx, owner, oldID)
		if !errors.Is(err, ErrTokenUnavailable) {
			t.Fatalf("start: %v", err)
		}
		if gotTokens, gotOps := counts(oldID); gotTokens != tokens || gotOps != ops {
			t.Fatalf("created rows: tokens %d->%d ops %d->%d", tokens, gotTokens, ops, gotOps)
		}
	}

	t.Run("期限の1日前は拒否しトークンも再接続も増やさない", func(t *testing.T) {
		issued, err := s.Issue(ctx, owner, "期限切れ")
		if err != nil {
			t.Fatal(err)
		}
		expiredAt := base.Add(-24 * time.Hour)
		if _, err := db.Exec(`UPDATE connection_tokens SET created_at=$2, expires_at=$3 WHERE id=$1`, issued.Token.ID, expiredAt.Add(-Lifetime), expiredAt); err != nil {
			t.Fatal(err)
		}
		rejectStart(t, issued.Token.ID)
		if _, err := s.Authenticate(ctx, issued.Secret); !errors.Is(err, ErrInvalidToken) {
			t.Fatal("expired token revived")
		}
	})
	t.Run("期限ちょうどは拒否しトークンも再接続も増やさない", func(t *testing.T) {
		issued, err := s.Issue(ctx, owner, "期限ちょうど")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE connection_tokens SET created_at=$2, expires_at=$3 WHERE id=$1`, issued.Token.ID, base.Add(-Lifetime), base); err != nil {
			t.Fatal(err)
		}
		rejectStart(t, issued.Token.ID)
		if _, err := s.Authenticate(ctx, issued.Secret); !errors.Is(err, ErrInvalidToken) {
			t.Fatal("exact expiry revived")
		}
	})
	t.Run("失効済みは拒否しトークンも再接続も増やさない", func(t *testing.T) {
		issued, err := s.Issue(ctx, owner, "失効")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Revoke(ctx, owner, issued.Token.ID); err != nil {
			t.Fatal(err)
		}
		rejectStart(t, issued.Token.ID)
		var expires time.Time
		if err := db.QueryRow(`SELECT expires_at FROM connection_tokens WHERE id=$1 AND revoked_at IS NOT NULL`, issued.Token.ID).Scan(&expires); err != nil {
			t.Fatal(err)
		}
		if !expires.Equal(issued.Token.ExpiresAt) {
			t.Fatalf("expiry changed: %v", expires)
		}
		if _, err := s.Authenticate(ctx, issued.Secret); !errors.Is(err, ErrInvalidToken) {
			t.Fatal("revoked token revived")
		}
	})
	t.Run("期限の1マイクロ秒前は開始できる", func(t *testing.T) {
		issued, err := s.Issue(ctx, owner, "期限直前")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE connection_tokens SET expires_at=$2 WHERE id=$1`, issued.Token.ID, base.Add(time.Microsecond)); err != nil {
			t.Fatal(err)
		}
		tokens, ops := counts(issued.Token.ID)
		started, err := s.StartReconnect(ctx, owner, issued.Token.ID)
		if err != nil || started.Secret == "" || started.Operation.State != "pending" {
			t.Fatalf("start before expiry: %+v %v", started.Operation, err)
		}
		if gotTokens, gotOps := counts(issued.Token.ID); gotTokens != tokens+1 || gotOps != ops+1 {
			t.Fatalf("rows: tokens %d ops %d", gotTokens, gotOps)
		}
	})
	t.Run("進行中は旧トークンが期限切れでも同じ操作を返す", func(t *testing.T) {
		issued, err := s.Issue(ctx, owner, "進行中の期限切れ")
		if err != nil {
			t.Fatal(err)
		}
		started, err := s.StartReconnect(ctx, owner, issued.Token.ID)
		if err != nil || started.Secret == "" {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE connection_tokens SET created_at=$2, expires_at=$3 WHERE id=$1`, issued.Token.ID, base.Add(-Lifetime), base); err != nil {
			t.Fatal(err)
		}
		tokens, ops := counts(issued.Token.ID)
		again, err := s.StartReconnect(ctx, owner, issued.Token.ID)
		if err != nil || again.Secret != "" || again.Operation.ID != started.Operation.ID || again.Operation.State != "pending" {
			t.Fatalf("in-progress retry: %+v %v", again, err)
		}
		if gotTokens, gotOps := counts(issued.Token.ID); gotTokens != tokens || gotOps != ops {
			t.Fatalf("retry created rows: tokens %d->%d ops %d->%d", tokens, gotTokens, ops, gotOps)
		}
		if _, err := s.Authenticate(ctx, issued.Secret); !errors.Is(err, ErrInvalidToken) {
			t.Fatal("expired old revived")
		}
		if _, err := s.Authenticate(ctx, started.Secret); err != nil {
			t.Fatalf("replacement: %v", err)
		}
	})
	t.Run("進行中は旧トークンを失効しても同じ操作を返す", func(t *testing.T) {
		issued, err := s.Issue(ctx, owner, "進行中の失効")
		if err != nil {
			t.Fatal(err)
		}
		started, err := s.StartReconnect(ctx, owner, issued.Token.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Revoke(ctx, owner, issued.Token.ID); err != nil {
			t.Fatal(err)
		}
		tokens, ops := counts(issued.Token.ID)
		again, err := s.StartReconnect(ctx, owner, issued.Token.ID)
		if err != nil || again.Secret != "" || again.Operation.ID != started.Operation.ID {
			t.Fatalf("revoked in-progress retry: %+v %v", again, err)
		}
		if gotTokens, gotOps := counts(issued.Token.ID); gotTokens != tokens || gotOps != ops {
			t.Fatalf("retry created rows: tokens %d->%d ops %d->%d", tokens, gotTokens, ops, gotOps)
		}
	})
	t.Run("完了済みの遅延再送は失効後も完了を返す", func(t *testing.T) {
		issued, err := s.Issue(ctx, owner, "完了済み")
		if err != nil {
			t.Fatal(err)
		}
		started, err := s.StartReconnect(ctx, owner, issued.Token.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.ConfirmReconnect(ctx, owner, started.Operation.NewTokenID); err != nil {
			t.Fatal(err)
		}
		completed, err := s.CompleteReconnect(ctx, owner, started.Operation.ID)
		if err != nil || completed.State != "completed" {
			t.Fatalf("complete: %+v %v", completed, err)
		}
		tokens, ops := counts(issued.Token.ID)
		again, err := s.StartReconnect(ctx, owner, issued.Token.ID)
		if err != nil || again.Secret != "" || again.Operation.ID != completed.ID || again.Operation.State != "completed" {
			t.Fatalf("completed retry: %+v %v", again, err)
		}
		if gotTokens, gotOps := counts(issued.Token.ID); gotTokens != tokens || gotOps != ops {
			t.Fatalf("retry created rows: tokens %d->%d ops %d->%d", tokens, gotTokens, ops, gotOps)
		}
		if _, err := s.Authenticate(ctx, issued.Secret); !errors.Is(err, ErrInvalidToken) {
			t.Fatal("completed old token revived")
		}
	})
	t.Run("24時間を過ぎた未完了は期限切れの旧トークンから作り直さない", func(t *testing.T) {
		issued, err := s.Issue(ctx, owner, "24時間後")
		if err != nil {
			t.Fatal(err)
		}
		started, err := s.StartReconnect(ctx, owner, issued.Token.ID)
		if err != nil {
			t.Fatal(err)
		}
		prev := clock
		t.Cleanup(func() { clock = prev })
		clock = started.Operation.ExpiresAt
		if _, err := db.Exec(`UPDATE connection_tokens SET expires_at=$2 WHERE id=$1`, issued.Token.ID, clock); err != nil {
			t.Fatal(err)
		}
		rejectStart(t, issued.Token.ID)
		var state string
		if err := db.QueryRow(`SELECT state FROM connection_reconnections WHERE id=$1`, started.Operation.ID).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state != "pending" {
			t.Fatalf("retired on rejection: %s", state)
		}
		var active int
		if err := db.QueryRow(`SELECT count(*) FROM connection_tokens WHERE id=$1 AND revoked_at IS NULL`, started.Operation.NewTokenID).Scan(&active); err != nil || active != 1 {
			t.Fatalf("replacement revoked on rejection: %d %v", active, err)
		}
	})
}
