package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/config"
)

type githubTokenRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
}

type githubTokenResponse struct {
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
}

type githubUserResponse struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

func FetchGitHubUser(
	cfg config.Config,
	code string,
) (id int64, login, name, avatarURL string, err error) {
	requestBody := githubTokenRequest{
		ClientID:     cfg.GitHubClientID,
		ClientSecret: cfg.GitHubClientSecret,
		Code:         code,
		RedirectURI:  cfg.GitHubCallbackURL,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return 0, "", "", "", err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		"https://github.com/login/oauth/access_token",
		bytes.NewReader(body),
	)
	if err != nil {
		return 0, "", "", "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", "", "", fmt.Errorf("GitHub token request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, "", "", "", fmt.Errorf(
			"GitHub token request returned status %d",
			resp.StatusCode,
		)
	}

	var tokenResponse githubTokenResponse

	err = json.NewDecoder(resp.Body).Decode(&tokenResponse)
	if err != nil {
		return 0, "", "", "", fmt.Errorf(
			"failed to decode GitHub token response",
		)
	}

	if tokenResponse.Error != "" {
		return 0, "", "", "", fmt.Errorf(
			"GitHub rejected the token exchange",
		)
	}

	if tokenResponse.AccessToken == "" {
		return 0, "", "", "", fmt.Errorf(
			"GitHub token response did not contain an access token",
		)
	}

	userReq, err := http.NewRequest(
		http.MethodGet,
		"https://api.github.com/user",
		nil,
	)
	if err != nil {
		return 0, "", "", "", fmt.Errorf(
			"failed to create GitHub user request",
		)
	}

	userReq.Header.Set(
		"Authorization",
		"Bearer "+tokenResponse.AccessToken,
	)
	userReq.Header.Set("User-Agent", "code-learning-companion")
	userReq.Header.Set("Accept", "application/json")

	userResp, err := client.Do(userReq)
	if err != nil {
		return 0, "", "", "", fmt.Errorf(
			"GitHub user request failed",
		)
	}
	defer userResp.Body.Close()

	if userResp.StatusCode != http.StatusOK {
		return 0, "", "", "", fmt.Errorf(
			"GitHub user request returned status %d",
			userResp.StatusCode,
		)
	}

	var user githubUserResponse

	err = json.NewDecoder(userResp.Body).Decode(&user)
	if err != nil {
		return 0, "", "", "", fmt.Errorf(
			"failed to decode GitHub user response",
		)
	}

	if user.ID <= 0 || user.Login == "" {
		return 0, "", "", "", fmt.Errorf(
			"GitHub user response did not contain a valid identity",
		)
	}

	return user.ID, user.Login, user.Name, user.AvatarURL, nil
}
