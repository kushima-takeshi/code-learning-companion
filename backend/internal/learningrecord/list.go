package learningrecord

import (
	"context"
	"fmt"
)

const (
	DefaultListLimit = 20
	MaxListLimit     = 100
	MaxListOffset    = 1<<31 - 1
)

// List returns a bounded page of the owner's summaries, newest first.
func (s *Service) List(ctx context.Context, userID string, limit, offset int) (ListPage, error) {
	if limit < 1 || limit > MaxListLimit || offset < 0 || offset > MaxListOffset {
		return ListPage{}, ErrInvalidPagination
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, theme, created_at
		FROM learning_records
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, userID, limit+1, offset)
	if err != nil {
		return ListPage{}, fmt.Errorf("list learning records: %w", err)
	}
	defer rows.Close()

	page := ListPage{Records: make([]RecordSummary, 0, limit)}
	for rows.Next() {
		var summary RecordSummary
		if err := rows.Scan(&summary.ID, &summary.Theme, &summary.CreatedAt); err != nil {
			return ListPage{}, fmt.Errorf("scan learning record summary: %w", err)
		}
		if len(page.Records) == limit {
			page.HasMore = true
		} else {
			page.Records = append(page.Records, summary)
		}
	}
	if err := rows.Err(); err != nil {
		return ListPage{}, fmt.Errorf("read learning record summaries: %w", err)
	}
	return page, nil
}
