package auth

import (
	"context"
	"database/sql"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

type User struct {
	ID           string
	GitHubUserID int64
	GitHubLogin  string
	DisplayName  sql.NullString
	AvatarURL    sql.NullString
}

func (s *Store) UpsertUser(
	ctx context.Context,
	githubUserID int64,
	login string,
	displayName, avatarURL sql.NullString,
) (string, error) {
	const query = `
		INSERT INTO users (
			github_user_id, github_login, display_name, avatar_url
		)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (github_user_id) DO UPDATE
		SET github_login = EXCLUDED.github_login,
		    display_name = EXCLUDED.display_name,
		    avatar_url = EXCLUDED.avatar_url,
		    updated_at = now()
		RETURNING id
	`

	var id string
	err := s.db.QueryRowContext(
		ctx,
		query,
		githubUserID,
		login,
		displayName,
		avatarURL,
	).Scan(&id)

	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) CreateSession(
	ctx context.Context,
	userID string,
	tokenHash []byte,
	expiresAt time.Time,
) error {
	const query = `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		userID,
		tokenHash,
		expiresAt,
	)
	return err
}

func (s *Store) FindUserByTokenHash(
	ctx context.Context,
	tokenHash []byte,
) (User, error) {
	const query = `
		SELECT
			u.id,
			u.github_user_id,
			u.github_login,
			u.display_name,
			u.avatar_url
		FROM sessions AS s
		JOIN users AS u ON u.id = s.user_id
		WHERE s.token_hash = $1
		  AND s.expires_at > now()
	`

	var user User
	err := s.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&user.ID,
		&user.GitHubUserID,
		&user.GitHubLogin,
		&user.DisplayName,
		&user.AvatarURL,
	)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) DeleteSession(
	ctx context.Context,
	tokenHash []byte,
) error {
	const query = `
		DELETE FROM sessions
		WHERE token_hash = $1
	`

	_, err := s.db.ExecContext(ctx, query, tokenHash)
	return err
}
