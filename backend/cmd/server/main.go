package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/config"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/db"
	appmigrate "github.com/kushima-takeshi/code-learning-companion/backend/internal/migrate"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	if err := appmigrate.Up(cfg.DatabaseURL, "file://migrations"); err != nil {
		log.Fatal(err)
	}
	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	authStore := auth.NewStore(database)
	authHandler := auth.NewHandler(cfg, authStore)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /auth/github/login", authHandler.GitHubLogin)
	mux.HandleFunc("GET /auth/github/callback", authHandler.GitHubCallback)

	mux.Handle(
		"GET /auth/me",
		authHandler.RequireAuth(http.HandlerFunc(authHandler.Me)),
	)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		}); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/health/db", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")

		if err := database.PingContext(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "error",
				"detail": "database unreachable",
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	addr := ":" + cfg.Port
	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, withCORS(cfg.CORSOrigin, mux)); err != nil {
		log.Fatal(err)
	}

}

func withCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
