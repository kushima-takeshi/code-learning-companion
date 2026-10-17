package github

import "errors"

var (
	ErrPRNotFound        = errors.New("pr_not_found")
	ErrGitHubUnavailable = errors.New("github_unavailable")
)
