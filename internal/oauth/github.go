// Package oauth implements the GitHub OAuth code flow (WS-8).
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitHub is a minimal GitHub OAuth client.
type GitHub struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	client       *http.Client
}

// NewGitHub returns a GitHub OAuth client.
func NewGitHub(clientID, clientSecret, redirectURI string) *GitHub {
	return &GitHub{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		client:       &http.Client{Timeout: 10 * time.Second},
	}
}

// Configured reports whether OAuth credentials are present.
func (g *GitHub) Configured() bool {
	return g.ClientID != "" && g.ClientSecret != ""
}

// AuthorizeURL returns the GitHub authorization URL for the given state.
func (g *GitHub) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("client_id", g.ClientID)
	q.Set("redirect_uri", g.RedirectURI)
	q.Set("scope", "read:user user:email")
	q.Set("state", state)
	return "https://github.com/login/oauth/authorize?" + q.Encode()
}

// Exchange swaps an authorization code for an access token.
func (g *GitHub) Exchange(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", g.RedirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("github oauth exchange failed: %s", out.Error)
	}
	return out.AccessToken, nil
}

// GitHubUser is the subset of the GitHub /user response we need.
type GitHubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}

// User fetches the authenticated GitHub user.
func (g *GitHub) User(ctx context.Context, accessToken string) (*GitHubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var u GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

// PrimaryEmail returns the primary verified email, or "" if unavailable.
func (g *GitHub) PrimaryEmail(ctx context.Context, accessToken string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := g.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return ""
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email
		}
	}
	return ""
}
