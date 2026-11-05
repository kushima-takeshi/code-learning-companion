package connectiontoken

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
)

type reconnectionResponse struct {
	ID          string     `json:"id"`
	OldTokenID  string     `json:"old_token_id"`
	NewTokenID  string     `json:"new_token_id"`
	State       string     `json:"state"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	ConfirmedAt *time.Time `json:"confirmed_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

func publicReconnect(r Reconnection) reconnectionResponse {
	return reconnectionResponse{ID: r.ID, OldTokenID: r.OldTokenID, NewTokenID: r.NewTokenID, State: r.State, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, ConfirmedAt: r.ConfirmedAt, CompletedAt: r.CompletedAt}
}
func writeReconnect(w http.ResponseWriter, r Reconnection, secret string, status int) {
	writeManagementJSON(w, status, struct {
		Reconnection reconnectionResponse `json:"reconnection"`
		Secret       string               `json:"secret,omitempty"`
	}{Reconnection: publicReconnect(r), Secret: secret})
}

// All inputs are path IDs. Reject bodies and queries instead of silently ignoring fields.
func reconnectRequest(w http.ResponseWriter, r *http.Request, tokenID bool) (userID, id string, ok bool) {
	user, loggedIn := auth.UserFromContext(r.Context())
	if !loggedIn {
		writeManagementJSON(w, 401, map[string]string{"error": "unauthorized"})
		return "", "", false
	}
	if r.URL.RawQuery != "" {
		writeManagementJSON(w, 400, map[string]string{"error": "invalid_reconnection_request"})
		return "", "", false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxIssueBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeManagementJSON(w, 413, map[string]string{"error": "request_too_large"})
		} else {
			writeManagementJSON(w, 400, map[string]string{"error": "invalid_reconnection_request"})
		}
		return "", "", false
	}
	if len(bytes.TrimSpace(body)) != 0 {
		writeManagementJSON(w, 400, map[string]string{"error": "invalid_reconnection_request"})
		return "", "", false
	}
	parsed, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		code := "invalid_reconnection_id"
		if tokenID {
			code = "invalid_token_id"
		}
		writeManagementJSON(w, 400, map[string]string{"error": code})
		return "", "", false
	}
	return user.ID, parsed.String(), true
}
func reconnectError(w http.ResponseWriter, err error) {
	status, code := 500, "internal_error"
	switch {
	case errors.Is(err, ErrTokenNotFound):
		status, code = 404, "token_not_found"
	case errors.Is(err, ErrReconnectNotFound):
		status, code = 404, "reconnection_not_found"
	case errors.Is(err, ErrReconnectConflict):
		status, code = 409, "reconnection_conflict"
	}
	writeManagementJSON(w, status, map[string]string{"error": code})
}
func (h *Handler) StartReconnect(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := reconnectRequest(w, r, true)
	if !ok {
		return
	}
	result, err := h.service.StartReconnect(r.Context(), userID, id)
	if err != nil {
		reconnectError(w, err)
		return
	}
	status := 200
	if result.Secret != "" {
		status = 201
	}
	// Apply the same read-only status projection (manual revocation, expiry) on retries.
	if result.Secret == "" {
		result.Operation, err = h.service.GetReconnect(r.Context(), userID, result.Operation.ID)
		if err != nil {
			reconnectError(w, err)
			return
		}
	}
	writeReconnect(w, result.Operation, result.Secret, status)
}
func (h *Handler) GetReconnect(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := reconnectRequest(w, r, false)
	if !ok {
		return
	}
	operation, err := h.service.GetReconnect(r.Context(), userID, id)
	if err != nil {
		reconnectError(w, err)
		return
	}
	writeReconnect(w, operation, "", 200)
}
func (h *Handler) CompleteReconnect(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := reconnectRequest(w, r, false)
	if !ok {
		return
	}
	operation, err := h.service.CompleteReconnect(r.Context(), userID, id)
	if err != nil {
		reconnectError(w, err)
		return
	}
	writeReconnect(w, operation, "", 200)
}
func (h *Handler) RestartReconnect(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := reconnectRequest(w, r, false)
	if !ok {
		return
	}
	result, err := h.service.RestartReconnect(r.Context(), userID, id)
	if err != nil {
		reconnectError(w, err)
		return
	}
	writeReconnect(w, result.Operation, result.Secret, 201)
}
