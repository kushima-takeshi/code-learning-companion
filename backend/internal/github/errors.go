package github

import "errors"

var (
	ErrInvalidURL        = errors.New("invalid_url")
	ErrPRNotFound        = errors.New("pr_not_found")
	ErrGitHubUnavailable = errors.New("github_unavailable")
)
