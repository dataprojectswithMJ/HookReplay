package store

import (
	"context"
	"fmt"
	"time"

	"github.com/hookreplay/hookreplay/internal/id"
)

// Chain is a stored chain (dashboard- or repo-authored).
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

// CreateChain stores a new chain.
func (s *Store) CreateChain(ctx context.Context, wsID, source, name, yamlText string) (*Chain, error) {
	c := &Chain{
		ID:          id.New("chn_"),
		WorkspaceID: wsID,
		Source:      source,
		Name:        name,
		YAMLText:    yamlText,
		Version:     1,
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO chains (id, workspace_id, source, name, yaml_text, version)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		c.ID, c.WorkspaceID, c.Source, c.Name, c.YAMLText, c.Version); err != nil {
		return nil, fmt.Errorf("store: create chain: %w", err)
	}
	return c, nil
}

// ListChains returns a workspace's chains, newest first.
func (s *Store) ListChains(ctx context.Context, wsID string) ([]Chain, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, workspace_id, source, name, yaml_text, version, created_at, updated_at
		FROM chains WHERE workspace_id = $1 ORDER BY updated_at DESC`, wsID)
	if err != nil {
		return nil, fmt.Errorf("store: list chains: %w", err)
	}
	defer rows.Close()
	var out []Chain = []Chain{}
	for rows.Next() {
		var c Chain
		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.Source, &c.Name, &c.YAMLText, &c.Version, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetChain returns one chain scoped to a workspace.
func (s *Store) GetChain(ctx context.Context, wsID, chainID string) (*Chain, error) {
	var c Chain
	err := s.pool.QueryRow(ctx, `
		SELECT id, workspace_id, source, name, yaml_text, version, created_at, updated_at
		FROM chains WHERE id = $1 AND workspace_id = $2`, chainID, wsID).
		Scan(&c.ID, &c.WorkspaceID, &c.Source, &c.Name, &c.YAMLText, &c.Version, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("store: get chain: %w", err)
	}
	return &c, nil
}

// UpdateChain bumps the version and replaces the YAML text.
func (s *Store) UpdateChain(ctx context.Context, wsID, chainID, yamlText string) error {
	res, err := s.pool.Exec(ctx, `
		UPDATE chains SET yaml_text = $1, version = version + 1, updated_at = now()
		WHERE id = $2 AND workspace_id = $3`, yamlText, chainID, wsID)
	if err != nil {
		return fmt.Errorf("store: update chain: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("chain not found")
	}
	return nil
}

// DeleteChain removes a chain.
func (s *Store) DeleteChain(ctx context.Context, wsID, chainID string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM chains WHERE id = $1 AND workspace_id = $2`, chainID, wsID); err != nil {
		return fmt.Errorf("store: delete chain: %w", err)
	}
	return nil
}

// CountExecutionsSince returns the number of executions since t (for usage).
func (s *Store) CountExecutionsSince(ctx context.Context, wsID string, since time.Time) (int, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM executions WHERE workspace_id = $1 AND created_at >= $2`, wsID, since).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: count executions: %w", err)
	}
	return n, nil
}
