// Package store is the PostgreSQL persistence layer. It owns schema
// migrations (embedded), connection management, and typed queries.
package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/hookreplay/hookreplay/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store wraps a pgx connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// Principal is the authenticated caller resolved from an API key.
type Principal struct {
	UserID      string
	WorkspaceID string
	Tier        string
	Email       string
	APIKeyID    string
	Scopes      []string
}

// HasScope reports whether the principal's key grants the given scope.
func (p *Principal) HasScope(scope string) bool {
	for _, s := range p.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// New connects to PostgreSQL, applies migrations, and seeds dev data.
func New(ctx context.Context, databaseURL string, devAPIKey string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: parse config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	s := &Store{pool: pool}
	if err := s.migrate(ctx); err != nil {
		return nil, err
	}
	if err := s.seed(ctx, devAPIKey); err != nil {
		return nil, err
	}
	return s, nil
}

// Close releases the connection pool.
func (s *Store) Close() { s.pool.Close() }

func (s *Store) migrate(ctx context.Context) error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("store: read migrations: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	if _, err := s.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("store: migrations table: %w", err)
	}

	for _, name := range names {
		var exists bool
		if err := s.pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name,
		).Scan(&exists); err != nil {
			return fmt.Errorf("store: migration check %s: %w", name, err)
		}
		if exists {
			continue
		}
		sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("store: read migration %s: %w", name, err)
		}
		for i, stmt := range splitStatements(string(sqlBytes)) {
			if _, err := s.pool.Exec(ctx, stmt); err != nil {
				return fmt.Errorf("store: apply migration %s stmt %d: %w", name, i+1, err)
			}
		}
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			return fmt.Errorf("store: record migration %s: %w", name, err)
		}
	}
	return nil
}

// seed creates a default workspace, dev user, and API key so the slice runs
// out of the box against the key from HOOKREPLAY_DEV_API_KEY.
func (s *Store) seed(ctx context.Context, devAPIKey string) error {
	const (
		wsID  = "ws_dev_00000000000000000000000001"
		user  = "usr_dev_00000000000000000000000001"
		keyID = "hrk_dev_00000000000000000000000001"
	)
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO workspaces (id, name, tier) VALUES ($1, 'Local Dev', 'free')
		ON CONFLICT (id) DO NOTHING`, wsID); err != nil {
		return fmt.Errorf("store: seed workspace: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, email, oauth_provider) VALUES ($1, 'dev@local', 'dev')
		ON CONFLICT (id) DO NOTHING`, user); err != nil {
		return fmt.Errorf("store: seed user: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')
		ON CONFLICT DO NOTHING`, wsID, user); err != nil {
		return fmt.Errorf("store: seed membership: %w", err)
	}
	if devAPIKey == "" {
		devAPIKey = os.Getenv("HOOKREPLAY_DEV_API_KEY")
	}
	if devAPIKey == "" {
		devAPIKey = "hrk_dev_local_dev_only_key"
	}
	hash := auth.Hash(devAPIKey)
	prefix := devAPIKey
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO api_keys (id, user_id, workspace_id, prefix, hashed_key, scopes)
		VALUES ($1, $2, $3, $4, $5, '["dispatch","secrets","read","admin"]'::jsonb)
		ON CONFLICT (id) DO UPDATE SET scopes = EXCLUDED.scopes`, keyID, user, wsID, prefix, hash); err != nil {
		return fmt.Errorf("store: seed api key: %w", err)
	}
	return nil
}

// PrincipalByAPIKeyHash resolves an authenticated principal from a hashed key.
func (s *Store) PrincipalByAPIKeyHash(ctx context.Context, hashedKey string) (*Principal, error) {
	var p Principal
	var scopes []byte
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, w.id, w.tier, u.email, k.id, k.scopes
		FROM api_keys k
		JOIN users u ON u.id = k.user_id
		JOIN workspaces w ON w.id = k.workspace_id
		WHERE k.hashed_key = $1`, hashedKey).
		Scan(&p.UserID, &p.WorkspaceID, &p.Tier, &p.Email, &p.APIKeyID, &scopes)
	if err != nil {
		return nil, fmt.Errorf("store: lookup principal: %w", err)
	}
	p.Scopes = scopesFromJSON(scopes)
	_, _ = s.pool.Exec(ctx, `UPDATE api_keys SET last_used_at = now() WHERE id = $1`, p.APIKeyID)
	return &p, nil
}

// splitStatements splits a SQL script into individual statements on
// semicolons. It assumes the script contains no semicolons inside string
// literals (true for the embedded migrations).
func splitStatements(script string) []string {
	var out []string
	for _, part := range strings.Split(script, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}
