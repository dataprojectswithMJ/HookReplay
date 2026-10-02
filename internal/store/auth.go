package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hookreplay/hookreplay/internal/auth"
	"github.com/hookreplay/hookreplay/internal/id"
	"github.com/jackc/pgx/v5"
)

// APIKey is a non-secret view of an API key.
type APIKey struct {
	ID         string     `json:"id"`
	Prefix     string     `json:"prefix"`
	Name       string     `json:"name,omitempty"`
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ErrAPIKeyNameExists is returned when a workspace already has a key with the
// requested name.
var ErrAPIKeyNameExists = errors.New("an API key with this name already exists")

// User is a user account.
type User struct {
	ID              string `json:"id"`
	Email           string `json:"email"`
	OAuthProvider   string `json:"oauth_provider"`
	OAuthProviderID string `json:"oauth_provider_id,omitempty"`
}

// Workspace is a workspace the user belongs to.
type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Tier string `json:"tier"`
	Role string `json:"role"`
}

// CreateAPIKey generates a key, stores only its hash, and returns the raw key
// exactly once. The raw key is never persisted or readable again. An optional
// name can be attached so it can be selected later with `api-keys use <name>`.
func (s *Store) CreateAPIKey(ctx context.Context, userID, workspaceID, name string, scopes []string) (string, string, error) {
	if name != "" {
		var exists bool
		if err := s.pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM api_keys WHERE workspace_id = $1 AND name = $2)`,
			workspaceID, name).Scan(&exists); err != nil {
			return "", "", fmt.Errorf("store: check api key name: %w", err)
		}
		if exists {
			return "", "", ErrAPIKeyNameExists
		}
	}

	raw, hashed, err := auth.NewKey()
	if err != nil {
		return "", "", err
	}
	keyID := id.New("hrk_")
	var nameVal any
	if name != "" {
		nameVal = name
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO api_keys (id, user_id, workspace_id, prefix, hashed_key, scopes, name)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)`,
		keyID, userID, workspaceID, auth.DisplayPrefix(raw), hashed, jsonStr(scopes), nameVal)
	if err != nil {
		return "", "", fmt.Errorf("store: create api key: %w", err)
	}
	return raw, keyID, nil
}

// ListAPIKeys returns all keys for a workspace (never the raw key/hash).
func (s *Store) ListAPIKeys(ctx context.Context, workspaceID string) ([]APIKey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, prefix, name, scopes, last_used_at, created_at
		FROM api_keys WHERE workspace_id = $1 ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("store: list api keys: %w", err)
	}
	defer rows.Close()
	var out []APIKey = []APIKey{}
	for rows.Next() {
		var k APIKey
		var scopes []byte
		var lastUsed *time.Time
		var name *string
		if err := rows.Scan(&k.ID, &k.Prefix, &name, &scopes, &lastUsed, &k.CreatedAt); err != nil {
			return nil, err
		}
		if name != nil {
			k.Name = *name
		}
		k.Scopes = scopesFromJSON(scopes)
		k.LastUsedAt = lastUsed
		out = append(out, k)
	}
	return out, rows.Err()
}

// CreateSession creates a user session and returns its raw token once. The raw
// token is never persisted or readable again (only its hash).
func (s *Store) CreateSession(ctx context.Context, userID string) (string, error) {
	raw, hashed, err := auth.NewSessionToken()
	if err != nil {
		return "", err
	}
	sessionID := id.New("ses_")
	_, err = s.pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, now() + interval '30 days')`,
		sessionID, userID, hashed)
	if err != nil {
		return "", fmt.Errorf("store: create session: %w", err)
	}
	return raw, nil
}

// PrincipalByUserTokenHash resolves a logged-in user from a hashed session
// token. Sessions grant the user's full scopes within their first workspace.
func (s *Store) PrincipalByUserTokenHash(ctx context.Context, hashedToken string) (*Principal, error) {
	var p Principal
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, w.id, w.tier, u.email
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		JOIN workspace_members m ON m.user_id = u.id
		JOIN workspaces w ON w.id = m.workspace_id
		WHERE s.token_hash = $1 AND s.expires_at > now()
		ORDER BY w.created_at ASC
		LIMIT 1`, hashedToken).
		Scan(&p.UserID, &p.WorkspaceID, &p.Tier, &p.Email)
	if err != nil {
		return nil, fmt.Errorf("store: lookup session principal: %w", err)
	}
	p.Scopes = auth.AllScopes
	return &p, nil
}

// WorkspaceHasAPIKey reports whether a workspace already has at least one key.
func (s *Store) WorkspaceHasAPIKey(ctx context.Context, workspaceID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM api_keys WHERE workspace_id = $1)`, workspaceID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("store: check api keys: %w", err)
	}
	return exists, nil
}

// DeleteAPIKey revokes a key belonging to a workspace.
func (s *Store) DeleteAPIKey(ctx context.Context, workspaceID, keyID string) error {
	res, err := s.pool.Exec(ctx, `DELETE FROM api_keys WHERE id = $1 AND workspace_id = $2`, keyID, workspaceID)
	if err != nil {
		return fmt.Errorf("store: delete api key: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("store: api key %q not found", keyID)
	}
	return nil
}

// ErrEmailLinkedElsewhere is returned when an email is already registered via a
// different provider (multi-provider account linking is a later workstream).
var ErrEmailLinkedElsewhere = errors.New("this email is already registered with a different sign-in method")

// UpsertUserByOAuth finds or creates a user by (provider, provider_id).
func (s *Store) UpsertUserByOAuth(ctx context.Context, email, provider, providerID string) (string, error) {
	var userID string
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM users WHERE oauth_provider = $1 AND oauth_provider_id = $2`,
		provider, providerID).Scan(&userID)
	if err == nil {
		return userID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("store: find user by oauth: %w", err)
	}

	// Same email via a different provider: not linked yet.
	var existingID string
	err = s.pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&existingID)
	if err == nil {
		return "", ErrEmailLinkedElsewhere
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("store: find user by email: %w", err)
	}

	userID = id.New("usr_")
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, email, oauth_provider, oauth_provider_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING`, userID, email, provider, providerID); err != nil {
		return "", fmt.Errorf("store: create user: %w", err)
	}
	return userID, nil
}

// CreateWorkspace creates a workspace and adds the user as owner.
func (s *Store) CreateWorkspace(ctx context.Context, name, userID string) (string, error) {
	wsID := id.New("ws_")
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO workspaces (id, name, tier) VALUES ($1, $2, 'free')`, wsID, name); err != nil {
		return "", fmt.Errorf("store: create workspace: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, wsID, userID); err != nil {
		return "", fmt.Errorf("store: create membership: %w", err)
	}
	return wsID, nil
}

// EnsureWorkspace returns the user's first workspace, creating one with the
// given name if none exists.
func (s *Store) EnsureWorkspace(ctx context.Context, userID, name string) (string, error) {
	ws, err := s.ListWorkspaces(ctx, userID)
	if err != nil {
		return "", err
	}
	if len(ws) > 0 {
		return ws[0].ID, nil
	}
	return s.CreateWorkspace(ctx, name, userID)
}

// ListWorkspaces returns the workspaces a user belongs to.
func (s *Store) ListWorkspaces(ctx context.Context, userID string) ([]Workspace, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT w.id, w.name, w.tier, m.role
		FROM workspace_members m
		JOIN workspaces w ON w.id = m.workspace_id
		WHERE m.user_id = $1 ORDER BY w.created_at ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("store: list workspaces: %w", err)
	}
	defer rows.Close()
	var out []Workspace = []Workspace{}
	for rows.Next() {
		var ws Workspace
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Tier, &ws.Role); err != nil {
			return nil, err
		}
		out = append(out, ws)
	}
	return out, rows.Err()
}

func scopesFromJSON(b []byte) []string {
	if len(b) == 0 {
		return []string{}
	}
	var scopes []string
	if err := json.Unmarshal(b, &scopes); err != nil {
		return []string{}
	}
	return scopes
}
