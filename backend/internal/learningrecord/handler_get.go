package learningrecord

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/connectiontoken"
)

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	token, ok := connectiontoken.TokenFromContext(r.Context())
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if record, ok := h.getRecord(w, r, token.UserID); ok {
		writeJSON(w, http.StatusOK, publicRecord(record))
	}
}

func (h *Handler) getWeb(w http.ResponseWriter, r *http.Request, userID string) {
	record, ok := h.getRecord(w, r, userID)
	if !ok {
		return
	}
	review, err := h.service.getReviewStatus(r.Context(), userID, record.ID)
	if !checkGetRecordError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, struct {
		recordResponse
		Review reviewStatus `json:"review"`
	}{recordResponse: publicRecord(record), Review: review})
}

func (h *Handler) getRecord(w http.ResponseWriter, r *http.Request, userID string) (Record, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_record_id"})
		return Record{}, false
	}
	record, err := h.service.Get(r.Context(), userID, id.String())
	return record, checkGetRecordError(w, err)
}

func checkGetRecordError(w http.ResponseWriter, err error) bool {
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "record_not_found"})
		return false
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return false
	}
	return true
}
