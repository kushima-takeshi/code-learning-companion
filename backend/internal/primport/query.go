package primport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *Service) List(
	ctx context.Context,
	userID string,
) ([]SnapshotSummary, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`
		SELECT
			id, html_url, owner, repo, pr_number,
			title, author_login, state, created_at
		FROM pr_snapshots
		WHERE user_id = $1
		ORDER BY created_at DESC
		`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list PR snapshots: %w", err)
	}
	defer rows.Close()

	snapshots := make([]SnapshotSummary, 0)

	for rows.Next() {
		var snapshot SnapshotSummary

		err := rows.Scan(
			&snapshot.ID,
			&snapshot.HTMLURL,
			&snapshot.Owner,
			&snapshot.Repo,
			&snapshot.PRNumber,
			&snapshot.Title,
			&snapshot.AuthorLogin,
			&snapshot.State,
			&snapshot.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan PR snapshot: %w", err)
		}

		snapshots = append(snapshots, snapshot)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read PR snapshots: %w", err)
	}

	return snapshots, nil
}

func (s *Service) Get(
	ctx context.Context,
	userID string,
	snapshotID string,
) (*SnapshotSummary, error) {
	var snapshot SnapshotSummary

	err := s.db.QueryRowContext(
		ctx,
		`
		SELECT
			id, html_url, owner, repo, pr_number,
			title, author_login, state, created_at
		FROM pr_snapshots
		WHERE id = $1 AND user_id = $2
		`,
		snapshotID,
		userID,
	).Scan(
		&snapshot.ID,
		&snapshot.HTMLURL,
		&snapshot.Owner,
		&snapshot.Repo,
		&snapshot.PRNumber,
		&snapshot.Title,
		&snapshot.AuthorLogin,
		&snapshot.State,
		&snapshot.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSnapshotNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get PR snapshot: %w", err)
	}

	return &snapshot, nil
}
