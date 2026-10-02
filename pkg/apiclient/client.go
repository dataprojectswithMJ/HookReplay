// Package apiclient is a thin, typed HTTP client for the HookReplay API.
// It is the single integration point used by both the CLI and the dashboard
// (via equivalent fetch calls) so they share one execution path.
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Error is the wire error envelope (§2: snake_case code, human message).
type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func (e *Error) Error() string { return e.Message }

type apiErrorEnvelope struct {
	Error *Error `json:"error"`
}

// Client talks to a HookReplay API server.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// New returns a Client. apiKey may be empty for unauthenticated calls.
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 45 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		var env apiErrorEnvelope
		if err := json.Unmarshal(data, &env); err == nil && env.Error != nil {
			return env.Error
		}
		return fmt.Errorf("api: %s %s -> %d: %s", method, path, resp.StatusCode, string(data))
	}

	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// --- Auth ---

// Me describes the authenticated principal and its default workspace.
type Me struct {
	UserID      string `json:"user_id"`
	WorkspaceID string `json:"workspace_id"`
	Tier        string `json:"tier"`
	Email       string `json:"email"`
}

func (c *Client) Me(ctx context.Context) (*Me, error) {
	var m Me
	if err := c.do(ctx, http.MethodGet, "/v1/me", nil, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// --- Templates ---

// Template is the public library view of a provider template.
type Template struct {
	Provider         string          `json:"provider"`
	Event            string          `json:"event"`
	Scheme           string          `json:"scheme"`
	Note             string          `json:"note"`
	SignatureHeader  string          `json:"signature_header,omitempty"`
	Payload          json.RawMessage `json:"payload,omitempty"`
	AttemptIntervals []string        `json:"attempt_intervals,omitempty"`
	MaxAttempts      int             `json:"max_attempts,omitempty"`
	TimeoutS         int             `json:"timeout_s,omitempty"`
}

func (c *Client) ListTemplates(ctx context.Context) ([]Template, error) {
	var out []Template
	if err := c.do(ctx, http.MethodGet, "/v1/templates", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetTemplate(ctx context.Context, provider, event string) (*Template, error) {
	var t Template
	p := fmt.Sprintf("/v1/templates/%s/%s", provider, event)
	if err := c.do(ctx, http.MethodGet, p, nil, &t); err != nil {
		return nil, err
	}
	return &t, nil
}
