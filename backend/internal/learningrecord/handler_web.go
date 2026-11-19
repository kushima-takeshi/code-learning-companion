package learningrecord

import (
	"net/http"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
)

// RegisterWebRoutes registers the read-only history API behind a Web session.
func (h *Handler) RegisterWebRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, trustedOrigin string) error {
	protection := http.NewCrossOriginProtection()
	if err := protection.AddTrustedOrigin(trustedOrigin); err != nil {
		return err
	}
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cross_origin_forbidden"})
	}))
	protect := func(next func(http.ResponseWriter, *http.Request, string)) http.Handler {
		read := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := auth.UserFromContext(r.Context())
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			next(w, r, user.ID)
		})
		inner := requireAuth(protection.Handler(read))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			inner.ServeHTTP(w, r)
		})
	}
	mux.Handle("GET /web/learning-records", protect(h.listWeb))
	mux.Handle("GET /web/learning-records/{id}", protect(h.getRecord))
	return nil
}

func (h *Handler) listWeb(w http.ResponseWriter, r *http.Request, userID string) {
	if page, ok := h.listPage(w, r, userID); ok {
		writeJSON(w, http.StatusOK, page)
	}
}
