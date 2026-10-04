package main

import (
	"context"
	"encoding/json"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/config"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/db"
	appmigrate "github.com/kushima-takeshi/code-learning-companion/backend/internal/migrate"
	"log"
	"net/http"
	"time"
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

	mux := http.NewServeMux()

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
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}

}
