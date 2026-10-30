package connectiontoken

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrTokenNotFound = errors.New("connection token not found")

// List returns only the owner's metadata, including expired and revoked connections.
func (s *Service) List(ctx context.Context, userID string) ([]Token, error) {
	rows, err := s.db.QueryContext(ctx, `
  SELECT id, user_id, name, created_at, expires_at, revoked_at
  FROM connection_tokens WHERE user_id=$1
  ORDER BY created_at DESC, id DESC
 `, userID)
	if err != nil {
		return nil, fmt.Errorf("list connection tokens: %w", err)
	}
	defer rows.Close()
	tokens := make([]Token, 0)
	for rows.Next() {
		token, err := scanToken(rows)
		if err != nil {
			return nil, fmt.Errorf("read connection token: %w", err)
		}
		tokens = append(tokens, token)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list connection tokens: %w", err)
	}
	return tokens, nil
}

// Revoke is owner-scoped and keeps the timestamp of the first revocation.
func (s *Service) Revoke(ctx context.Context, userID, tokenID string) error {
	result, err := s.db.ExecContext(ctx, `
  UPDATE connection_tokens SET revoked_at=COALESCE(revoked_at,$3)
  WHERE id=$1 AND user_id=$2
 `, tokenID, userID, s.now().UTC().Truncate(time.Microsecond))
	if err != nil {
		return fmt.Errorf("revoke connection token: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke connection token result: %w", err)
	}
	if count == 0 {
		return ErrTokenNotFound
	}
	return nil
}
