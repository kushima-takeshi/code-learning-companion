package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/config"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/connectiontoken"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/db"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/github"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/learningrecord"
	appmigrate "github.com/kushima-takeshi/code-learning-companion/backend/internal/migrate"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/primport"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/storage"
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

	objectStorage := storage.New(
		cfg.S3Endpoint,
		cfg.S3AccessKey,
		cfg.S3SecretKey,
		cfg.S3Bucket,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = objectStorage.EnsureBucket(ctx)
	cancel()

	if err != nil {
		log.Fatalf("initialize object storage: %v", err)
	}

	log.Printf("storage bucket %q is ready", cfg.S3Bucket)

	authStore := auth.NewStore(database)
	authHandler := auth.NewHandler(cfg, authStore)
	githubClient := github.NewClient(cfg.GitHubToken)
	importService := primport.New(database, githubClient, objectStorage)
	importHandler := primport.NewHandler(importService)

	mux := http.NewServeMux()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	connectionTokens := connectiontoken.New(database)
	go connectiontoken.RunCleanupSchedule(ctx, connectiontoken.CleanupSchedule{
		Interval: connectiontoken.CleanupInterval,
		Now:      time.Now,
		Cleanup:  connectionTokens.CleanupUnusableTokens,
		Logf:     log.Printf,
	})
	connectionTokenHandler := connectiontoken.NewHandler(connectionTokens)
	if err := connectionTokenHandler.RegisterRoutes(mux, authHandler.RequireAuth, cfg.CORSOrigin); err != nil {
		log.Fatalf("initialize connection token management: %v", err)
	}
	learningRecords := learningrecord.New(database)
	go learningrecord.RunReviewNotificationSchedule(ctx, learningrecord.ReviewNotificationSchedule{
		Interval: learningrecord.ReviewNotificationInterval,
		Notify: func(ctx context.Context, now time.Time) error {
			return learningRecords.LogDueReviewNotifications(ctx, now, cfg.CORSOrigin, log.Printf)
		},
	})
	learningRecordHandler := learningrecord.NewHandler(learningRecords)
	learningRecordHandler.RegisterRoutes(mux, connectionTokens)
	if err := learningRecordHandler.RegisterWebRoutes(mux, authHandler.RequireAPIAuth, cfg.CORSOrigin); err != nil {
		log.Fatalf("initialize web learning records: %v", err)
	}

	mux.HandleFunc("GET /auth/github/login", authHandler.GitHubLogin)
	mux.HandleFunc("GET /auth/github/callback", authHandler.GitHubCallback)
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)

	mux.Handle(
		"GET /auth/me",
		authHandler.RequireAuth(http.HandlerFunc(authHandler.Me)),
	)

	mux.Handle(
		"POST /pr-snapshots",
		authHandler.RequireAuth(http.HandlerFunc(importHandler.Create)),
	)

	mux.Handle(
		"GET /pr-snapshots",
		authHandler.RequireAuth(http.HandlerFunc(importHandler.List)),
	)

	mux.Handle(
		"GET /pr-snapshots/{id}",
		authHandler.RequireAuth(http.HandlerFunc(importHandler.Get)),
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
	server := &http.Server{Addr: addr, Handler: withCORS(cfg.CORSOrigin, mux)}
	log.Printf("server listening on %s", addr)
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		stop()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatal(err)
		}
	}
}

func withCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
