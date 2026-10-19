package primport

import (
	"context"
	"database/sql"
	"io"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/github"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/storage"
)

// Keep the required operations small so tests can simulate external failures.
type prFetcher interface {
	FetchPR(context.Context, string, string, int) (*github.PRSnapshot, error)
}

type objectStore interface {
	Put(context.Context, string, io.Reader, string) error
	Delete(context.Context, string) error
}

type Service struct {
	db     *sql.DB
	github prFetcher
	store  objectStore
}

func New(
	db *sql.DB,
	githubClient *github.Client,
	store *storage.Storage,
) *Service {
	return &Service{
		db:     db,
		github: githubClient,
		store:  store,
	}
}
