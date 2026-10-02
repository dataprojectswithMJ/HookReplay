package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Chain is a stored chain.
type Chain struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Source      string    `json:"source"`
	Name        string    `json:"name"`
	YAMLText    string    `json:"yaml_text"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Domain is an endpoint-verification domain.
type Domain struct {
	ID                string     `json:"id"`
	Domain            string     `json:"domain"`
	VerificationToken string     `json:"verification_token"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
	Method            string     `json:"method"`
}

// Usage is the workspace usage view.
type Usage struct {
	Period      string `json:"period"`
	Executions  int    `json:"executions"`
	Replays     int    `json:"replays"`
	Tier        string `json:"tier"`
	ExecLimit   any    `json:"exec_limit"`
	ChainsLimit int    `json:"chains_limit"`
}

func (c *Client) ListChains(ctx context.Context) ([]Chain, error) {
	var out []Chain
	if err := c.do(ctx, http.MethodGet, "/v1/chains", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateChain(ctx context.Context, name, yamlText, source string) (*Chain, error) {
	var out Chain
	if err := c.do(ctx, http.MethodPost, "/v1/chains", map[string]string{
		"name": name, "yaml_text": yamlText, "source": source,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteChain(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/chains/"+id, nil, nil)
}

func (c *Client) GetUsage(ctx context.Context) (*Usage, error) {
	var out Usage
	if err := c.do(ctx, http.MethodGet, "/v1/usage", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListDomains(ctx context.Context) ([]Domain, error) {
	var out []Domain
	if err := c.do(ctx, http.MethodGet, "/v1/domains", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateDomain(ctx context.Context, domain string) (*Domain, error) {
	var out Domain
	if err := c.do(ctx, http.MethodPost, "/v1/domains", map[string]string{"domain": domain}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) VerifyDomain(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/v1/domains/%s/verify", id), nil, nil)
}
