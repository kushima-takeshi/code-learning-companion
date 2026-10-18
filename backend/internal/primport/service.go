package primport

import (
	"database/sql"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/github"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/storage"
)

type Service struct {
	db     *sql.DB
	github *github.Client
	store  *storage.Storage
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
