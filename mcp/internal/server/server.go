// Package server exposes the local MCP tools.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kushima-takeshi/code-learning-companion/mcp/internal/api"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// listInput carries optional paging. Nil means the query parameter is omitted.
type listInput struct {
	Limit  *int `json:"limit,omitempty"`
	Offset *int `json:"offset,omitempty"`
}

type listPage struct {
	Records []listRecord `json:"records"`
	HasMore bool         `json:"has_more"`
}

type listRecord struct {
	ID        string `json:"id"`
	Theme     string `json:"theme"`
	CreatedAt string `json:"created_at"`
}

// New registers list, save, and get.
func New(client *api.Client) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "code-learning-companion", Version: "0.1.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_learning_records",
		Description: "List saved learning records. Optional limit (1-100) and offset.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, listPage, error) {
		return listLearningRecords(ctx, client, in)
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "save_learning_record",
		Description: "Save one confirmed learning record. Does not create a new API.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in saveInput) (*mcp.CallToolResult, savedRecord, error) {
		return saveLearningRecord(ctx, client, in)
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_learning_record",
		Description: "Get one saved learning record by id. Uses the existing record API.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getInput) (*mcp.CallToolResult, savedRecord, error) {
		return getLearningRecord(ctx, client, in)
	})
	return s
}

func listLearningRecords(ctx context.Context, client *api.Client, in listInput) (*mcp.CallToolResult, listPage, error) {
	status, body, err := client.DoJSON(ctx, http.MethodGet, listPath(in), nil)
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

func listPath(in listInput) string {
	query := url.Values{}
	if in.Limit != nil {
		query.Set("limit", strconv.Itoa(*in.Limit))
	}
	if in.Offset != nil {
		query.Set("offset", strconv.Itoa(*in.Offset))
	}
	if len(query) == 0 {
		return "/learning-records"
	}
	return "/learning-records?" + query.Encode()
}

func failureText(action string, status int, body []byte) string {
	code := "unknown"
	var parsed struct {
		Error   string        `json:"error"`
		Details []fieldDetail `json:"details"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error != "" {
		code = parsed.Error
	}
	text := action + " failed: " + strconv.Itoa(status) + " " + code
	if code == "validation_failed" {
		if extra := formatDetails(parsed.Details); extra != "" {
			text += " " + extra
		}
	}
	return text
}

// fieldDetail is the API's FieldError. Other JSON fields are ignored.
type fieldDetail struct {
	Path          string   `json:"path"`
	Code          string   `json:"code"`
	AllowedValues []string `json:"allowed_values"`
}

func formatDetails(details []fieldDetail) string {
	if len(details) == 0 {
		return ""
	}
	parts := make([]string, 0, len(details))
	for _, detail := range details {
		item := "path=" + detail.Path + " code=" + detail.Code
		if len(detail.AllowedValues) > 0 {
			item += " allowed_values=" + strings.Join(detail.AllowedValues, ",")
		}
		parts = append(parts, item)
	}
	return "details " + strings.Join(parts, "; ")
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

type getInput struct {
	ID string `json:"id"`
}

func getLearningRecord(ctx context.Context, client *api.Client, in getInput) (*mcp.CallToolResult, savedRecord, error) {
	path := "/learning-records/" + url.PathEscape(in.ID)
	status, body, err := client.DoJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, savedRecord{}, errors.New("get learning record failed: " + err.Error())
	}
	if status != http.StatusOK {
		return nil, savedRecord{}, errors.New(failureText("get learning record", status, body))
	}
	var record savedRecord
	if err := json.Unmarshal(body, &record); err != nil {
		return nil, savedRecord{}, errors.New("get learning record failed: invalid response")
	}
	return nil, record, nil
}
