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

	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	GitHubToken string
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

	s3Endpoint := os.Getenv("S3_ENDPOINT")
	if s3Endpoint == "" {
		return Config{}, fmt.Errorf("S3_ENDPOINT is required")
	}

	s3AccessKey := os.Getenv("S3_ACCESS_KEY")
	if s3AccessKey == "" {
		return Config{}, fmt.Errorf("S3_ACCESS_KEY is required")
	}

	s3SecretKey := os.Getenv("S3_SECRET_KEY")
	if s3SecretKey == "" {
		return Config{}, fmt.Errorf("S3_SECRET_KEY is required")
	}

	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		return Config{}, fmt.Errorf("S3_BUCKET is required")
	}

	githubToken := os.Getenv("GITHUB_TOKEN")

	return Config{
		Port:               port,
		DatabaseURL:        databaseURL,
		CORSOrigin:         corsOrigin,
		GitHubClientID:     githubClientID,
		GitHubClientSecret: githubClientSecret,
		GitHubCallbackURL:  githubCallbackURL,
		S3Endpoint:         s3Endpoint,
		S3AccessKey:        s3AccessKey,
		S3SecretKey:        s3SecretKey,
		S3Bucket:           s3Bucket,
		GitHubToken:        githubToken,
	}, nil
}
