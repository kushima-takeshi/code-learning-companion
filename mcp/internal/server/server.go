// Package server exposes list_learning_records over MCP.
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

// New registers list_learning_records. It does not register save or get.
func New(client *api.Client) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "code-learning-companion", Version: "0.1.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_learning_records",
		Description: "List saved learning records for the connection token. No arguments.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ listInput) (*mcp.CallToolResult, listPage, error) {
		return listLearningRecords(ctx, client)
	})
	return s
}

func listLearningRecords(ctx context.Context, client *api.Client) (*mcp.CallToolResult, listPage, error) {
	status, body, err := client.DoJSON(ctx, http.MethodGet, "/learning-records", nil)
	if err != nil {
		return nil, listPage{}, errors.New("list learning records failed: " + err.Error())
	}
	if status != http.StatusOK {
		return nil, listPage{}, errors.New(failureText(status, body))
	}
	var page listPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, listPage{}, errors.New("list learning records failed: invalid response")
	}
	return nil, page, nil
}

func failureText(status int, body []byte) string {
	code := "unknown"
	var parsed struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error != "" {
		code = parsed.Error
	}
	return "list learning records failed: " + strconv.Itoa(status) + " " + code
}
