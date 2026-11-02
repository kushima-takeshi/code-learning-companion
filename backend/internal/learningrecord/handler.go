package learningrecord

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/connectiontoken"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(mux *http.ServeMux, tokens *connectiontoken.Service) {
	mux.Handle("GET /learning-records", tokens.RequireToken(http.HandlerFunc(h.List)))
	mux.Handle("POST /learning-records", tokens.RequireToken(http.HandlerFunc(h.Save)))
	mux.Handle("GET /learning-records/{id}", tokens.RequireToken(http.HandlerFunc(h.Get)))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	token, ok := connectiontoken.TokenFromContext(r.Context())
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	limit, offset, err := parseListQuery(r.URL.RawQuery)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_pagination"})
		return
	}
	page, err := h.service.List(r.Context(), token.UserID, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func parseListQuery(raw string) (int, int, error) {
	query, err := url.ParseQuery(raw)
	if err != nil {
		return 0, 0, ErrInvalidPagination
	}
	limit, offset := DefaultListLimit, 0
	for key, values := range query {
		if (key != "limit" && key != "offset") || len(values) != 1 || values[0] == "" {
			return 0, 0, ErrInvalidPagination
		}
		for _, c := range values[0] {
			if c < '0' || c > '9' {
				return 0, 0, ErrInvalidPagination
			}
		}
		n, err := strconv.Atoi(values[0])
		if err != nil {
			return 0, 0, ErrInvalidPagination
		}
		if key == "limit" {
			limit = n
		} else {
			offset = n
		}
	}
	if limit < 1 || limit > MaxListLimit {
		return 0, 0, ErrInvalidPagination
	}
	return limit, offset, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
