package learningrecord

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
)

type Service struct {
	db *sql.DB
}

func New(db *sql.DB) *Service {
	return &Service{
		db: db,
	}
}

// Save validates a request, inserts a new record, or returns an identical prior result.
func (s *Service) Save(ctx context.Context, userID string, input SaveInput) (Record, error) {
	if err := input.Validate(); err != nil {
		return Record{}, err
	}

	saved, err := s.getByRequestID(ctx, userID, input.RequestID)
	if errors.Is(err, sql.ErrNoRows) {
		return s.insertRecord(ctx, userID, input)
	}
	if err != nil {
		return Record{}, err
	}

	if saved.Theme != input.Theme || !reflect.DeepEqual(saved.Questions, input.Questions) {
		return Record{}, ErrRequestConflict
	}

	return saved, nil
}
