// Package oauth implements OAuth code flows (GitHub + Google, WS-8).
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

// Google is a minimal Google OAuth (OpenID Connect) client.
type Google struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	client       *http.Client
}

// NewGoogle returns a Google OAuth client.
func NewGoogle(clientID, clientSecret, redirectURI string) *Google {
	return &Google{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		client:       &http.Client{Timeout: 10 * time.Second},
	}
}

// Configured reports whether Google OAuth credentials are present.
func (g *Google) Configured() bool {
	return g.ClientID != "" && g.ClientSecret != ""
}

// AuthorizeURL returns the Google authorization URL for the given state.
func (g *Google) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("client_id", g.ClientID)
	q.Set("redirect_uri", g.RedirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	return "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode()
}

// Exchange swaps an authorization code for an access token.
func (g *Google) Exchange(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", g.RedirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
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
		return "", fmt.Errorf("google oauth exchange failed: %s", out.Error)
	}
	return out.AccessToken, nil
}

// GoogleUser is the subset of the Google userinfo response we need.
type GoogleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

// User fetches the authenticated Google user.
func (g *Google) User(ctx context.Context, accessToken string) (*GoogleUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://openidconnect.googleapis.com/v1/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var u GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}
