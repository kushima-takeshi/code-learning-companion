package config

import "testing"

func TestLoadRequiresToken(t *testing.T) {
	t.Setenv(tokenEnv, "   ")
	t.Setenv(baseURLEnv, "")
	_, err := Load()
	if err == nil {
		t.Fatal("empty token must be rejected")
	}
	if got := err.Error(); got != "MCP_CONNECTION_TOKEN is required" {
		t.Fatalf("error = %q", got)
	}
}

func TestLoadDefaultsBaseURL(t *testing.T) {
	t.Setenv(tokenEnv, "clc_secret")
	t.Setenv(baseURLEnv, "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "clc_secret" || cfg.BaseURL != defaultBase {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadTrimsBaseURLSlash(t *testing.T) {
	t.Setenv(tokenEnv, "clc_secret")
	t.Setenv(baseURLEnv, "http://127.0.0.1:8080/")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "http://127.0.0.1:8080" {
		t.Fatalf("base URL = %q", cfg.BaseURL)
	}
}
