package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	httpClient *http.Client
	token      string
}

func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		token: token,
	}
}

func (c *Client) newRequest(
	ctx context.Context,
	requestURL string,
	accept string,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		requestURL,
		nil,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "code-learning-companion")
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return req, nil
}

func (c *Client) fetchPRDetail(
	ctx context.Context,
	owner string,
	repo string,
	number int,
) (*prResponse, error) {
	requestURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/pulls/%d",
		owner,
		repo,
		number,
	)

	req, err := c.newRequest(
		ctx,
		requestURL,
		"application/vnd.github+json",
	)
	if err != nil {
		return nil, ErrGitHubUnavailable
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ErrGitHubUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrPRNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrGitHubUnavailable
	}

	var detail prResponse
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, ErrGitHubUnavailable
	}

	return &detail, nil
}

func (c *Client) fetchDiff(
	ctx context.Context,
	owner string,
	repo string,
	number int,
) (string, error) {
	requestURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/pulls/%d",
		owner,
		repo,
		number,
	)

	req, err := c.newRequest(
		ctx,
		requestURL,
		"application/vnd.github.diff",
	)
	if err != nil {
		return "", ErrGitHubUnavailable
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", ErrGitHubUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", ErrGitHubUnavailable
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", ErrGitHubUnavailable
	}

	return string(body), nil
}

func (c *Client) fetchFilesPage(
	ctx context.Context,
	pageURL string,
) ([]fileResponse, string, error) {
	if err := validatePageURL(pageURL); err != nil {
		return nil, "", err
	}

	req, err := c.newRequest(
		ctx,
		pageURL,
		"application/vnd.github+json",
	)
	if err != nil {
		return nil, "", ErrGitHubUnavailable
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", ErrGitHubUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", ErrGitHubUnavailable
	}

	var files []fileResponse
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, "", ErrGitHubUnavailable
	}

	nextURL := nextPageURL(resp.Header.Get("Link"))

	return files, nextURL, nil
}

func (c *Client) fetchFiles(
	ctx context.Context,
	owner string,
	repo string,
	number int,
) ([]fileResponse, error) {
	pageURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/pulls/%d/files?per_page=100",
		owner,
		repo,
		number,
	)

	var allFiles []fileResponse

	for page := 0; page < 10; page++ {
		files, nextURL, err := c.fetchFilesPage(ctx, pageURL)
		if err != nil {
			return nil, err
		}

		allFiles = append(allFiles, files...)

		if nextURL == "" {
			return allFiles, nil
		}

		pageURL = nextURL
	}

	return nil, ErrGitHubUnavailable
}

func (c *Client) fetchCommitsPage(
	ctx context.Context,
	pageURL string,
) ([]commitResponse, string, error) {
	if err := validatePageURL(pageURL); err != nil {
		return nil, "", err
	}

	req, err := c.newRequest(
		ctx,
		pageURL,
		"application/vnd.github+json",
	)
	if err != nil {
		return nil, "", ErrGitHubUnavailable
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", ErrGitHubUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", ErrGitHubUnavailable
	}

	var commits []commitResponse
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		return nil, "", ErrGitHubUnavailable
	}

	nextURL := nextPageURL(resp.Header.Get("Link"))

	return commits, nextURL, nil
}

func (c *Client) fetchCommits(
	ctx context.Context,
	owner string,
	repo string,
	number int,
) ([]commitResponse, error) {
	pageURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/pulls/%d/commits?per_page=100",
		owner,
		repo,
		number,
	)

	var allCommits []commitResponse

	for page := 0; page < 10; page++ {
		commits, nextURL, err := c.fetchCommitsPage(ctx, pageURL)
		if err != nil {
			return nil, err
		}

		allCommits = append(allCommits, commits...)

		if nextURL == "" {
			return allCommits, nil
		}

		pageURL = nextURL
	}

	return nil, ErrGitHubUnavailable
}

func (c *Client) fetchReviewCommentsPage(
	ctx context.Context,
	pageURL string,
) ([]reviewCommentResponse, string, error) {
	if err := validatePageURL(pageURL); err != nil {
		return nil, "", err
	}

	req, err := c.newRequest(
		ctx,
		pageURL,
		"application/vnd.github+json",
	)
	if err != nil {
		return nil, "", ErrGitHubUnavailable
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", ErrGitHubUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", ErrGitHubUnavailable
	}

	var comments []reviewCommentResponse
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return nil, "", ErrGitHubUnavailable
	}

	nextURL := nextPageURL(resp.Header.Get("Link"))

	return comments, nextURL, nil
}

func (c *Client) fetchReviewComments(
	ctx context.Context,
	owner string,
	repo string,
	number int,
) ([]reviewCommentResponse, error) {
	pageURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/pulls/%d/comments?per_page=100",
		owner,
		repo,
		number,
	)

	var allComments []reviewCommentResponse

	for page := 0; page < 10; page++ {
		comments, nextURL, err := c.fetchReviewCommentsPage(ctx, pageURL)
		if err != nil {
			return nil, err
		}

		allComments = append(allComments, comments...)

		if nextURL == "" {
			return allComments, nil
		}

		pageURL = nextURL
	}

	return nil, ErrGitHubUnavailable
}

func (c *Client) FetchPR(
	ctx context.Context,
	owner string,
	repo string,
	number int,
) (*PRSnapshot, error) {
	detail, err := c.fetchPRDetail(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}

	diff, err := c.fetchDiff(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}

	files, err := c.fetchFiles(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}

	commits, err := c.fetchCommits(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}

	comments, err := c.fetchReviewComments(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}

	changedFiles, err := c.buildChangedFiles(
		ctx,
		owner,
		repo,
		detail.Head.SHA,
		files,
	)
	if err != nil {
		return nil, err
	}

	snapshot := &PRSnapshot{
		Title:          detail.Title,
		Body:           detail.Body,
		State:          detail.State,
		Author:         detail.User.Login,
		HeadSHA:        detail.Head.SHA,
		HTMLURL:        detail.HTMLURL,
		Diff:           diff,
		Files:          changedFiles,
		Commits:        convertCommits(commits),
		ReviewComments: convertReviewComments(comments),
	}

	return snapshot, nil
}
