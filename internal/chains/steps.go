package chains

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hookreplay/hookreplay/internal/signing"
	"github.com/hookreplay/hookreplay/pkg/apiclient"
)

func (r *Runner) executeWebhook(ctx context.Context, step *Step, rc *runCtx) (StepResult, error) {
	req := apiclient.DispatchRequest{
		Template:    step.Webhook,
		Target:      rc.target,
		Environment: rc.env,
		Overrides:   resolveOverrides(step.Override, rc),
	}
	if name, ok := strings.CutPrefix(step.Secret, "env:"); ok {
		if v := os.Getenv(name); v != "" {
			req.Secret = v
		} else {
			req.SecretEnvRef = step.Secret
		}
	}
	if step.Deliver != nil {
		req.Deliver = &apiclient.DeliverConfig{Result: step.Deliver.Result, Reason: step.Deliver.Reason}
	}

	// Retry rehearsal: when the step asserts on re-delivery, drive retries
	// synchronously (per the provider's delivery.yml backoff) so they can be
	// observed. Suppress the dispatcher's async re-delivery scheduling so the
	// two paths don't double-fire.
	if step.Expect != nil && (step.Expect.Retries != nil || len(step.Expect.Delays) > 0) {
		req.SuppressRetries = true
		spec := deliverySpecFor(step.Webhook)
		retries, delays, exec, err := rehearseRetries(ctx, func() (*apiclient.Execution, error) {
			return r.API.Dispatch(ctx, req)
		}, spec.AttemptIntervals, spec.MaxAttempts)
		if err != nil {
			return StepResult{Name: step.Name}, err
		}
		body, _ := base64.StdEncoding.DecodeString(exec.ResponseBody)
		return StepResult{
			Name: step.Name, Status: exec.ResponseStatus, Body: body,
			LatencyMS: exec.LatencyMS, Error: exec.ErrorCode,
			Retries: retries, Delays: delays,
		}, nil
	}

	exec, err := r.API.Dispatch(ctx, req)
	if err != nil {
		return StepResult{Name: step.Name}, err
	}
	body, _ := base64.StdEncoding.DecodeString(exec.ResponseBody)
	return StepResult{
		Name:      step.Name,
		Status:    exec.ResponseStatus,
		Body:      body,
		LatencyMS: exec.LatencyMS,
		Error:     exec.ErrorCode,
	}, nil
}

func (r *Runner) executeAPI(ctx context.Context, step *Step, rc *runCtx) (StepResult, error) {
	method := step.API.Method
	if method == "" {
		method = http.MethodGet
	}
	url := rc.resolve(step.API.URL)
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return StepResult{Name: step.Name}, err
	}
	for k, v := range step.API.Headers {
		req.Header.Set(k, rc.resolve(v))
	}

	client := r.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return StepResult{Name: step.Name, LatencyMS: latency}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return StepResult{Name: step.Name, Status: resp.StatusCode, Body: body, LatencyMS: latency}, nil
}

func (r *Runner) executeCustom(ctx context.Context, step *Step, rc *runCtx) (StepResult, error) {
	body, err := customPayload(step.Custom.Payload)
	if err != nil {
		return StepResult{Name: step.Name}, err
	}

	var sigHeader http.Header
	if step.Custom.Sign != nil {
		cfg := step.Custom.Sign
		sigHeader, err = signing.Sign(signing.SchemeCustom, body,
			signing.Material{Key: []byte(resolveSecretValue(cfg.Secret)), Custom: &signing.CustomConfig{
				Algorithm:       cfg.Algorithm,
				Header:          cfg.Header,
				Format:          cfg.Format,
				Encoding:        cfg.Encoding,
				TimestampHeader: cfg.TimestampHeader,
			}}, time.Now())
		if err != nil {
			return StepResult{Name: step.Name}, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rc.target, bytes.NewReader(body))
	if err != nil {
		return StepResult{Name: step.Name}, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range step.Custom.Headers {
		req.Header.Set(k, rc.resolve(v))
	}
	for k, vs := range sigHeader {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	client := r.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return StepResult{Name: step.Name, LatencyMS: latency}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return StepResult{Name: step.Name, Status: resp.StatusCode, Body: respBody, LatencyMS: latency}, nil
}

// customPayload resolves a custom step payload: a file path (string) or inline map.
func customPayload(p any) ([]byte, error) {
	switch v := p.(type) {
	case string:
		return os.ReadFile(v)
	default:
		b, err := json.Marshal(p)
		if err != nil {
			return nil, fmt.Errorf("invalid custom payload: %w", err)
		}
		return b, nil
	}
}

// resolveSecretValue resolves env:NAME from the shell environment.
func resolveSecretValue(s string) string {
	if name, ok := strings.CutPrefix(s, "env:"); ok {
		return os.Getenv(name)
	}
	return s
}

func resolveOverrides(overrides map[string]any, rc *runCtx) map[string]any {
	if len(overrides) == 0 {
		return nil
	}
	out := make(map[string]any, len(overrides))
	for k, v := range overrides {
		if s, ok := v.(string); ok {
			out[k] = rc.resolve(s)
		} else {
			out[k] = v
		}
	}
	return out
}
