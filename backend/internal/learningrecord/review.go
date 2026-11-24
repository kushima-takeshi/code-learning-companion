package learningrecord

import (
	"context"
	"fmt"
	"time"
)

const firstReviewDelay = 72 * time.Hour

type dueReview struct {
	ID    string    `json:"id"`
	Stage int       `json:"stage"`
	DueAt time.Time `json:"due_at"`
}

func firstReviewDueAt(createdAt time.Time) time.Time {
	return createdAt.Add(firstReviewDelay)
}

// firstReviewCutoff is the latest creation time whose first review is due.
func firstReviewCutoff(now time.Time) time.Time {
	return now.Add(-firstReviewDelay)
}

func (s *Service) listDueReviews(ctx context.Context, userID string) ([]dueReview, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, created_at
		FROM learning_records
		WHERE user_id = $1 AND created_at <= $2
		ORDER BY created_at, id
	`, userID, firstReviewCutoff(s.now()))
	if err != nil {
		return nil, fmt.Errorf("list due reviews: %w", err)
	}
	defer rows.Close()

	records := make([]dueReview, 0)
	for rows.Next() {
		var id string
		var createdAt time.Time
		if err := rows.Scan(&id, &createdAt); err != nil {
			return nil, fmt.Errorf("scan due review: %w", err)
		}
		records = append(records, dueReview{ID: id, Stage: 1, DueAt: firstReviewDueAt(createdAt)})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read due reviews: %w", err)
	}
	return records, nil
}
