// Command adapter is the local MCP server. It speaks stdio and does not listen.
package main

import (
	"context"
	"log"
	"os"

	"github.com/kushima-takeshi/code-learning-companion/mcp/internal/api"
	"github.com/kushima-takeshi/code-learning-companion/mcp/internal/config"
	"github.com/kushima-takeshi/code-learning-companion/mcp/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	log.SetOutput(os.Stderr)
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	client, err := api.New(cfg.BaseURL, cfg.Token)
	if err != nil {
		log.Fatal(err)
	}
	if err := server.New(client).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
