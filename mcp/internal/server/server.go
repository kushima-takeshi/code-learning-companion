// Package server exposes the local MCP tools.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/kushima-takeshi/code-learning-companion/mcp/internal/api"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// listInput has no fields. This tool takes no arguments.
type listInput struct{}

type listPage struct {
	Records []listRecord `json:"records"`
	HasMore bool         `json:"has_more"`
}

type listRecord struct {
	ID        string `json:"id"`
	Theme     string `json:"theme"`
	CreatedAt string `json:"created_at"`
}

// New registers list and save. It does not register get.
func New(client *api.Client) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "code-learning-companion", Version: "0.1.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_learning_records",
		Description: "List saved learning records for the connection token. No arguments.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ listInput) (*mcp.CallToolResult, listPage, error) {
		return listLearningRecords(ctx, client)
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "save_learning_record",
		Description: "Save one confirmed learning record. Does not create a new API.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in saveInput) (*mcp.CallToolResult, savedRecord, error) {
		return saveLearningRecord(ctx, client, in)
	})
	return s
}

func listLearningRecords(ctx context.Context, client *api.Client) (*mcp.CallToolResult, listPage, error) {
	status, body, err := client.DoJSON(ctx, http.MethodGet, "/learning-records", nil)
	if err != nil {
		return nil, listPage{}, errors.New("list learning records failed: " + err.Error())
	}
	if status != http.StatusOK {
		return nil, listPage{}, errors.New(failureText("list learning records", status, body))
	}
	var page listPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, listPage{}, errors.New("list learning records failed: invalid response")
	}
	return nil, page, nil
}

func failureText(action string, status int, body []byte) string {
	code := "unknown"
	var parsed struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error != "" {
		code = parsed.Error
	}
	return action + " failed: " + strconv.Itoa(status) + " " + code
}

// saveInput matches learningrecord.SaveInput. hint_used is required by that API.
type saveInput struct {
	RequestID string         `json:"request_id"`
	Theme     string         `json:"theme"`
	Questions []saveQuestion `json:"questions"`
}

type saveQuestion struct {
	Type     string       `json:"type"`
	Prompt   string       `json:"prompt"`
	Answer   saveAnswer   `json:"answer"`
	Feedback saveFeedback `json:"feedback"`
}

type saveAnswer struct {
	Text     string `json:"text"`
	HintUsed *bool  `json:"hint_used"`
}

type saveFeedback struct {
	Understanding string `json:"understanding"`
	Explanation   string `json:"explanation"`
}

type savedRecord struct {
	ID        string         `json:"id"`
	RequestID string         `json:"request_id"`
	Theme     string         `json:"theme"`
	Questions []saveQuestion `json:"questions"`
	CreatedAt string         `json:"created_at"`
}

func saveLearningRecord(ctx context.Context, client *api.Client, in saveInput) (*mcp.CallToolResult, savedRecord, error) {
	status, body, err := client.DoJSON(ctx, http.MethodPost, "/learning-records", in)
	if err != nil {
		return nil, savedRecord{}, errors.New("save learning record failed: " + err.Error())
	}
	if status != http.StatusOK {
		return nil, savedRecord{}, errors.New(failureText("save learning record", status, body))
	}
	var saved savedRecord
	if err := json.Unmarshal(body, &saved); err != nil {
		return nil, savedRecord{}, errors.New("save learning record failed: invalid response")
	}
	return nil, saved, nil
}
