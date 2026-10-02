package store

import (
	"context"
	"fmt"
	"time"

	"github.com/hookreplay/hookreplay/internal/id"
)

// Domain is a workspace's endpoint-verification domain.
type Domain struct {
	ID                string     `json:"id"`
	WorkspaceID       string     `json:"workspace_id"`
	Domain            string     `json:"domain"`
	VerificationToken string     `json:"verification_token"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
	Method            string     `json:"method"`
	CreatedAt         time.Time  `json:"created_at"`
}

// CreateDomain registers a domain with a served-token verification method.
func (s *Store) CreateDomain(ctx context.Context, wsID, domain string) (*Domain, error) {
	d := &Domain{
		ID:                id.New("dom_"),
		WorkspaceID:       wsID,
		Domain:            domain,
		VerificationToken: id.New("verify_"),
		Method:            "served_token",
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO domains (id, workspace_id, domain, verification_token, method)
		VALUES ($1,$2,$3,$4,$5)`,
		d.ID, d.WorkspaceID, d.Domain, d.VerificationToken, d.Method); err != nil {
		return nil, fmt.Errorf("store: create domain: %w", err)
	}
	return d, nil
}

// ListDomains returns a workspace's domains, newest first.
func (s *Store) ListDomains(ctx context.Context, wsID string) ([]Domain, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, workspace_id, domain, verification_token, verified_at, method, created_at
		FROM domains WHERE workspace_id = $1 ORDER BY created_at DESC`, wsID)
	if err != nil {
		return nil, fmt.Errorf("store: list domains: %w", err)
	}
	defer rows.Close()
	var out []Domain = []Domain{}
	for rows.Next() {
		var d Domain
		var verifiedAt *time.Time
		if err := rows.Scan(&d.ID, &d.WorkspaceID, &d.Domain, &d.VerificationToken, &verifiedAt, &d.Method, &d.CreatedAt); err != nil {
			return nil, err
		}
		d.VerifiedAt = verifiedAt
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetDomain returns one domain.
func (s *Store) GetDomain(ctx context.Context, wsID, domainID string) (*Domain, error) {
	var d Domain
	var verifiedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT id, workspace_id, domain, verification_token, verified_at, method, created_at
		FROM domains WHERE id = $1 AND workspace_id = $2`, domainID, wsID).
		Scan(&d.ID, &d.WorkspaceID, &d.Domain, &d.VerificationToken, &verifiedAt, &d.Method, &d.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("store: get domain: %w", err)
	}
	d.VerifiedAt = verifiedAt
	return &d, nil
}

// MarkDomainVerified stamps verified_at.
func (s *Store) MarkDomainVerified(ctx context.Context, wsID, domainID string) error {
	if _, err := s.pool.Exec(ctx, `
		UPDATE domains SET verified_at = now() WHERE id = $1 AND workspace_id = $2`, domainID, wsID); err != nil {
		return fmt.Errorf("store: verify domain: %w", err)
	}
	return nil
}
