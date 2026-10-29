package learningrecord

import (
	"context"
	"database/sql"
	"errors"
	"reflect"

	"github.com/jackc/pgx/v5/pgconn"
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
		saved, err = s.insertRecord(ctx, userID, input)
		if err == nil {
			return saved, nil
		}

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) ||
			pgErr.Code != "23505" ||
			pgErr.ConstraintName != "learning_records_user_request_unique" {
			return Record{}, err
		}

		// 別の要求が先に保存した記録を取得し、既存の内容比較へ進む。
		saved, err = s.getByRequestID(ctx, userID, input.RequestID)
	}
	if err != nil {
		return Record{}, err
	}

	if saved.Theme != input.Theme || !reflect.DeepEqual(saved.Questions, input.Questions) {
		return Record{}, ErrRequestConflict
	}

	return saved, nil
}
