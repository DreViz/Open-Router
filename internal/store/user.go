package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────────────
// User represents a row in the users table.
//
// Why separate from model/chat.go?
//   model/ = types for the API (what the client sends/receives)
//   store/ = types for the database (what PostgreSQL stores)
//
// They're different because the database has fields the client never sees
// (like password_hash, created_at) and the API has fields the database
// doesn't store directly (like Temperature as a pointer).
// ──────────────────────────────────────────────────────
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// CreateUser inserts a new user into the database.
//
// Parameters:
//   - email: the user's email (must be unique)
//   - passwordHash: the bcrypt hash of their password (NOT the raw password!)
//
// Returns: the created User (with the ID that PostgreSQL generated)
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	var user User

	// db.QueryRow runs a SQL query that returns exactly one row.
	// The "RETURNING *" clause tells PostgreSQL to return the inserted row.
	// Scan() maps the returned columns to our User struct fields.
	err := s.db.QueryRow(ctx,
		`INSERT INTO users (email, password_hash)
		 VALUES ($1, $2)
		 RETURNING id, email, password_hash, created_at`,
		email, passwordHash,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return &user, nil
}

// GetUserByEmail finds a user by their email address.
// Used during login: user types email → we look them up → verify password.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := s.db.QueryRow(ctx,
		`SELECT id, email, password_hash, created_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &user, nil
}

// GetUserByID finds a user by their UUID.
// Used by auth middleware after looking up an API key.
func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var user User
	err := s.db.QueryRow(ctx,
		`SELECT id, email, password_hash, created_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &user, nil
}
