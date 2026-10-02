package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/hookreplay/hookreplay/internal/id"
	"github.com/jackc/pgx/v5"
)

// CreateUserWithPassword inserts a new password user. Returns an error if the
// email is already registered (by any provider).
func (s *Store) CreateUserWithPassword(ctx context.Context, email, passwordHash string) (string, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists); err != nil {
		return "", fmt.Errorf("store: check email: %w", err)
	}
	if exists {
		return "", fmt.Errorf("an account with %q already exists", email)
	}
	userID := id.New("usr_")
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, email, oauth_provider, password_hash)
		VALUES ($1, $2, 'password', $3)`, userID, email, passwordHash); err != nil {
		return "", fmt.Errorf("store: create password user: %w", err)
	}
	return userID, nil
}

// GetUserByEmail returns the user ID and password hash for an email. It
// returns ("", "", nil) if no user exists, and (userID, "", nil) if the user
// has no password (an OAuth-only account).
func (s *Store) GetUserByEmail(ctx context.Context, email string) (string, string, error) {
	var userID string
	var hash *string
	err := s.pool.QueryRow(ctx,
		`SELECT id, password_hash FROM users WHERE email = $1`, email).Scan(&userID, &hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", nil
		}
		return "", "", fmt.Errorf("store: get user by email: %w", err)
	}
	if hash == nil {
		return userID, "", nil
	}
	return userID, *hash, nil
}
