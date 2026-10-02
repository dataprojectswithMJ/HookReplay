package apiclient

import (
	"context"
	"net/http"
)

// ChainStepResult is a per-step result from a chain run.
type ChainStepResult struct {
	Name         string   `json:"name"`
	Status       int      `json:"status"`
	Body         string   `json:"body,omitempty"` // base64
	LatencyMS    int64    `json:"latency_ms"`
	Retries      int      `json:"retries,omitempty"`
	Delays       []string `json:"delays,omitempty"`
	Error        string   `json:"error,omitempty"`
	Skipped      bool     `json:"skipped"`
	Failed       bool     `json:"failed"`
	ExpectErrors []string `json:"expect_errors,omitempty"`
}

// ChainRunResult is the aggregate chain run result.
type ChainRunResult struct {
	Name   string            `json:"name"`
	Steps  []ChainStepResult `json:"steps"`
	Passed bool              `json:"passed"`
}

// ExecuteChain runs a chain server-side and returns per-step results.
func (c *Client) ExecuteChain(ctx context.Context, yamlText, target, env string) (*ChainRunResult, error) {
	var out ChainRunResult
	if err := c.do(ctx, http.MethodPost, "/v1/chains/execute", map[string]string{
		"yaml_text":   yamlText,
		"target":      target,
		"environment": env,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
