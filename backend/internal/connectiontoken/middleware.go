package connectiontoken

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type tokenContextKey struct{}

// TokenFromContext is separate from the Web-session user context.
func TokenFromContext(ctx context.Context) (Token, bool) {
	token, ok := ctx.Value(tokenContextKey{}).(Token)
	return token, ok
}

// RequireToken is only installed on explicitly allowed learning-record routes.
func (s *Service) RequireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		unauthorized := func() {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeManagementJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}
		values := r.Header.Values("Authorization")
		if len(values) != 1 {
			unauthorized()
			return
		}
		parts := strings.Fields(values[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			unauthorized()
			return
		}
		token, err := s.Authenticate(r.Context(), parts[1])
		if errors.Is(err, ErrInvalidToken) {
			unauthorized()
			return
		}
		if err != nil {
			writeManagementJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		ctx := context.WithValue(r.Context(), tokenContextKey{}, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
