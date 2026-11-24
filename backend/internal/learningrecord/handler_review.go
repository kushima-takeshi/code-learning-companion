package learningrecord

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"
)

const maxReviewBodyBytes = 4 << 10

func (h *Handler) saveReview(w http.ResponseWriter, r *http.Request, userID string) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_record_id"})
		return
	}

	var input *struct {
		Result string          `json:"result"`
		Stage  json.RawMessage `json:"stage"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxReviewBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || input == nil || decoder.Decode(new(json.RawMessage)) != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	switch input.Result {
	case "できた", "一部できた", "もう一度", "判定保留":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_result"})
		return
	}

	var requestedStage int
	if err := json.Unmarshal(input.Stage, &requestedStage); err != nil || requestedStage < 1 || requestedStage > 3 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_stage"})
		return
	}

	stage, err := h.service.saveReview(r.Context(), userID, id.String(), input.Result, requestedStage)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "record_not_found"})
	case errors.Is(err, errReviewConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "review_conflict"})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
	default:
		writeJSON(w, http.StatusCreated, struct {
			Stage int `json:"stage"`
		}{Stage: stage})
	}
}
