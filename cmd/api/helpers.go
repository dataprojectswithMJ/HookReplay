package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hookreplay/hookreplay/internal/crypt"
	"github.com/hookreplay/hookreplay/internal/id"
	"github.com/hookreplay/hookreplay/internal/store"
	"github.com/hookreplay/hookreplay/pkg/apiclient"
)

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes the §2 error envelope.
func writeError(w http.ResponseWriter, status int, code, message, requestID string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":       code,
			"message":    message,
			"request_id": requestID,
		},
	})
}

func requestID(r *http.Request) string {
	if v := r.Header.Get("X-Request-ID"); v != "" {
		return v
	}
	return id.New("req_")
}

func (s *server) resolveSecret(ctx context.Context, wsID string, req apiclient.DispatchRequest) ([]byte, error) {
	if req.Secret != "" {
		// Inline secret (dev convenience; never persisted or logged).
		return []byte(req.Secret), nil
	}
	if req.SecretEnvRef != "" {
		name, ok := strings.CutPrefix(req.SecretEnvRef, "env:")
		if !ok || name == "" {
			return nil, fmt.Errorf("invalid secret_env_ref %q; expected env:NAME", req.SecretEnvRef)
		}
		env := req.Environment
		if env == "" {
			env = "local"
		}
		ct, err := s.store.GetSecretCiphertext(ctx, wsID, env, name)
		if err != nil {
			return nil, fmt.Errorf("secret %q not found in env %q", name, env)
		}
		plain, err := crypt.Decrypt(ct, s.masterKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt secret %q", name)
		}
		return plain, nil
	}
	return nil, fmt.Errorf("a secret (or secret_env_ref) is required")
}

func renderPayload(base json.RawMessage, overrides map[string]any) ([]byte, error) {
	if len(overrides) == 0 {
		return base, nil // byte-exact: no reserialization
	}
	var m map[string]any
	if err := json.Unmarshal(base, &m); err != nil {
		return nil, fmt.Errorf("template payload is not a JSON object: %w", err)
	}
	for path, val := range overrides {
		if err := setDotted(m, path, val); err != nil {
			return nil, err
		}
	}
	return json.Marshal(m)
}

func setDotted(m map[string]any, path string, val any) error {
	parts := strings.Split(path, ".")
	cur := m
	for i, p := range parts {
		if p == "" {
			return fmt.Errorf("invalid override path %q", path)
		}
		if i == len(parts)-1 {
			cur[p] = val
			return nil
		}
		next, ok := cur[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[p] = next
		}
		cur = next
	}
	return nil
}

func headerToMap(h map[string][]string) map[string]string {
	out := map[string]string{}
	for k, vs := range h {
		out[k] = strings.Join(vs, ", ")
	}
	return out
}

func deliverToMap(d *apiclient.DeliverConfig) map[string]string {
	if d == nil {
		return map[string]string{}
	}
	return map[string]string{"result": d.Result, "reason": d.Reason}
}

func validDeliverReason(r string) bool {
	switch r {
	case "invalid_signature", "missing_signature", "stale_signature", "malformed_payload", "connection_refused":
		return true
	}
	return false
}

func targetKind(target string) string {
	if strings.Contains(target, "/t/") {
		return "tunnel"
	}
	return "url"
}

// isTunnelOrLocal reports whether a free-tier workspace may fire at a target.
// Free tier is restricted to workspace tunnels (and localhost for dev) §5.8.
func isTunnelOrLocal(target string) bool {
	if strings.Contains(target, "/t/") {
		return true
	}
	u, err := url.Parse(target)
	if err != nil {
		return false
	}
	h := u.Hostname()
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func toExecutionJSON(e *store.Execution) apiclient.Execution {
	return apiclient.Execution{
		ID:                e.ID,
		WorkspaceID:       e.WorkspaceID,
		StepName:          e.StepName,
		TargetKind:        e.TargetKind,
		Target:            e.Target,
		Provider:          e.Provider,
		TemplateID:        e.TemplateID,
		RequestBody:       base64.StdEncoding.EncodeToString(e.RequestBody),
		RequestBodyLen:    len(e.RequestBody),
		RequestBodySHA:    sha256hex(e.RequestBody),
		RequestHeaders:    e.RequestHeaders,
		SignatureHeader:   e.SignatureHeader,
		ResponseStatus:    e.ResponseStatus,
		ResponseBody:      base64.StdEncoding.EncodeToString(e.ResponseBody),
		LatencyMS:         e.LatencyMS,
		Attempt:           e.Attempt,
		ParentExecutionID: e.ParentExecutionID,
		ErrorCode:         e.ErrorCode,
		CreatedAt:         e.CreatedAt,
	}
}
