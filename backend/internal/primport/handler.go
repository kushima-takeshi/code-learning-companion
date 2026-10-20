package primport

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "internal_error",
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
