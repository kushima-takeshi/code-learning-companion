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
	// 1. Parse the PR URL. If invalid, return before reading the file.
	owner, repo, number, err := github.ParsePRURL(input.URL)
	if err != nil {
		return nil, err
	}

	// 2. Read the optional conversation file and check its extension, size, and NUL bytes.
	// Continue fetching the PR even if the file is absent or produces a warning.
	markdown, warning := readUpload(input.File)

	// 3. Fetch the PR data from GitHub into memory.
	// Return on failure. Nothing has been saved to MinIO or the database yet.
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
