package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/config"
)

type Handler struct {
	cfg   config.Config
	store *Store
}

type meResponse struct {
	ID          string  `json:"id"`
	GitHubLogin string  `json:"github_login"`
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
}

func NewHandler(cfg config.Config, store *Store) *Handler {
	return &Handler{
		cfg:   cfg,
		store: store,
	}
}

func (h *Handler) GitHubLogin(w http.ResponseWriter, r *http.Request) {
	randomBytes := make([]byte, 32)

	if _, err := rand.Read(randomBytes); err != nil {
		http.Error(
			w,
			"failed to start GitHub login",
			http.StatusInternalServerError,
		)
		return
	}

	state := hex.EncodeToString(randomBytes)

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   10 * 60,
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	params := url.Values{}
	params.Set("client_id", h.cfg.GitHubClientID)
	params.Set("redirect_uri", h.cfg.GitHubCallbackURL)
	params.Set("state", state)

	authorizeURL := "https://github.com/login/oauth/authorize?" + params.Encode()

	http.Redirect(w, r, authorizeURL, http.StatusFound)
}

func (h *Handler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, cookieErr := r.Cookie("oauth_state")

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	if r.URL.Query().Get("error") != "" {
		frontendURL, err := url.Parse(h.cfg.CORSOrigin)
		if err != nil {
			http.Error(
				w,
				"invalid frontend URL",
				http.StatusInternalServerError,
			)
			return
		}

		params := frontendURL.Query()
		params.Set("auth_error", "denied")
		frontendURL.RawQuery = params.Encode()

		http.Redirect(w, r, frontendURL.String(), http.StatusFound)
		return
	}

	state := r.URL.Query().Get("state")

	if cookieErr != nil ||
		state == "" ||
		stateCookie.Value == "" ||
		state != stateCookie.Value {
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing OAuth code", http.StatusBadRequest)
		return
	}

	githubID, login, name, avatarURL, err := FetchGitHubUser(h.cfg, code)
	if err != nil {
		http.Error(
			w,
			"failed to fetch GitHub user",
			http.StatusBadGateway,
		)
		return
	}

	userID, err := h.store.UpsertUser(
		r.Context(),
		githubID,
		login,
		sql.NullString{
			String: name,
			Valid:  name != "",
		},
		sql.NullString{
			String: avatarURL,
			Valid:  avatarURL != "",
		},
	)
	if err != nil {
		http.Error(
			w,
			"failed to save user",
			http.StatusInternalServerError,
		)
		return
	}

	token, tokenHash, err := NewSessionToken()
	if err != nil {
		http.Error(
			w,
			"failed to create session token",
			http.StatusInternalServerError,
		)
		return
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour).Truncate(time.Second)

	err = h.store.CreateSession(
		r.Context(),
		userID,
		tokenHash,
		expiresAt,
	)
	if err != nil {
		http.Error(
			w,
			"failed to save session",
			http.StatusInternalServerError,
		)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, h.cfg.CORSOrigin, http.StatusFound)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	response := meResponse{
		ID:          user.ID,
		GitHubLogin: user.GitHubLogin,
	}

	if user.DisplayName.Valid {
		response.DisplayName = &user.DisplayName.String
	}
	if user.AvatarURL.Valid {
		response.AvatarURL = &user.AvatarURL.String
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(response)
}
