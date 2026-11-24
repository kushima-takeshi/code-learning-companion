package learningrecord

import (
	"net/http"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
)

// RegisterWebRoutes registers read-only learning record and review routes behind a Web session.
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
	mux.Handle("GET /web/reviews/due", protect(h.listDueReviews))
	return nil
}

func (h *Handler) listWeb(w http.ResponseWriter, r *http.Request, userID string) {
	if page, ok := h.listPage(w, r, userID); ok {
		writeJSON(w, http.StatusOK, page)
	}
}

func (h *Handler) listDueReviews(w http.ResponseWriter, r *http.Request, userID string) {
	records, err := h.service.listDueReviews(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Records []dueReview `json:"records"`
	}{Records: records})
}
