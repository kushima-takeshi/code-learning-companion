package primport

import (
	"context"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/github"
)

type preparedImport struct {
	Owner    string
	Repo     string
	Number   int
	Snapshot *github.PRSnapshot
	Markdown []byte
	Warning  string
}

func (s *Service) prepare(
	ctx context.Context,
	input ImportInput,
) (*preparedImport, error) {
	owner, repo, number, err := github.ParsePRURL(input.URL)
	if err != nil {
		return nil, err
	}

	markdown, warning := readUpload(input.File)

	snapshot, err := s.github.FetchPR(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}

	return &preparedImport{
		Owner:    owner,
		Repo:     repo,
		Number:   number,
		Snapshot: snapshot,
		Markdown: markdown,
		Warning:  warning,
	}, nil
}
