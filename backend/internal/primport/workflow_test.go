package primport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/github"
)

func TestImportSuccess(t *testing.T) {
	for _, withConversation := range []bool{false, true} {
		t.Run(fmt.Sprintf("conversation_%t", withConversation), func(t *testing.T) {
			s, fetcher, store, db := testService(t)
			input := testInput()
			if withConversation {
				input.File = conversationFile("# conversation")
			}
			fetcher.check = func(owner, repo string, number int) {
				if owner != "owner" || repo != "repo" || number != 7 {
					t.Fatalf("wrong PR: %s/%s/%d", owner, repo, number)
				}
			}
			store.onPut = func() {
				if fetcher.calls != 1 || db.begins != 0 {
					t.Fatal("Put before fetch or after DB transaction began")
				}
			}
			wantPuts, wantRows := 7, 1
			if withConversation {
				wantPuts++
				wantRows++
			}
			db.beforeBegin = func() {
				if len(store.objects) != wantPuts {
					t.Fatal("transaction began before all objects were saved")
				}
			}
			result, err := s.Import(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := uuid.Parse(result.SnapshotID); err != nil {
				t.Fatal(err)
			}
			if result.Warning != "" || len(store.objects) != wantPuts || len(store.deletes) != 0 {
				t.Fatalf("unexpected result/storage: %+v objects=%d deletes=%v", result, len(store.objects), store.deletes)
			}
			prefix := input.UserID + "/pr-snapshots/" + result.SnapshotID + "/"
			checks := map[string]savedObject{
				prefix + "body.md":        {"PR body", "text/markdown; charset=utf-8"},
				prefix + "diff.patch":     {"PR diff", "text/plain; charset=utf-8"},
				prefix + "contents/0.txt": {"package main\n", "text/plain; charset=utf-8"},
				prefix + "contents/1.txt": {"", "text/plain; charset=utf-8"},
			}
			for key, want := range checks {
				if got, ok := store.objects[key]; !ok || got != want {
					t.Errorf("object %q=%+v, want %+v", key, got, want)
				}
			}
			var files []storedFile
			if err := json.Unmarshal([]byte(store.objects[prefix+"files.json"].body), &files); err != nil {
				t.Fatal(err)
			}
			if len(files) != 3 {
				t.Fatalf("files=%+v", files)
			}
			for i := 0; i < 2; i++ {
				if !files[i].ContentStored || files[i].ContentKey != prefix+fmt.Sprintf("contents/%d.txt", i) || files[i].Path != fetcher.snapshot.Files[i].Filename {
					t.Errorf("file %d=%+v", i, files[i])
				}
			}
			if files[0].Patch == nil || *files[0].Patch != *fetcher.snapshot.Files[0].Patch {
				t.Fatal("patch missing or changed")
			}
			if files[2].ContentStored || files[2].ContentKey != "" || files[2].Status != "removed" {
				t.Fatal("removed file incorrectly saved")
			}
			var rawFiles []map[string]any
			json.Unmarshal([]byte(store.objects[prefix+"files.json"].body), &rawFiles)
			if _, ok := rawFiles[2]["content_key"]; ok {
				t.Fatal("absent content key must be omitted")
			}
			var commits []github.Commit
			var comments []github.ReviewComment
			if err := json.Unmarshal([]byte(store.objects[prefix+"commits.json"].body), &commits); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(store.objects[prefix+"review-comments.json"].body), &comments); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(commits, fetcher.snapshot.Commits) || !reflect.DeepEqual(comments, fetcher.snapshot.ReviewComments) {
				t.Fatal("commits/comments changed")
			}
			for _, name := range []string{"files.json", "commits.json", "review-comments.json"} {
				if store.objects[prefix+name].contentType != "application/json" {
					t.Errorf("wrong JSON type: %s", name)
				}
			}
			if db.begins != 1 || db.commits != 1 || db.rollbacks != 0 || len(db.committed) != wantRows {
				t.Fatalf("DB boundaries: %+v", db)
			}
			pr := db.committed[0]
			if !strings.Contains(pr.query, "INSERT INTO pr_snapshots") || strings.Contains(strings.ToUpper(pr.query), "ON CONFLICT") {
				t.Fatal("unexpected snapshot SQL")
			}
			wantArgs := []any{result.SnapshotID, input.UserID, fetcher.snapshot.HTMLURL, "owner", "repo", int64(7), "Test PR", "author", "open", "abc123", prefix + "body.md", prefix + "diff.patch", prefix + "files.json", prefix + "commits.json", prefix + "review-comments.json", store.objects[prefix+"files.json"].body}
			if len(pr.args) != len(wantArgs) {
				t.Fatalf("SQL arguments=%d", len(pr.args))
			}
			for i, want := range wantArgs {
				if !reflect.DeepEqual(pr.args[i].Value, want) {
					t.Errorf("SQL argument %d=%v, want %v", i+1, pr.args[i].Value, want)
				}
			}
			if withConversation {
				row := db.committed[1]
				if !strings.Contains(row.query, "INSERT INTO conversation_uploads") || len(row.args) != 5 {
					t.Fatal("unexpected conversation SQL")
				}
				id := row.args[0].Value.(string)
				if _, err := uuid.Parse(id); err != nil {
					t.Fatal(err)
				}
				if id == result.SnapshotID || row.args[1].Value != input.UserID || row.args[2].Value != result.SnapshotID || row.args[3].Value != "conversation.md" {
					t.Fatal("wrong conversation identity/owner/link")
				}
				key := input.UserID + "/conversation-uploads/" + id + "/conversation.md"
				if row.args[4].Value != key || store.objects[key].body != "# conversation" {
					t.Fatal("wrong conversation object")
				}
			}
		})
	}
}

func TestImportInvalidConversationContinues(t *testing.T) {
	tests := []struct {
		name, filename, body, warning string
		readError                     bool
	}{
		{"extension", "notes.txt", "hello", "invalid_file_extension", false},
		{"size", "notes.md", strings.Repeat("x", 1024*1024+1), "file_too_large", false},
		{"nul", "notes.md", "a\x00b", "file_contains_nul", false},
		{"read", "notes.md", "", "file_read_failed", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, fetcher, store, db := testService(t)
			input := testInput()
			input.File = &UploadFile{Filename: tt.filename, Reader: strings.NewReader(tt.body)}
			if tt.readError {
				input.File.Reader = errorReader{}
			}
			result, err := s.Import(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if result.Warning != tt.warning || fetcher.calls != 1 || len(db.committed) != 1 || len(store.objects) != 7 {
				t.Fatalf("warning/continuation failed: result=%+v", result)
			}
			for key := range store.objects {
				if strings.Contains(key, "conversation-uploads") {
					t.Fatal("invalid conversation saved")
				}
			}
		})
	}
}

func TestImportEmptyConversationIsSaved(t *testing.T) {
	s, _, store, db := testService(t)
	input := testInput()
	input.File = conversationFile("")
	if _, err := s.Import(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if len(db.committed) != 2 {
		t.Fatal("empty valid conversation was skipped")
	}
	key := db.committed[1].args[4].Value.(string)
	if obj, ok := store.objects[key]; !ok || obj.body != "" {
		t.Fatal("empty conversation not saved correctly")
	}
}

func TestImportGitHubFailureDoesNotSave(t *testing.T) {
	s, fetcher, store, db := testService(t)
	fetcher.err = github.ErrGitHubUnavailable
	result, err := s.Import(context.Background(), testInput())
	if result != nil || !errors.Is(err, github.ErrGitHubUnavailable) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(store.puts) != 0 || len(store.deletes) != 0 || db.begins != 0 {
		t.Fatal("external writes after fetch failure")
	}
}

func TestImportEveryPutFailureCleansPreviousObjects(t *testing.T) {
	// Body, diff, two file contents, three JSON objects, conversation.
	for failAt := 1; failAt <= 8; failAt++ {
		t.Run(fmt.Sprintf("put_%d", failAt), func(t *testing.T) {
			s, _, store, db := testService(t)
			store.failPut = failAt
			input := testInput()
			input.File = conversationFile("hello")
			result, err := s.Import(context.Background(), input)
			if result != nil || !errors.Is(err, errInjected) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if len(store.objects) != 0 || len(store.puts) != failAt || len(store.deletes) != failAt-1 || db.begins != 0 {
				t.Fatalf("cleanup failed: objects=%d puts=%d deletes=%d begins=%d", len(store.objects), len(store.puts), len(store.deletes), db.begins)
			}
			for i, key := range store.deletes {
				if key != store.puts[i] {
					t.Fatal("deleted wrong object")
				}
			}
		})
	}
}

func TestImportDatabaseFailures(t *testing.T) {
	for _, phase := range []string{"begin", "snapshot_insert", "conversation_insert", "commit"} {
		t.Run(phase, func(t *testing.T) {
			s, _, store, db := testService(t)
			switch phase {
			case "begin":
				db.failBegin = true
			case "snapshot_insert":
				db.failExec = 1
			case "conversation_insert":
				db.failExec = 2
			case "commit":
				db.failCommit = true
			}
			input := testInput()
			input.File = conversationFile("hello")
			result, err := s.Import(context.Background(), input)
			if result != nil || !errors.Is(err, errInjected) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if len(store.objects) != 0 || len(store.deletes) != 8 || len(db.committed) != 0 {
				t.Fatal("DB failure left saved objects or committed rows")
			}
			if (phase == "snapshot_insert" || phase == "conversation_insert") && db.rollbacks != 1 {
				t.Fatal("insert failure did not roll back")
			}
			if phase != "commit" && db.commits != 0 {
				t.Fatal("commit attempted after an earlier error")
			}
		})
	}
}

func TestImportRepeatedPRCreatesDistinctSnapshots(t *testing.T) {
	s, _, store, db := testService(t)
	first, err := s.Import(context.Background(), testInput())
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Import(context.Background(), testInput())
	if err != nil {
		t.Fatal(err)
	}
	if first.SnapshotID == second.SnapshotID || len(db.committed) != 2 || len(store.objects) != 14 {
		t.Fatal("repeated import overwrote previous snapshot")
	}
	for _, call := range db.committed {
		if strings.Contains(strings.ToUpper(call.query), "ON CONFLICT") {
			t.Fatal("upsert is not allowed")
		}
	}
}

func TestCleanupSurvivesCancellationAndContinuesAfterDeleteFailure(t *testing.T) {
	s, _, store, _ := testService(t)
	for _, key := range []string{"a", "b", "c"} {
		store.objects[key] = savedObject{}
	}
	store.failDelete = "b"
	original := errors.New("original DB failure")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.cleanupAfterFailure(ctx, []string{"a", "b", "c"}, original)
	if !errors.Is(err, original) || !errors.Is(err, errInjected) {
		t.Fatalf("lost error: %v", err)
	}
	if !reflect.DeepEqual(store.deletes, []string{"a", "b", "c"}) || len(store.objects) != 1 {
		t.Fatal("cleanup stopped early or used canceled context")
	}
	if _, ok := store.objects["b"]; !ok {
		t.Fatal("wrong remaining object")
	}
}

func TestImportFilesJSONEmptyArray(t *testing.T) {
	s, fetcher, store, _ := testService(t)
	fetcher.snapshot.Files = nil
	result, err := s.Import(context.Background(), testInput())
	if err != nil {
		t.Fatal(err)
	}
	key := testInput().UserID + "/pr-snapshots/" + result.SnapshotID + "/files.json"
	if store.objects[key].body != "[]" {
		t.Fatalf("empty files JSON=%q", store.objects[key].body)
	}
}
