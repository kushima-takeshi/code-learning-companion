package learningrecord

import (
	"context"
	"encoding/json"
	"fmt"
)

func (s *Service) Get(
	ctx context.Context,
	userID string,
	recordID string,
) (Record, error) {
	var record Record
	var questionsJSON []byte

	err := s.db.QueryRowContext(
		ctx,
		`
		SELECT id, user_id, request_id, theme, questions, created_at
		FROM learning_records
		WHERE id = $1 AND user_id = $2
		`,
		recordID,
		userID,
	).Scan(
		&record.ID,
		&record.UserID,
		&record.RequestID,
		&record.Theme,
		&questionsJSON,
		&record.CreatedAt,
	)
	if err != nil {
		return Record{}, fmt.Errorf("get learning record: %w", err)
	}

	if err := json.Unmarshal(questionsJSON, &record.Questions); err != nil {
		return Record{}, fmt.Errorf("decode stored questions: %w", err)
	}

	return record, nil
}
