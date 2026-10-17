package github

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func isDependencyPath(filename string) bool {
	parts := strings.Split(filename, "/")

	for _, directory := range parts[:len(parts)-1] {
		if directory == "node_modules" || directory == "vendor" {
			return true
		}
	}

	return false
}

const maxFileContentBytes = 200 * 1024

func readFileContent(reader io.Reader) (*string, string, error) {
	body, err := io.ReadAll(
		io.LimitReader(reader, maxFileContentBytes+1),
	)
	if err != nil {
		return nil, "", ErrGitHubUnavailable
	}

	if len(body) > maxFileContentBytes {
		return nil, "too_large", nil
	}

	if bytes.IndexByte(body, 0) >= 0 {
		return nil, "binary", nil
	}

	content := string(body)
	return &content, "", nil
}

func (c *Client) fetchFileContent(
	ctx context.Context,
	owner string,
	repo string,
	filename string,
	headSHA string,
) (*string, string, error) {
	pathParts := strings.Split(filename, "/")
	for i, part := range pathParts {
		pathParts[i] = url.PathEscape(part)
	}
	escapedPath := strings.Join(pathParts, "/")

	query := url.Values{}
	query.Set("ref", headSHA)

	requestURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/contents/%s?%s",
		url.PathEscape(owner),
		url.PathEscape(repo),
		escapedPath,
		query.Encode(),
	)

	req, err := c.newRequest(
		ctx,
		requestURL,
		"application/vnd.github.raw",
	)
	if err != nil {
		return nil, "", ErrGitHubUnavailable
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", ErrGitHubUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "not_found", nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", ErrGitHubUnavailable
	}

	return readFileContent(resp.Body)
}

func (c *Client) buildChangedFile(
	ctx context.Context,
	owner string,
	repo string,
	headSHA string,
	file fileResponse,
) (ChangedFile, error) {
	result := ChangedFile{
		Filename: file.Filename,
		Status:   file.Status,
		Patch:    file.Patch,
	}

	if file.Status == "removed" {
		result.ContentSkipReason = "removed"
		return result, nil
	}

	if isDependencyPath(file.Filename) {
		result.ContentSkipReason = "dependency"
		return result, nil
	}

	content, reason, err := c.fetchFileContent(
		ctx,
		owner,
		repo,
		file.Filename,
		headSHA,
	)
	if err != nil {
		return ChangedFile{}, err
	}

	result.Content = content
	result.ContentSkipReason = reason

	return result, nil
}

func (c *Client) buildChangedFiles(
	ctx context.Context,
	owner string,
	repo string,
	headSHA string,
	files []fileResponse,
) ([]ChangedFile, error) {
	results := make([]ChangedFile, 0, len(files))

	for _, file := range files {
		result, err := c.buildChangedFile(
			ctx,
			owner,
			repo,
			headSHA,
			file,
		)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
}
