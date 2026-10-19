package primport

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/github"
)

var errInjected = errors.New("injected failure")

type fakeFetcher struct {
	snapshot *github.PRSnapshot
	err      error
	calls    int
	check    func(string, string, int)
}

func (f *fakeFetcher) FetchPR(_ context.Context, owner, repo string, number int) (*github.PRSnapshot, error) {
	f.calls++
	if f.check != nil {
		f.check(owner, repo, number)
	}
	return f.snapshot, f.err
}

type savedObject struct{ body, contentType string }
type memoryStore struct {
	objects       map[string]savedObject
	puts, deletes []string
	failPut       int
	failDelete    string
	onPut         func()
}

func (s *memoryStore) Put(ctx context.Context, key string, reader io.Reader, contentType string) error {
	if s.onPut != nil {
		s.onPut()
	}
	s.puts = append(s.puts, key)
	if len(s.puts) == s.failPut {
		return errInjected
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.objects[key] = savedObject{string(body), contentType}
	return nil
}
func (s *memoryStore) Delete(ctx context.Context, key string) error {
	s.deletes = append(s.deletes, key)
	if err := ctx.Err(); err != nil {
		return err
	}
	if key == s.failDelete {
		return errInjected
	}
	delete(s.objects, key)
	return nil
}

// This driver records SQL calls and transaction boundaries; it does not emulate PostgreSQL.
type sqlCall struct {
	query string
	args  []driver.NamedValue
}
type recordingDB struct {
	calls, committed           []sqlCall
	begins, commits, rollbacks int
	failBegin, failCommit      bool
	failExec                   int
	beforeBegin                func()
	query                      func(context.Context, string, []driver.NamedValue) (driver.Rows, error)
}
type recordingConnector struct{ state *recordingDB }
type recordingDriver struct{}
type recordingConn struct {
	state   *recordingDB
	pending []sqlCall
}

func (d recordingConnector) Connect(context.Context) (driver.Conn, error) {
	return &recordingConn{state: d.state}, nil
}
func (d recordingConnector) Driver() driver.Driver       { return recordingDriver{} }
func (recordingDriver) Open(string) (driver.Conn, error) { return nil, errors.New("use connector") }
func (c *recordingConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected Prepare")
}
func (c *recordingConn) Close() error { return nil }
func (c *recordingConn) Begin() (driver.Tx, error) {
	c.state.begins++
	if c.state.beforeBegin != nil {
		c.state.beforeBegin()
	}
	if c.state.failBegin {
		return nil, errInjected
	}
	c.pending = nil
	return c, nil
}
func (c *recordingConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	call := sqlCall{query, append([]driver.NamedValue(nil), args...)}
	c.state.calls = append(c.state.calls, call)
	if len(c.state.calls) == c.state.failExec {
		return nil, errInjected
	}
	c.pending = append(c.pending, call)
	return driver.RowsAffected(1), nil
}
func (c *recordingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.state.query == nil {
		return nil, errors.New("unexpected QueryContext")
	}
	return c.state.query(ctx, query, args)
}
func (c *recordingConn) Commit() error {
	c.state.commits++
	if c.state.failCommit {
		return errInjected
	}
	c.state.committed = append(c.state.committed, c.pending...)
	c.pending = nil
	return nil
}
func (c *recordingConn) Rollback() error { c.state.rollbacks++; c.pending = nil; return nil }

func testService(t *testing.T) (*Service, *fakeFetcher, *memoryStore, *recordingDB) {
	t.Helper()
	content, empty, patch := "package main\n", "", "@@ -1 +1 @@\n-old\n+new"
	fetcher := &fakeFetcher{snapshot: &github.PRSnapshot{
		Title: "Test PR", Body: "PR body", Diff: "PR diff", State: "open", Author: "author", HeadSHA: "abc123",
		HTMLURL: "https://github.com/owner/repo/pull/7",
		Files: []github.ChangedFile{
			{Filename: "src/main.go", Status: "modified", Patch: &patch, Content: &content},
			{Filename: "empty.txt", Status: "added", Content: &empty},
			{Filename: "removed.go", Status: "removed"},
		},
		Commits:        []github.Commit{{SHA: "abc123", Message: "change"}},
		ReviewComments: []github.ReviewComment{{Path: "src/main.go", Body: "review"}},
	}}
	store := &memoryStore{objects: make(map[string]savedObject)}
	state := &recordingDB{}
	db := sql.OpenDB(recordingConnector{state})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return &Service{db: db, github: fetcher, store: store}, fetcher, store, state
}
func testInput() ImportInput {
	return ImportInput{UserID: "11111111-1111-4111-8111-111111111111", URL: "https://github.com/owner/repo/pull/7"}
}
func conversationFile(body string) *UploadFile {
	return &UploadFile{Filename: "conversation.md", Reader: strings.NewReader(body)}
}
