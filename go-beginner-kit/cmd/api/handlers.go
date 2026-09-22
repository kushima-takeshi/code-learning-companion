package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"example.com/go-beginner-kit/internal/study"
)

type createRequest struct {
	Title   string `json:"title"`
	Minutes int    `json:"minutes"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(append(data, '\n')); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func newHandler(store *study.Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /records", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, store.List())
	})
	mux.HandleFunc("POST /records", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var input createRequest
		if err := decoder.Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body (limit: 1 MiB)")
			return
		}
		// 1回のDecodeだけでは、後ろに別のJSONがあっても見逃す。
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			writeError(w, http.StatusBadRequest, "body must contain exactly one JSON value")
			return
		}
		record, err := store.Add(input.Title, input.Minutes)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.Header().Set("Location", "/records/"+strconv.Itoa(record.ID))
		writeJSON(w, http.StatusCreated, record)
	})
	mux.HandleFunc("GET /records/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "id must be a positive integer")
			return
		}
		record, err := store.Find(id)
		if errors.Is(err, study.ErrNotFound) {
			writeError(w, http.StatusNotFound, "record not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, record)
	})
	return mux
}
