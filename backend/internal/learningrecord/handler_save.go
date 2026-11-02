package learningrecord

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/connectiontoken"
)

func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	token, ok := connectiontoken.TokenFromContext(r.Context())
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "unsupported_media_type"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxSaveInputBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request_too_large"})
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		}
		return
	}
	// A JSON null is not a save object; DecodeSaveInput otherwise treats it as zero values.
	if bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	input, err := DecodeSaveInput(body)
	if err != nil {
		var validation *ValidationError
		if errors.As(err, &validation) {
			writeJSON(w, http.StatusBadRequest, struct {
				Error   string       `json:"error"`
				Details []FieldError `json:"details"`
			}{Error: "validation_failed", Details: validation.Details})
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		}
		return
	}
	record, err := h.service.Save(r.Context(), token.UserID, input)
	if errors.Is(err, ErrRequestConflict) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "request_conflict"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	writeJSON(w, http.StatusOK, publicRecord(record))
}
