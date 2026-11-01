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
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_record_id"})
		return
	}
	record, err := h.service.Get(r.Context(), token.UserID, id.String())
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "record_not_found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	writeJSON(w, http.StatusOK, publicRecord(record))
}
