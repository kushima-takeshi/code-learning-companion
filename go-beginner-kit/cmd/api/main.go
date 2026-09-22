package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"example.com/go-beginner-kit/internal/study"
)

func main() {
	server := &http.Server{
		Addr:              "127.0.0.1:8080",
		Handler:           newHandler(study.NewStore()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Print("listening on http://127.0.0.1:8080")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Print(err)
	}
}
