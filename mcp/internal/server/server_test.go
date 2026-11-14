package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kushima-takeshi/code-learning-companion/mcp/internal/api"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestListToolReturnsRecords(t *testing.T) {
	const token = "clc_secret_value"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/learning-records" || r.URL.RawQuery != "" {
			t.Errorf("request %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"records":[{"id":"rec-1","theme":"loops","created_at":"2026-11-13T00:00:00Z"}],"has_more":false}`))
	}))
	defer upstream.Close()

	text, isError := callList(t, upstream.URL, token)
	if isError || !strings.Contains(text, "loops") || strings.Contains(text, token) {
		t.Fatalf("isError=%v text=%s", isError, text)
	}
}

func TestFailureTextUsesStatusAndCodeOnly(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		want   string
	}{
		{400, `{"error":"invalid_pagination"}`, "400 invalid_pagination"},
		{500, `{"error":"internal_error"}`, "500 internal_error"},
	} {
		got := failureText("list learning records", tc.status, []byte(tc.body))
		if !strings.Contains(got, tc.want) || strings.Contains(got, "{") {
			t.Fatalf("status %d text %s", tc.status, got)
		}
	}
}

func TestListToolHidesTokenOnUnauthorized(t *testing.T) {
	const token = "clc_secret_value"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = ioWrite(w, `{"error":"unauthorized","detail":"`+token+`"}`)
	}))
	defer upstream.Close()

	text, isError := callList(t, upstream.URL, token)
	if !isError || !strings.Contains(text, "401 unauthorized") || strings.Contains(text, token) {
		t.Fatalf("isError=%v text=%s", isError, text)
	}
}

func TestSaveToolPostsRecord(t *testing.T) {
	const token = "clc_secret_value"
	hint := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/learning-records" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type %q", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"rec-1","request_id":"6ba7b810-9dad-11d1-80b4-00c04fd430c8","theme":"loops","questions":[],"created_at":"2026-11-13T00:00:00Z"}`))
	}))
	defer upstream.Close()

	text, isError := callTool(t, upstream.URL, token, "save_learning_record", saveInput{
		RequestID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		Theme:     "loops",
		Questions: []saveQuestion{{
			Type:     "free_text",
			Prompt:   "why",
			Answer:   saveAnswer{Text: "because", HintUsed: &hint},
			Feedback: saveFeedback{Understanding: "できた", Explanation: "ok"},
		}},
	})
	if isError || !strings.Contains(text, "rec-1") || strings.Contains(text, token) {
		t.Fatalf("isError=%v text=%s", isError, text)
	}
}

func TestSaveToolHidesTokenOnConflict(t *testing.T) {
	const token = "clc_secret_value"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"request_conflict","detail":"` + token + `"}`))
	}))
	defer upstream.Close()

	text, isError := callTool(t, upstream.URL, token, "save_learning_record", saveInput{})
	if !isError || !strings.Contains(text, "409 request_conflict") || strings.Contains(text, token) {
		t.Fatalf("isError=%v text=%s", isError, text)
	}
}

func callList(t *testing.T, baseURL, token string) (string, bool) {
	t.Helper()
	return callTool(t, baseURL, token, "list_learning_records", nil)
}

func callTool(t *testing.T, baseURL, token, name string, args any) (string, bool) {
	t.Helper()
	client, err := api.New(baseURL, token)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := New(client).Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	var text string
	for _, item := range result.Content {
		if typed, ok := item.(*mcp.TextContent); ok {
			text += typed.Text
		}
	}
	return text, result.IsError
}

func ioWrite(w http.ResponseWriter, body string) (int, error) {
	return w.Write([]byte(body))
}
