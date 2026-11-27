package learningrecord

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const firstReviewDelay = 72 * time.Hour

var errReviewConflict = errors.New("learning record review conflict")

type dueReview struct {
	ID         string    `json:"id"`
	Theme      string    `json:"theme"`
	Repository string    `json:"repository,omitempty"`
	Stage      int       `json:"stage"`
	DueAt      time.Time `json:"due_at"`
}

// nextReviewStage finds the first missing stage for the outer record.
const nextReviewStage = `
	SELECT candidate.stage
	FROM generate_series(1, 3) AS candidate(stage)
	WHERE NOT EXISTS (
		SELECT 1 FROM learning_record_reviews AS review
		WHERE review.learning_record_id = record.id AND review.stage = candidate.stage
	)
	ORDER BY candidate.stage
	LIMIT 1
`

const reviewDueAt = `record.created_at + CASE next.stage
	WHEN 1 THEN interval '72 hours'
	WHEN 2 THEN interval '336 hours'
	WHEN 3 THEN interval '720 hours'
END`

func (s *Service) listDueReviews(ctx context.Context, userID string) ([]dueReview, error) {
	now := s.now()
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, theme, repository, stage, due_at
		FROM (
			SELECT record.id, record.theme, record.repository, next.stage, `+reviewDueAt+` AS due_at
			FROM learning_records AS record
			CROSS JOIN LATERAL (`+nextReviewStage+`) AS next
			WHERE record.user_id = $1 AND record.created_at <= $2
		) AS reviews
		WHERE due_at <= $3
		ORDER BY due_at, id
	`, userID, now.Add(-firstReviewDelay), now)
	if err != nil {
		return nil, fmt.Errorf("list due reviews: %w", err)
	}
	defer rows.Close()

	records := make([]dueReview, 0)
	for rows.Next() {
		var record dueReview
		if err := rows.Scan(&record.ID, &record.Theme, &record.Repository, &record.Stage, &record.DueAt); err != nil {
			return nil, fmt.Errorf("scan due review: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read due reviews: %w", err)
	}
	return records, nil
}

func (s *Service) saveReview(ctx context.Context, userID, recordID, result string, requestedStage int) (int, error) {
	// Insert only when the requested stage is the first missing stage.
	// A concurrent insert for that stage leaves this request with no saved row.
	const query = `
		WITH owned_record AS (
			SELECT id, user_id FROM learning_records
			WHERE id = $1 AND user_id = $2
			FOR KEY SHARE
		), saved AS (
			INSERT INTO learning_record_reviews (learning_record_id, user_id, stage, result, saved_at)
			SELECT record.id, record.user_id, next.stage, $3, $4
			FROM owned_record AS record
			CROSS JOIN LATERAL (` + nextReviewStage + `) AS next
			WHERE next.stage = $5
			ON CONFLICT (learning_record_id, stage) DO NOTHING
			RETURNING stage
		)
		SELECT COALESCE((SELECT stage FROM saved), 0) FROM owned_record
	`
	var stage int
	if err := s.db.QueryRowContext(ctx, query, recordID, userID, result, s.now(), requestedStage).Scan(&stage); err != nil {
		return 0, fmt.Errorf("save review: %w", err)
	}
	if stage == 0 {
		return 0, errReviewConflict
	}
	return stage, nil
}
