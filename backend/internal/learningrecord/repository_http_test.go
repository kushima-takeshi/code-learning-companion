package learningrecord

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestLearningRepositoryHTTPIntegration(t *testing.T) {
	const repository = "my-org/Repo_1.test"
	db, handler, tokens := newListHTTP(t)
	owner := listUser(t, db, 1)
	token, err := tokens.Issue(context.Background(), owner, "owner")
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body string) (int, string) {
		res := recordRequest(handler, method, path, token.Secret, "application/json", body, nil)
		return res.Code, res.Body.String()
	}

	for _, tc := range []struct {
		name, field, want string
	}{
		{name: "with repository", field: `,"repository":"` + repository + `"`, want: `"` + repository + `"`},
		{name: "omitted"},
		{name: "empty", field: `,"repository":""`},
		{name: "null", field: `,"repository":null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := validSaveInputForTest(1)
			input.RequestID = uuid.NewString()
			body := strings.TrimSuffix(inputJSON(t, input), "}") + tc.field + "}"
			status, saved := request(http.MethodPost, "/learning-records", body)
			if status != http.StatusOK {
				t.Fatalf("save: %d %s", status, saved)
			}
			var record struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal([]byte(saved), &record); err != nil || record.ID == "" {
				t.Fatalf("saved record: %s (%v)", saved, err)
			}
			if tc.want != "" {
				status, retry := request(http.MethodPost, "/learning-records", body)
				if status != http.StatusOK || retry != saved {
					t.Fatalf("same repository on retry: %d %s", status, retry)
				}
				changed := strings.Replace(body, repository, "owner/different", 1)
				status, conflict := request(http.MethodPost, "/learning-records", changed)
				if status != http.StatusConflict || strings.TrimSpace(conflict) != `{"error":"request_conflict"}` {
					t.Fatalf("different repository on retry: %d %s", status, conflict)
				}
			}
			status, detail := request(http.MethodGet, "/learning-records/"+record.ID, "")
			if status != http.StatusOK || detail != saved {
				t.Fatalf("detail: %d %s; save: %s", status, detail, saved)
			}
			status, list := request(http.MethodGet, "/learning-records", "")
			var page struct {
				Records []map[string]json.RawMessage `json:"records"`
			}
			if err := json.Unmarshal([]byte(list), &page); err != nil || status != http.StatusOK {
				t.Fatalf("list: %d %s (%v)", status, list, err)
			}
			for _, summary := range page.Records {
				if string(summary["id"]) != `"`+record.ID+`"` {
					continue
				}
				var full map[string]json.RawMessage
				if err := json.Unmarshal([]byte(detail), &full); err != nil {
					t.Fatal(err)
				}
				if string(full["repository"]) != tc.want || string(summary["repository"]) != tc.want {
					t.Fatalf("repository in detail: %s; summary: %s; want: %q", full["repository"], summary["repository"], tc.want)
				}
				return
			}
			t.Fatalf("missing saved record %s in list: %s", record.ID, list)
		})
	}

	t.Run("invalid format", func(t *testing.T) {
		body := strings.TrimSuffix(inputJSON(t, validSaveInputForTest(1)), "}")
		for _, value := range []string{"repo", "/repo", "owner/", "owner/repo/extra", "https://owner/repo", "owner/repo name", " ", "owner/.."} {
			status, response := request(http.MethodPost, "/learning-records", body+`,"repository":"`+value+`"}`)
			want := `{"error":"validation_failed","details":[{"path":"repository","code":"invalid_value"}]}`
			if status != http.StatusBadRequest || strings.TrimSpace(response) != want {
				t.Errorf("repository %q: %d %s", value, status, response)
			}
		}
	})
}
