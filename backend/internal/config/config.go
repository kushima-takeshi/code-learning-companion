package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port               string
	DatabaseURL        string
	CORSOrigin         string
	GitHubClientID     string
	GitHubClientSecret string
	GitHubCallbackURL  string
}

func Load() (Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	corsOrigin := os.Getenv("CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "http://localhost:5173"
	}

	githubClientID := os.Getenv("GITHUB_CLIENT_ID")
	if githubClientID == "" {
		return Config{}, fmt.Errorf("GITHUB_CLIENT_ID is required")
	}

	githubClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	if githubClientSecret == "" {
		return Config{}, fmt.Errorf("GITHUB_CLIENT_SECRET is required")
	}

	githubCallbackURL := os.Getenv("GITHUB_CALLBACK_URL")
	if githubCallbackURL == "" {
		return Config{}, fmt.Errorf("GITHUB_CALLBACK_URL is required")
	}

	return Config{
		Port:               port,
		DatabaseURL:        databaseURL,
		CORSOrigin:         corsOrigin,
		GitHubClientID:     githubClientID,
		GitHubClientSecret: githubClientSecret,
		GitHubCallbackURL:  githubCallbackURL,
	}, nil
}
