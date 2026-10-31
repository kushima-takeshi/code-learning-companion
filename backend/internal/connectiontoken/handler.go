package connectiontoken

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
)

const maxIssueBodyBytes = 4 << 10

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// RegisterRoutes keeps management behind Web sessions, never connection tokens.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, trustedOrigin string) error {
	protection := http.NewCrossOriginProtection()
	if err := protection.AddTrustedOrigin(trustedOrigin); err != nil {
		return err
	}
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeManagementJSON(w, http.StatusForbidden, map[string]string{"error": "cross_origin_forbidden"})
	}))
	protect := func(next http.HandlerFunc) http.Handler {
		inner := requireAuth(protection.Handler(next))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			inner.ServeHTTP(w, r)
		})
	}
	mux.Handle("POST /connection-tokens", protect(h.Issue))
	mux.Handle("GET /connection-tokens", protect(h.List))
	mux.Handle("POST /connection-tokens/{id}/revoke", protect(h.Revoke))
	return nil
}

// tokenResponse intentionally omits the owner, hash and secret.
type tokenResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
}

func publicToken(token Token) tokenResponse {
	return tokenResponse{ID: token.ID, Name: token.Name, CreatedAt: token.CreatedAt, ExpiresAt: token.ExpiresAt, RevokedAt: token.RevokedAt}
}

func (h *Handler) Issue(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeManagementJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		writeManagementJSON(w, 415, map[string]string{"error": "unsupported_media_type"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxIssueBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeManagementJSON(w, 413, map[string]string{"error": "request_too_large"})
		} else {
			writeManagementJSON(w, 400, map[string]string{"error": "invalid_json"})
		}
		return
	}
	var input *struct {
		Name string `json:"name"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || input == nil {
		writeManagementJSON(w, 400, map[string]string{"error": "invalid_json"})
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeManagementJSON(w, 400, map[string]string{"error": "invalid_json"})
		return
	}
	issued, err := h.service.Issue(r.Context(), user.ID, input.Name)
	if err != nil {
		if errors.Is(err, ErrInvalidName) {
			writeManagementJSON(w, 400, map[string]string{"error": "invalid_name"})
		} else {
			writeManagementJSON(w, 500, map[string]string{"error": "internal_error"})
		}
		return
	}
	writeManagementJSON(w, 201, struct {
		Token  tokenResponse `json:"token"`
		Secret string        `json:"secret"`
	}{Token: publicToken(issued.Token), Secret: issued.Secret})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeManagementJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	tokens, err := h.service.List(r.Context(), user.ID)
	if err != nil {
		writeManagementJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	response := make([]tokenResponse, 0, len(tokens))
	for _, token := range tokens {
		response = append(response, publicToken(token))
	}
	writeManagementJSON(w, http.StatusOK, response)
}

func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeManagementJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeManagementJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_token_id"})
		return
	}
	if err := h.service.Revoke(r.Context(), user.ID, id.String()); err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			writeManagementJSON(w, http.StatusNotFound, map[string]string{"error": "token_not_found"})
		} else {
			writeManagementJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func writeManagementJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
