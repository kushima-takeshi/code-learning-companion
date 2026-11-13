// Package config reads the local MCP adapter settings from the environment.
package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	tokenEnv    = "MCP_CONNECTION_TOKEN"
	baseURLEnv  = "MCP_API_BASE_URL"
	defaultBase = "http://localhost:8080"
)

// Config is the adapter's view of where to call and which token to send.
// Token is the connection secret. It must not be written to logs or errors.
type Config struct {
	Token   string
	BaseURL string
}

// Load requires MCP_CONNECTION_TOKEN. An empty MCP_API_BASE_URL uses localhost:8080.
func Load() (Config, error) {
	token := strings.TrimSpace(os.Getenv(tokenEnv))
	if token == "" {
		return Config{}, fmt.Errorf("%s is required", tokenEnv)
	}
	baseURL := strings.TrimSpace(os.Getenv(baseURLEnv))
	if baseURL == "" {
		baseURL = defaultBase
	}
	return Config{Token: token, BaseURL: strings.TrimRight(baseURL, "/")}, nil
}
