package primport

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/github"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "unauthorized",
		})
		return
	}

	if err := r.ParseMultipartForm(1 << 20); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid_form",
		})
		return
	}

	prURL := r.FormValue("url")

	var upload *UploadFile

	file, header, err := r.FormFile("file")

	switch {
	case errors.Is(err, http.ErrMissingFile):
		// 会話ファイルは任意なので、なしで進める。

	case err != nil:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid_file",
		})
		return

	default:
		defer file.Close()

		upload = &UploadFile{
			Filename: header.Filename,
			Reader:   file,
		}
	}

	result, err := h.service.Import(
		r.Context(),
		ImportInput{
			UserID: user.ID,
			URL:    prURL,
			File:   upload,
		},
	)

	if err != nil {
		status := http.StatusInternalServerError
		code := "internal_error"

		switch {
		case errors.Is(err, github.ErrInvalidURL):
			status = http.StatusBadRequest
			code = "invalid_url"

		case errors.Is(err, github.ErrPRNotFound):
			status = http.StatusNotFound
			code = "pr_not_found"

		case errors.Is(err, github.ErrGitHubUnavailable):
			status = http.StatusBadGateway
			code = "github_unavailable"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": code,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"snapshot_id": result.SnapshotID,
		"warning":     result.Warning,
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "unauthorized",
		})
		return
	}

	snapshots, err := h.service.List(r.Context(), user.ID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "internal_error",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(snapshots)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "unauthorized",
		})
		return
	}

	snapshotID := r.PathValue("id")

	snapshot, err := h.service.Get(
		r.Context(),
		user.ID,
		snapshotID,
	)
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal_error"

		if errors.Is(err, ErrSnapshotNotFound) {
			status = http.StatusNotFound
			code = "snapshot_not_found"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": code,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(snapshot)
}
