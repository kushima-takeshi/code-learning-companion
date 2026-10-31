package connectiontoken

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const Lifetime = 90 * 24 * time.Hour

var ErrInvalidName = errors.New("connection token name is required")

type Token struct {
	ID        string
	UserID    string
	Name      string
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// IssuedToken is returned only by Issue. Metadata never contains the secret or hash.
type IssuedToken struct {
	Token  Token
	Secret string
}

type Service struct {
	db  *sql.DB
	now func() time.Time
}

func New(db *sql.DB) *Service { return &Service{db: db, now: time.Now} }

func (s *Service) Issue(ctx context.Context, userID, name string) (IssuedToken, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return IssuedToken{}, ErrInvalidName
	}

	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return IssuedToken{}, fmt.Errorf("generate connection token: %w", err)
	}
	secret := "clc_" + hex.EncodeToString(randomBytes)
	hash := sha256.Sum256([]byte(secret))
	createdAt := s.now().UTC().Truncate(time.Microsecond)
	token, err := scanToken(s.db.QueryRowContext(ctx, `
		INSERT INTO connection_tokens (user_id, name, token_hash, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, name, created_at, expires_at, revoked_at
	`, userID, name, hash[:], createdAt, createdAt.Add(Lifetime)))
	if err != nil {
		return IssuedToken{}, fmt.Errorf("issue connection token: %w", err)
	}
	return IssuedToken{Token: token, Secret: secret}, nil
}

type tokenScanner interface {
	Scan(...any) error
}

func scanToken(row tokenScanner) (Token, error) {
	var token Token
	var revokedAt sql.NullTime
	if err := row.Scan(&token.ID, &token.UserID, &token.Name, &token.CreatedAt, &token.ExpiresAt, &revokedAt); err != nil {
		return Token{}, err
	}
	if revokedAt.Valid {
		token.RevokedAt = &revokedAt.Time
	}
	return token, nil
}
