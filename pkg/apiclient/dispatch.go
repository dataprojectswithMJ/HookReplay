package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// DeliverConfig controls failure simulation for a dispatch (§7.2).
type DeliverConfig struct {
	Result string `json:"result,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// DispatchRequest is a single webhook fire (§5.4).
type DispatchRequest struct {
	Template        string         `json:"template,omitempty"`
	Target          string         `json:"target"`
	SecretEnvRef    string         `json:"secret_env_ref,omitempty"`
	Secret          string         `json:"secret,omitempty"`
	Environment     string         `json:"environment,omitempty"`
	Overrides       map[string]any `json:"overrides,omitempty"`
	Deliver         *DeliverConfig `json:"deliver,omitempty"`
	IdempotencyKey  string         `json:"idempotency_key,omitempty"`
	SuppressRetries bool           `json:"suppress_retries,omitempty"`
}

// Execution is a persisted dispatch result.
type Execution struct {
	ID                string            `json:"id"`
	WorkspaceID       string            `json:"workspace_id"`
	StepName          string            `json:"step_name"`
	TargetKind        string            `json:"target_kind"`
	Target            string            `json:"target"`
	Provider          string            `json:"provider"`
	TemplateID        string            `json:"template_id"`
	RequestBody       string            `json:"request_body"` // base64
	RequestBodyLen    int               `json:"request_body_len"`
	RequestBodySHA    string            `json:"request_body_sha256"`
	RequestHeaders    map[string]string `json:"request_headers"`
	SignatureHeader   map[string]string `json:"signature_header"`
	ResponseStatus    int               `json:"response_status"`
	ResponseBody      string            `json:"response_body"` // base64
	LatencyMS         int64             `json:"latency_ms"`
	Attempt           int               `json:"attempt"`
	ParentExecutionID string            `json:"parent_execution_id,omitempty"`
	ErrorCode         string            `json:"error_code,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
}

func (c *Client) Dispatch(ctx context.Context, req DispatchRequest) (*Execution, error) {
	var e Execution
	if err := c.do(ctx, http.MethodPost, "/v1/dispatch", req, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

func (c *Client) GetExecution(ctx context.Context, id string) (*Execution, error) {
	var e Execution
	if err := c.do(ctx, http.MethodGet, "/v1/executions/"+id, nil, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

func (c *Client) ListExecutions(ctx context.Context, limit int) ([]Execution, error) {
	if limit <= 0 {
		limit = 50
	}
	var out []Execution
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/v1/executions?limit=%d", limit), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// --- Secrets ---

// SecretName is the only shape a secret is ever returned in (never a value).
type SecretName struct {
	Env       string    `json:"env"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *Client) SetSecret(ctx context.Context, env, name, value string) error {
	return c.do(ctx, http.MethodPut, fmt.Sprintf("/v1/secrets/%s/%s", env, name),
		map[string]string{"value": value}, nil)
}

func (c *Client) DeleteSecret(ctx context.Context, env, name string) error {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/v1/secrets/%s/%s", env, name), nil, nil)
}

func (c *Client) ListSecrets(ctx context.Context) ([]SecretName, error) {
	var out []SecretName
	if err := c.do(ctx, http.MethodGet, "/v1/secrets", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
