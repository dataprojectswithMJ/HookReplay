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
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

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
// exactly once. The raw key is never persisted or readable again.
func (s *Store) CreateAPIKey(ctx context.Context, userID, workspaceID string, scopes []string) (string, string, error) {
	raw, hashed, err := auth.NewKey()
	if err != nil {
		return "", "", err
	}
	keyID := id.New("hrk_")
	_, err = s.pool.Exec(ctx, `
		INSERT INTO api_keys (id, user_id, workspace_id, prefix, hashed_key, scopes)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)`,
		keyID, userID, workspaceID, auth.DisplayPrefix(raw), hashed, jsonStr(scopes))
	if err != nil {
		return "", "", fmt.Errorf("store: create api key: %w", err)
	}
	return raw, keyID, nil
}

// ListAPIKeys returns all keys for a workspace (never the raw key/hash).
func (s *Store) ListAPIKeys(ctx context.Context, workspaceID string) ([]APIKey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, prefix, scopes, last_used_at, created_at
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
		if err := rows.Scan(&k.ID, &k.Prefix, &scopes, &lastUsed, &k.CreatedAt); err != nil {
			return nil, err
		}
		k.Scopes = scopesFromJSON(scopes)
		k.LastUsedAt = lastUsed
		out = append(out, k)
	}
	return out, rows.Err()
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
