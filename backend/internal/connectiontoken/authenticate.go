package connectiontoken

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidToken = errors.New("invalid connection token")

// Authenticate identifies an active token; HTTP route permissions are enforced separately.
func (s *Service) Authenticate(ctx context.Context, secret string) (Token, error) {
	if len(secret) != 68 || !strings.HasPrefix(secret, "clc_") {
		return Token{}, ErrInvalidToken
	}
	if _, err := hex.DecodeString(secret[4:]); err != nil {
		return Token{}, ErrInvalidToken
	}
	hash := sha256.Sum256([]byte(secret))
	token, err := scanToken(s.db.QueryRowContext(ctx, `
  SELECT id, user_id, name, created_at, expires_at, revoked_at
  FROM connection_tokens
  WHERE token_hash=$1 AND expires_at > $2 AND revoked_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM connection_reconnections r WHERE r.new_token_id=connection_tokens.id
    AND r.state <> 'completed'
    AND (r.state NOT IN ('pending', 'confirmed') OR r.expires_at <= $2)
  )
 `, hash[:], s.now().UTC()))
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, ErrInvalidToken
	}
	if err != nil {
		return Token{}, fmt.Errorf("authenticate connection token: %w", err)
	}
	return token, nil
}
