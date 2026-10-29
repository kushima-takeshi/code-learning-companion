package learningrecord

import (
	"context"
	"encoding/json"
	"fmt"
)

func (s *Service) insertRecord(
	ctx context.Context,
	userID string,
	input SaveInput,
) (Record, error) {
	questionsJSON, err := json.Marshal(input.Questions)
	if err != nil {
		return Record{}, fmt.Errorf("encode questions: %w", err)
	}

	record := Record{
		UserID:    userID,
		RequestID: input.RequestID,
		Theme:     input.Theme,
		Questions: input.Questions,
	}

	err = s.db.QueryRowContext(
		ctx,
		`
		INSERT INTO learning_records (
			user_id, request_id, theme, questions
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
		`,
		userID,
		input.RequestID,
		input.Theme,
		string(questionsJSON),
	).Scan(&record.ID, &record.CreatedAt)
	if err != nil {
		return Record{}, fmt.Errorf("insert learning record: %w", err)
	}

	return record, nil
}
