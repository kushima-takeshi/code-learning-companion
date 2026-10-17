package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Replace only the transport: exercise real request construction without network access.
type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

const testPRPath = "/repos/owner/repo/pulls/7"
const testDetail = `{"title":"Title","body":"Body","state":"open","user":{"login":"owner"},"head":{"sha":"head123"},"html_url":"https://github.com/owner/repo/pull/7"}`

type responseStep struct {
	path, accept, body, link string
	status                   int
	err                      error
	check                    func(*testing.T, *http.Request)
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

func scriptedClient(t *testing.T, token string, steps []responseStep) *Client {
	t.Helper()
	c := NewClient(token)
	calls := 0
	var bodies []*trackedBody
	c.httpClient.Transport = testTransport(func(r *http.Request) (*http.Response, error) {
		if calls >= len(steps) {
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
		step := steps[calls]
		calls++
		if r.Method != http.MethodGet || r.URL.Scheme != "https" || r.URL.Host != "api.github.com" || r.URL.Path != step.path {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
		}
		accept := step.accept
		if accept == "" {
			accept = "application/vnd.github+json"
		}
		for name, want := range map[string]string{"User-Agent": "code-learning-companion", "Accept": accept, "X-GitHub-Api-Version": "2022-11-28"} {
			if got := r.Header.Get(name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
		if token == "" {
			if _, ok := r.Header["Authorization"]; ok {
				t.Error("unexpected Authorization header")
			}
		} else if r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("incorrect Authorization header")
		}
		if step.check != nil {
			step.check(t, r)
		}
		if step.err != nil {
			return nil, step.err
		}
		body := &trackedBody{Reader: strings.NewReader(step.body)}
		bodies = append(bodies, body)
		status := step.status
		if status == 0 {
			status = http.StatusOK
		}
		header := make(http.Header)
		if step.link != "" {
			header.Set("Link", step.link)
		}
		return &http.Response{StatusCode: status, Header: header, Body: body, Request: r}, nil
	})
	t.Cleanup(func() {
		if calls != len(steps) {
			t.Errorf("requests = %d, want %d", calls, len(steps))
		}
		for i, b := range bodies {
			if !b.closed {
				t.Errorf("response body %d was not closed", i)
			}
		}
	})
	return c
}

func successfulSteps() []responseStep {
	return []responseStep{
		{path: testPRPath, body: testDetail},
		{path: testPRPath, accept: "application/vnd.github.diff", body: "diff text"},
		{path: testPRPath + "/files", body: `[{"filename":"main.go","status":"modified","patch":"patch text"}]`},
		{path: testPRPath + "/commits", body: `[{"sha":"head123","commit":{"message":"message","author":{"name":"Taro"}},"author":{"login":"taro"}},{"sha":"other","commit":{"message":"older","author":{"name":"Local"}},"author":null}]`},
		{path: testPRPath + "/comments", body: `[{"path":"main.go","line":42,"body":"review","user":{"login":"reviewer"},"commit_id":"head123"},{"path":"main.go","line":null,"body":"file review","user":{"login":"reviewer"},"commit_id":"head123"}]`},
		{path: "/repos/owner/repo/contents/main.go", accept: "application/vnd.github.raw", body: "package main\n", check: func(t *testing.T, r *http.Request) {
			if r.URL.Query().Get("ref") != "head123" {
				t.Error("content not pinned to head SHA")
			}
		}},
	}
}

func TestFetchPRSuccess(t *testing.T) {
	for _, token := range []string{"", "test-only-token"} {
		name := "without_token"
		if token != "" {
			name = "with_token"
		}
		t.Run(name, func(t *testing.T) {
			c := scriptedClient(t, token, successfulSteps())
			if c.httpClient.Timeout != 15*time.Second {
				t.Error("unexpected timeout")
			}
			got, err := c.FetchPR(context.Background(), "owner", "repo", 7)
			if err != nil || got == nil {
				t.Fatalf("FetchPR failed: %v", err)
			}
			if got.Title != "Title" || got.Body != "Body" || got.State != "open" || got.Author != "owner" || got.HeadSHA != "head123" || got.HTMLURL != "https://github.com/owner/repo/pull/7" || got.Diff != "diff text" {
				t.Errorf("incorrect PR metadata: %+v", got)
			}
			if len(got.Files) != 1 || got.Files[0].Content == nil || *got.Files[0].Content != "package main\n" || got.Files[0].Patch == nil || *got.Files[0].Patch != "patch text" {
				t.Fatalf("incorrect files: %+v", got.Files)
			}
			if len(got.Commits) != 2 || got.Commits[0].SHA != "head123" || got.Commits[0].Message != "message" || got.Commits[0].AuthorName != "Taro" || got.Commits[0].AuthorLogin == nil || *got.Commits[0].AuthorLogin != "taro" || got.Commits[1].AuthorLogin != nil {
				t.Errorf("incorrect commits: %+v", got.Commits)
			}
			if len(got.ReviewComments) != 2 || got.ReviewComments[0].Line == nil || *got.ReviewComments[0].Line != 42 || got.ReviewComments[0].Author != "reviewer" || got.ReviewComments[0].CommitSHA != "head123" || got.ReviewComments[1].Line != nil {
				t.Errorf("incorrect comments: %+v", got.ReviewComments)
			}
		})
	}
}

func TestFetchPRFailuresReturnNoSnapshot(t *testing.T) {
	for stage, name := range []string{"detail", "diff", "files", "commits", "comments", "content"} {
		for _, mode := range []string{"503", "429", "timeout", "invalid_json"} {
			if mode == "invalid_json" && (name == "diff" || name == "content") {
				continue
			}
			t.Run(name+"/"+mode, func(t *testing.T) {
				steps := successfulSteps()[:stage+1]
				switch mode {
				case "503":
					steps[stage].status = 503
				case "429":
					steps[stage].status = 429
				case "timeout":
					steps[stage].err = context.DeadlineExceeded
				case "invalid_json":
					steps[stage].body = "{"
				}
				c := scriptedClient(t, "", steps)
				got, err := c.FetchPR(context.Background(), "owner", "repo", 7)
				if got != nil || !errors.Is(err, ErrGitHubUnavailable) {
					t.Fatalf("snapshot = %v, error = %v", got, err)
				}
			})
		}
	}
	t.Run("detail_404_stops_immediately", func(t *testing.T) {
		c := scriptedClient(t, "", []responseStep{{path: testPRPath, status: 404}})
		got, err := c.FetchPR(context.Background(), "owner", "repo", 7)
		if got != nil || !errors.Is(err, ErrPRNotFound) {
			t.Fatalf("snapshot = %v, error = %v", got, err)
		}
	})
	t.Run("content_404_keeps_patch", func(t *testing.T) {
		steps := successfulSteps()
		steps[5].status = 404
		got, err := scriptedClient(t, "", steps).FetchPR(context.Background(), "owner", "repo", 7)
		if err != nil || got == nil {
			t.Fatalf("FetchPR failed: %v", err)
		}
		f := got.Files[0]
		if f.Content != nil || f.ContentSkipReason != "not_found" || f.Patch == nil || *f.Patch != "patch text" {
			t.Errorf("incorrect fallback: %+v", f)
		}
	})
}

func TestListsPagination(t *testing.T) {
	endpoints := []struct {
		name, body string
		fetch      func(*Client) (int, bool, error)
	}{
		{"files", `[{"filename":"main.go"}]`, func(c *Client) (int, bool, error) {
			v, e := c.fetchFiles(context.Background(), "owner", "repo", 7)
			return len(v), v == nil, e
		}},
		{"commits", `[{"sha":"abc"}]`, func(c *Client) (int, bool, error) {
			v, e := c.fetchCommits(context.Background(), "owner", "repo", 7)
			return len(v), v == nil, e
		}},
		{"comments", `[{"body":"review"}]`, func(c *Client) (int, bool, error) {
			v, e := c.fetchReviewComments(context.Background(), "owner", "repo", 7)
			return len(v), v == nil, e
		}},
	}
	for _, endpoint := range endpoints {
		for _, scenario := range []string{"empty", "two_pages", "ten_pages", "eleventh_page", "second_page_failure", "external_next"} {
			t.Run(endpoint.name+"/"+scenario, func(t *testing.T) {
				pages := 2
				wantErr := false
				switch scenario {
				case "empty", "external_next":
					pages = 1
				case "ten_pages", "eleventh_page":
					pages = 10
				}
				if scenario == "eleventh_page" || scenario == "second_page_failure" || scenario == "external_next" {
					wantErr = true
				}
				var steps []responseStep
				for i := 1; i <= pages; i++ {
					page := i
					step := responseStep{path: testPRPath + "/" + endpoint.name, body: endpoint.body, check: func(t *testing.T, r *http.Request) {
						if r.URL.Query().Get("per_page") != "100" {
							t.Error("missing per_page=100")
						}
						wantPage := ""
						if page > 1 {
							wantPage = fmt.Sprint(page)
						}
						if r.URL.Query().Get("page") != wantPage {
							t.Error("incorrect page URL")
						}
					}}
					if i < pages || scenario == "eleventh_page" {
						step.link = fmt.Sprintf(`<https://api.github.com%s/%s?per_page=100&page=%d>; rel="next", <https://api.github.com%s/%s?per_page=100&page=99>; rel="last"`, testPRPath, endpoint.name, i+1, testPRPath, endpoint.name)
					}
					if scenario == "empty" {
						step.body = "[]"
					}
					if scenario == "second_page_failure" && i == 2 {
						step.status = 503
					}
					if scenario == "external_next" {
						step.link = `<https://example.com/page2>; rel="next"`
					}
					steps = append(steps, step)
				}
				count, isNil, err := endpoint.fetch(scriptedClient(t, "", steps))
				if wantErr {
					if !errors.Is(err, ErrGitHubUnavailable) || !isNil {
						t.Fatalf("partial result: count=%d nil=%v err=%v", count, isNil, err)
					}
					return
				}
				want := pages
				if scenario == "empty" {
					want = 0
				}
				if err != nil || count != want {
					t.Fatalf("count=%d err=%v, want %d", count, err, want)
				}
			})
		}
	}
}
