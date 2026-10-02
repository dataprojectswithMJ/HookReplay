package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// APIKey is the non-secret view of an API key.
type APIKey struct {
	ID         string     `json:"id"`
	Prefix     string     `json:"prefix"`
	Name       string     `json:"name,omitempty"`
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Workspace is a workspace membership.
type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Tier string `json:"tier"`
	Role string `json:"role"`
}

// CreateAPIKey issues a key and returns its id and raw key (raw shown once).
func (c *Client) CreateAPIKey(ctx context.Context, name string, scopes []string) (id, raw string, err error) {
	var out struct {
		ID     string `json:"id"`
		APIKey string `json:"api_key"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1/api-keys", map[string]any{"name": name, "scopes": scopes}, &out); err != nil {
		return "", "", err
	}
	return out.ID, out.APIKey, nil
}

// ListAPIKeys lists keys (never raw values).
func (c *Client) ListAPIKeys(ctx context.Context) ([]APIKey, error) {
	var out []APIKey
	if err := c.do(ctx, http.MethodGet, "/v1/api-keys", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteAPIKey revokes a key.
func (c *Client) DeleteAPIKey(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/api-keys/"+id, nil, nil)
}

// ListWorkspaces lists the authenticated user's memberships.
func (c *Client) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	var out []Workspace
	if err := c.do(ctx, http.MethodGet, "/v1/workspaces", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateWorkspace creates a workspace owned by the authenticated user.
func (c *Client) CreateWorkspace(ctx context.Context, name string) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1/workspaces", map[string]string{"name": name}, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// OAuthStartURL returns the URL the CLI should open in a browser.
func (c *Client) OAuthStartURL(state string) string {
	return c.BaseURL + "/v1/auth/oauth/start?state=" + state
}

// AuthStartURL returns the URL that opens the web login page for a CLI login.
func (c *Client) AuthStartURL(state string) string {
	return c.BaseURL + "/v1/auth/start?state=" + state
}

// OAuthConfigured reports whether the server has GitHub OAuth configured,
// probing the start endpoint without following the redirect to GitHub.
func (c *Client) OAuthConfigured(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.BaseURL+"/v1/auth/oauth/start?state=probe", nil)
	if err != nil {
		return false
	}
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Timeout:       10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 300 && resp.StatusCode < 400
}

// OAuthResult polls for the user token issued by a completed login.
func (c *Client) OAuthResult(ctx context.Context, state string) (string, error) {
	var out struct {
		UserToken string `json:"user_token"`
	}
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/v1/auth/result?state=%s", state), nil, &out); err != nil {
		return "", err
	}
	return out.UserToken, nil
}
