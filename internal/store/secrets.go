package store

import (
	"context"
	"fmt"
	"time"

	"github.com/hookreplay/hookreplay/internal/id"
)

// SecretMeta is the only shape a secret is ever listed in (never its value).
type SecretMeta struct {
	Env       string
	Name      string
	UpdatedAt time.Time
}

// SetSecret upserts the ciphertext of a secret (write-only).
func (s *Store) SetSecret(ctx context.Context, workspaceID, env, name string, ciphertext []byte) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO secrets (id, workspace_id, env, name, ciphertext)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (workspace_id, env, name)
		DO UPDATE SET ciphertext = EXCLUDED.ciphertext, updated_at = now()`,
		id.New("sec_"), workspaceID, env, name, ciphertext)
	if err != nil {
		return fmt.Errorf("store: set secret: %w", err)
	}
	return nil
}

// DeleteSecret removes a secret (write-only; no read back).
func (s *Store) DeleteSecret(ctx context.Context, workspaceID, env, name string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM secrets WHERE workspace_id = $1 AND env = $2 AND name = $3`,
		workspaceID, env, name)
	if err != nil {
		return fmt.Errorf("store: delete secret: %w", err)
	}
	return nil
}

// ListSecrets returns names + envs + updated_at only — never values.
func (s *Store) ListSecrets(ctx context.Context, workspaceID string) ([]SecretMeta, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT env, name, updated_at FROM secrets WHERE workspace_id = $1 ORDER BY env, name`,
		workspaceID)
	if err != nil {
		return nil, fmt.Errorf("store: list secrets: %w", err)
	}
	defer rows.Close()
	var out []SecretMeta = []SecretMeta{}
	for rows.Next() {
		var m SecretMeta
		if err := rows.Scan(&m.Env, &m.Name, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetSecretCiphertext returns the encrypted secret blob for the dispatch path.
// Callers MUST decrypt in-memory and zeroize; there is no plaintext read path.
func (s *Store) GetSecretCiphertext(ctx context.Context, workspaceID, env, name string) ([]byte, error) {
	var ct []byte
	err := s.pool.QueryRow(ctx,
		`SELECT ciphertext FROM secrets WHERE workspace_id = $1 AND env = $2 AND name = $3`,
		workspaceID, env, name).Scan(&ct)
	if err != nil {
		return nil, fmt.Errorf("store: get secret ciphertext: %w", err)
	}
	return ct, nil
}
