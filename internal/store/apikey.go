package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// APIKey represents a row in the api_keys table.
type APIKey struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	KeyHash   string
	KeyPrefix string
	IsActive  bool
	CreatedAt time.Time
}

// CreateAPIKey inserts a new API key into the database.
//
// Parameters:
//   - userID: which user this key belongs to
//   - name: a label like "production-key" (user chooses this)
//   - keyHash: SHA256 hash of the full API key
//   - keyPrefix: first 12 chars of the key (for display: "sk-vh-a3b1...")
//
// Note: we store the HASH, not the real key. The real key is returned
// to the user exactly once when they create it, then discarded.
func (s *Store) CreateAPIKey(ctx context.Context, userID uuid.UUID, name, keyHash, keyPrefix string) (*APIKey, error) {
	var key APIKey
	err := s.db.QueryRow(ctx,
		`INSERT INTO api_keys (user_id, name, key_hash, key_prefix)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, name, key_hash, key_prefix, is_active, created_at`,
		userID, name, keyHash, keyPrefix,
	).Scan(&key.ID, &key.UserID, &key.Name, &key.KeyHash, &key.KeyPrefix, &key.IsActive, &key.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create api key: %w", err)
	}
	return &key, nil
}

// GetAPIKeyByHash looks up an API key by its SHA256 hash.
//
// This is called on EVERY request to /v1/chat/completions:
//   1. Client sends: "Authorization: Bearer sk-vh-a3b1c5d7..."
//   2. We SHA256 hash the key
//   3. Look up the hash in the database
//   4. If found + active → allow the request
//
// The index we created in the migration makes this fast (O(log n) not O(n)).
func (s *Store) GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	var key APIKey
	err := s.db.QueryRow(ctx,
		`SELECT id, user_id, name, key_hash, key_prefix, is_active, created_at
		 FROM api_keys WHERE key_hash = $1 AND is_active = true`,
		keyHash,
	).Scan(&key.ID, &key.UserID, &key.Name, &key.KeyHash, &key.KeyPrefix, &key.IsActive, &key.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("api key not found: %w", err)
	}
	return &key, nil
}

// ListAPIKeysByUser returns all API keys for a user.
// Used by GET /v1/keys endpoint to show the user their keys.
// We only return the prefix (not the hash) — the real key is gone.
func (s *Store) ListAPIKeysByUser(ctx context.Context, userID uuid.UUID) ([]APIKey, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, name, key_hash, key_prefix, is_active, created_at
		 FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list api keys: %w", err)
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix, &k.IsActive, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, nil
}

// DeactivateAPIKey sets is_active = false on an API key.
// This "revokes" the key — it can no longer be used.
func (s *Store) DeactivateAPIKey(ctx context.Context, keyID uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		`UPDATE api_keys SET is_active = false WHERE id = $1`,
		keyID,
	)
	if err != nil {
		return fmt.Errorf("failed to deactivate api key: %w", err)
	}
	return nil
}
