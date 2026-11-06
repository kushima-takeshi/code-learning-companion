package connectiontoken

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const ReconnectLifetime = 24 * time.Hour

var (
	ErrReconnectNotFound = errors.New("reconnection not found")
	ErrReconnectConflict = errors.New("reconnection cannot proceed in its current state")
	ErrTokenUnavailable  = errors.New("connection token is expired or revoked")
)

// Reconnection contains metadata only. It never holds a token secret or hash.
type Reconnection struct {
	ID          string
	UserID      string
	OldTokenID  string
	NewTokenID  string
	State       string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	ConfirmedAt *time.Time
	CompletedAt *time.Time
}

type ReconnectResult struct {
	Operation Reconnection
	Secret    string
}

const reconnectColumns = `id, user_id, old_token_id, new_token_id, state, created_at, expires_at, confirmed_at, completed_at`

func scanReconnect(row tokenScanner) (Reconnection, error) {
	var r Reconnection
	err := row.Scan(&r.ID, &r.UserID, &r.OldTokenID, &r.NewTokenID, &r.State, &r.CreatedAt, &r.ExpiresAt, &r.ConfirmedAt, &r.CompletedAt)
	return r, err
}

// lockTarget serializes all operations for the same old token, including first start.
// Provisional replacement tokens cannot be targets themselves; this also prevents lock cycles.
func lockTarget(ctx context.Context, tx *sql.Tx, userID, oldID string) (Token, error) {
	token, err := scanToken(tx.QueryRowContext(ctx, `SELECT id,user_id,name,created_at,expires_at,revoked_at FROM connection_tokens WHERE user_id=$1 AND id=$2 FOR UPDATE`, userID, oldID))
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, ErrTokenNotFound
	}
	if err != nil {
		return Token{}, err
	}
	var provisional bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM connection_reconnections WHERE new_token_id=$1 AND state <> 'completed')`, oldID).Scan(&provisional)
	if err != nil {
		return Token{}, err
	}
	if provisional {
		return Token{}, ErrReconnectConflict
	}
	return token, nil
}

func lockReconnect(ctx context.Context, tx *sql.Tx, userID, id string) (Reconnection, Token, error) {
	var oldID string
	err := tx.QueryRowContext(ctx, `SELECT old_token_id FROM connection_reconnections WHERE user_id=$1 AND id=$2`, userID, id).Scan(&oldID)
	if errors.Is(err, sql.ErrNoRows) {
		return Reconnection{}, Token{}, ErrReconnectNotFound
	}
	if err != nil {
		return Reconnection{}, Token{}, err
	}
	old, err := lockTarget(ctx, tx, userID, oldID)
	if err != nil {
		return Reconnection{}, Token{}, err
	}
	r, err := scanReconnect(tx.QueryRowContext(ctx, `SELECT `+reconnectColumns+` FROM connection_reconnections WHERE user_id=$1 AND id=$2 FOR UPDATE`, userID, id))
	return r, old, err
}

func newReconnect(ctx context.Context, tx *sql.Tx, old Token, now time.Time) (ReconnectResult, error) {
	issued, err := issueToken(ctx, tx, old.UserID, old.Name, now)
	if err != nil {
		return ReconnectResult{}, err
	}
	r, err := scanReconnect(tx.QueryRowContext(ctx, `INSERT INTO connection_reconnections(user_id,old_token_id,new_token_id,state,created_at,expires_at) VALUES($1,$2,$3,'pending',$4,$5) RETURNING `+reconnectColumns, old.UserID, old.ID, issued.Token.ID, now, now.Add(ReconnectLifetime)))
	if err != nil {
		return ReconnectResult{}, err
	}
	return ReconnectResult{Operation: r, Secret: issued.Secret}, nil
}

// StartReconnect returns the current operation on a retry, without re-disclosing its secret.
func (s *Service) StartReconnect(ctx context.Context, userID, oldID string) (ReconnectResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReconnectResult{}, err
	}
	defer tx.Rollback()
	old, err := lockTarget(ctx, tx, userID, oldID)
	if err != nil {
		return ReconnectResult{}, err
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	current, err := scanReconnect(tx.QueryRowContext(ctx, `SELECT `+reconnectColumns+` FROM connection_reconnections WHERE old_token_id=$1 AND state IN ('pending','confirmed','completed')`, oldID))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ReconnectResult{}, err
	}
	if err == nil && (current.State == "completed" || now.Before(current.ExpiresAt)) {
		if err := tx.Commit(); err != nil {
			return ReconnectResult{}, err
		}
		return ReconnectResult{Operation: current}, nil
	}
	// A new operation would mint a token for an unusable connection. Retries above stay available.
	if tokenUnavailable(old, now) {
		return ReconnectResult{}, ErrTokenUnavailable
	}
	if err == nil {
		if err := retireReconnect(ctx, tx, current, "expired", current.ExpiresAt); err != nil {
			return ReconnectResult{}, err
		}
	}
	result, err := newReconnect(ctx, tx, old, now)
	if err != nil {
		return ReconnectResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReconnectResult{}, fmt.Errorf("start reconnection: %w", err)
	}
	return result, nil
}

func tokenUnavailable(token Token, now time.Time) bool {
	return token.RevokedAt != nil || !now.Before(token.ExpiresAt)
}

// GetReconnect is read-only. Expiry and manual revocation are reflected without a cleanup job.
func (s *Service) GetReconnect(ctx context.Context, userID, id string) (Reconnection, error) {
	// One snapshot keeps the operation state and replacement availability consistent.
	r, err := scanReconnect(s.db.QueryRowContext(ctx, `
 SELECT r.id,r.user_id,r.old_token_id,r.new_token_id,
 CASE WHEN r.state IN ('pending','confirmed') AND r.expires_at <= $3 THEN 'expired'
      WHEN r.state IN ('pending','confirmed') AND (t.revoked_at IS NOT NULL OR t.expires_at <= $3) THEN 'unavailable'
      ELSE r.state END,
 r.created_at,r.expires_at,r.confirmed_at,r.completed_at
 FROM connection_reconnections r JOIN connection_tokens t ON t.id=r.new_token_id
 WHERE r.user_id=$1 AND r.id=$2`, userID, id, s.now().UTC()))
	if errors.Is(err, sql.ErrNoRows) {
		return Reconnection{}, ErrReconnectNotFound
	}
	if err != nil {
		return Reconnection{}, err
	}
	return r, nil
}

func activeReplacement(ctx context.Context, tx *sql.Tx, r Reconnection, clock func() time.Time) (time.Time, error) {
	token, err := scanToken(tx.QueryRowContext(ctx, `SELECT id,user_id,name,created_at,expires_at,revoked_at FROM connection_tokens WHERE id=$1 AND user_id=$2 FOR UPDATE`, r.NewTokenID, r.UserID))
	if err != nil {
		return time.Time{}, err
	}
	now := clock().UTC().Truncate(time.Microsecond)
	if token.RevokedAt != nil || !now.Before(token.ExpiresAt) {
		return time.Time{}, ErrReconnectConflict
	}
	return now, nil
}

// ConfirmReconnect is called only after a successful list query with this token.
// It grants no Web management permission and never revokes the old token.
func (s *Service) ConfirmReconnect(ctx context.Context, userID, tokenID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM connection_reconnections WHERE user_id=$1 AND new_token_id=$2`, userID, tokenID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	r, _, err := lockReconnect(ctx, tx, userID, id)
	if err != nil {
		return err
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	if r.State != "completed" && ((r.State != "pending" && r.State != "confirmed") || !now.Before(r.ExpiresAt)) {
		return ErrInvalidToken
	}
	now, err = activeReplacement(ctx, tx, r, s.now)
	if err != nil {
		if errors.Is(err, ErrReconnectConflict) {
			return ErrInvalidToken
		}
		return err
	}
	if r.State != "completed" && !now.Before(r.ExpiresAt) {
		return ErrInvalidToken
	}
	if r.State == "pending" {
		if _, err := tx.ExecContext(ctx, `UPDATE connection_reconnections SET state='confirmed',confirmed_at=$2 WHERE id=$1`, id, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Service) CompleteReconnect(ctx context.Context, userID, id string) (Reconnection, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Reconnection{}, err
	}
	defer tx.Rollback()
	r, _, err := lockReconnect(ctx, tx, userID, id)
	if err != nil {
		return Reconnection{}, err
	}
	if r.State == "completed" {
		if err := tx.Commit(); err != nil {
			return Reconnection{}, err
		}
		return r, nil
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	if r.State != "confirmed" || !now.Before(r.ExpiresAt) {
		return Reconnection{}, ErrReconnectConflict
	}
	now, err = activeReplacement(ctx, tx, r, s.now)
	if err != nil {
		return Reconnection{}, err
	}
	if !now.Before(r.ExpiresAt) {
		return Reconnection{}, ErrReconnectConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE connection_tokens SET revoked_at=COALESCE(revoked_at,$2) WHERE id=$1 AND user_id=$3`, r.OldTokenID, now, userID); err != nil {
		return Reconnection{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE connection_reconnections SET state='completed',completed_at=$2 WHERE id=$1`, id, now); err != nil {
		return Reconnection{}, err
	}
	r.State = "completed"
	r.CompletedAt = &now
	if err := tx.Commit(); err != nil {
		return Reconnection{}, err
	}
	return r, nil
}

func retireReconnect(ctx context.Context, tx *sql.Tx, r Reconnection, state string, at time.Time) error {
	if _, err := tx.ExecContext(ctx, `UPDATE connection_tokens SET revoked_at=LEAST(COALESCE(revoked_at,$2),$2) WHERE id=$1`, r.NewTokenID, at); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE connection_reconnections SET state=$2 WHERE id=$1`, r.ID, state)
	return err
}

// RestartReconnect invalidates the abandoned new token and creates a fresh attempt atomically.
// The old operation ID becomes stale; retrying it cannot issue yet another token.
func (s *Service) RestartReconnect(ctx context.Context, userID, id string) (ReconnectResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReconnectResult{}, err
	}
	defer tx.Rollback()
	r, old, err := lockReconnect(ctx, tx, userID, id)
	if err != nil {
		return ReconnectResult{}, err
	}
	if r.State != "pending" && r.State != "confirmed" {
		return ReconnectResult{}, ErrReconnectConflict
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	if !now.Before(r.ExpiresAt) {
		return ReconnectResult{}, ErrReconnectConflict
	}
	if err := retireReconnect(ctx, tx, r, "superseded", now); err != nil {
		return ReconnectResult{}, err
	}
	result, err := newReconnect(ctx, tx, old, now)
	if err != nil {
		return ReconnectResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReconnectResult{}, err
	}
	return result, nil
}
