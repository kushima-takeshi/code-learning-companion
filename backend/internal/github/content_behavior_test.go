package github

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type countingReader struct{ bytesRead int }

func (r *countingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	r.bytesRead += len(p)
	return len(p), nil
}

type brokenReader struct{}

func (brokenReader) Read(p []byte) (int, error) { return copy(p, "partial"), io.ErrUnexpectedEOF }

func TestReadFileContentReadBound(t *testing.T) {
	reader := &countingReader{}
	content, reason, err := readFileContent(reader)
	if content != nil || reason != "too_large" || err != nil {
		t.Fatalf("content=%v reason=%q err=%v", content, reason, err)
	}
	if reader.bytesRead != maxFileContentBytes+1 {
		t.Errorf("read %d bytes, want %d", reader.bytesRead, maxFileContentBytes+1)
	}
}
func TestReadFileContentEmptyAndReadError(t *testing.T) {
	t.Run("empty_is_present", func(t *testing.T) {
		content, reason, err := readFileContent(strings.NewReader(""))
		if err != nil || reason != "" || content == nil || *content != "" {
			t.Fatalf("content=%v reason=%q err=%v", content, reason, err)
		}
	})
	t.Run("read_error_discards_partial_content", func(t *testing.T) {
		content, reason, err := readFileContent(brokenReader{})
		if content != nil || reason != "" || !errors.Is(err, ErrGitHubUnavailable) {
			t.Fatalf("content=%v reason=%q err=%v", content, reason, err)
		}
	})
}

func TestBuildChangedFileSkipsWithoutRequests(t *testing.T) {
	for _, tc := range []struct{ path, status, reason string }{
		{"old.go", "removed", "removed"},
		{"node_modules/pkg/main.js", "added", "dependency"},
		{"frontend/node_modules/pkg/main.js", "modified", "dependency"},
		{"backend/vendor/pkg/main.go", "added", "dependency"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			patch := "patch"
			c := scriptedClient(t, "", nil)
			got, err := c.buildChangedFile(context.Background(), "owner", "repo", "head123", fileResponse{Filename: tc.path, Status: tc.status, Patch: &patch})
			if err != nil || got.Content != nil || got.ContentSkipReason != tc.reason || got.Patch == nil || *got.Patch != patch || got.Filename != tc.path || got.Status != tc.status {
				t.Fatalf("file=%+v err=%v", got, err)
			}
		})
	}
}
func TestDependencyPathDoesNotMatchFileNames(t *testing.T) {
	for _, path := range []string{"main.go", "src/vendor_utils.go", "src/vendor", "node_modules", "my_vendor/main.go"} {
		if isDependencyPath(path) {
			t.Errorf("incorrectly excluded %q", path)
		}
	}
}

func TestFileContentEscapingAndRef(t *testing.T) {
	filename := "src/日本語 #?%.go"
	c := scriptedClient(t, "", []responseStep{{
		path: "/repos/owner/repo/contents/" + filename, accept: "application/vnd.github.raw", body: "", check: func(t *testing.T, r *http.Request) {
			if r.URL.Fragment != "" || len(r.URL.Query()) != 1 || r.URL.Query().Get("ref") != "head123" {
				t.Errorf("incorrect URL: %s", r.URL)
			}
			if strings.Contains(r.URL.EscapedPath(), " ") {
				t.Error("space not escaped")
			}
		},
	}})
	content, reason, err := c.fetchFileContent(context.Background(), "owner", "repo", filename, "head123")
	if err != nil || reason != "" || content == nil || *content != "" {
		t.Fatalf("content=%v reason=%q err=%v", content, reason, err)
	}
}
func TestBuildChangedFilesDiscardsEarlierResults(t *testing.T) {
	c := scriptedClient(t, "", []responseStep{
		{path: "/repos/owner/repo/contents/a.go", accept: "application/vnd.github.raw", body: "first"},
		{path: "/repos/owner/repo/contents/b.go", accept: "application/vnd.github.raw", status: 503},
	})
	got, err := c.buildChangedFiles(context.Background(), "owner", "repo", "head123", []fileResponse{{Filename: "a.go"}, {Filename: "b.go"}, {Filename: "c.go"}})
	if got != nil || !errors.Is(err, ErrGitHubUnavailable) {
		t.Fatalf("files=%v err=%v", got, err)
	}
}
