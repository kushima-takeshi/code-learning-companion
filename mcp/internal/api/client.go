// Package api calls the existing learning-record HTTP API with a bearer token.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

const maxResponseBytes = 1 << 20

// Client sends JSON to one API base URL. It does not open a listening port.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New rejects an empty token. baseURL must already include the scheme and host.
func New(baseURL, token string) (*Client, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("connection token is required")
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("API base URL is required")
	}
	return &Client{baseURL: baseURL, token: token, http: http.DefaultClient}, nil
}

// DoJSON sends payload as JSON when it is non-nil. The returned body is the
// raw response, including error JSON. Errors never include the token.
func (c *Client) DoJSON(ctx context.Context, method, path string, payload any) (int, []byte, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, c.redact(err)
		}
		body = bytes.NewReader(raw)
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return 0, nil, c.redact(err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, c.redact(err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return resp.StatusCode, nil, c.redact(err)
	}
	if len(respBody) > maxResponseBytes {
		return resp.StatusCode, nil, errors.New("API response exceeds 1 MiB")
	}
	return resp.StatusCode, respBody, nil
}

func (c *Client) redact(err error) error {
	if err == nil || c.token == "" {
		return err
	}
	text := err.Error()
	if !strings.Contains(text, c.token) {
		return err
	}
	return errors.New(strings.ReplaceAll(text, c.token, "[redacted]"))
}
