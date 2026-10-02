// Package auth implements API-key authentication (§5.1).
//
// API keys carry the `hrk_` prefix and are stored as SHA-256 hashes. The raw
// key is returned exactly once at creation time; no read path exists.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

const keyPrefix = "hrk_"

// API-key scopes. Enforcement is per-route in the API layer.
const (
	ScopeDispatch = "dispatch" // POST /v1/dispatch
	ScopeSecrets  = "secrets"  // write secrets
	ScopeRead     = "read"     // GET executions, templates, me, secrets
	ScopeAdmin    = "admin"    // manage API keys + workspaces
)

// AllScopes is the full permission set granted to owners / the dev key.
var AllScopes = []string{ScopeDispatch, ScopeSecrets, ScopeRead, ScopeAdmin}

// NewKey generates a new API key, returning (full key, sha256 hex hash).
func NewKey() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	full := keyPrefix + hex.EncodeToString(raw)
	return full, Hash(full), nil
}

// Hash returns the SHA-256 hex digest of a key — the only stored form.
func Hash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// DisplayPrefix returns a short, non-secret prefix for UI/CLI listing.
func DisplayPrefix(key string) string {
	const n = 12
	if len(key) <= n {
		return key
	}
	return key[:n]
}

