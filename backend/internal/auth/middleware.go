package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
)

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "unauthorized",
	})
}

func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value == "" {
			writeUnauthorized(w)
			return
		}

		tokenHash := HashToken(cookie.Value)

		user, err := h.store.FindUserByTokenHash(r.Context(), tokenHash)
		if errors.Is(err, sql.ErrNoRows) {
			writeUnauthorized(w)
			return
		}
		if err != nil {
			http.Error(
				w,
				"failed to check session",
				http.StatusInternalServerError,
			)
			return
		}

		ctx := withUser(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
