package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// CreditAccount represents a row in the credit_accounts table.
type CreditAccount struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Balance   float64
}

// CreateCreditAccount creates a new credit account for a user.
// Called during signup — new users start with the given initial balance.
func (s *Store) CreateCreditAccount(ctx context.Context, userID uuid.UUID, initialBalance float64) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO credit_accounts (user_id, balance) VALUES ($1, $2)`,
		userID, initialBalance,
	)
	if err != nil {
		return fmt.Errorf("failed to create credit account: %w", err)
	}
	return nil
}

// GetBalance returns a user's current credit balance.
func (s *Store) GetBalance(ctx context.Context, userID uuid.UUID) (float64, error) {
	var balance float64
	err := s.db.QueryRow(ctx,
		`SELECT balance FROM credit_accounts WHERE user_id = $1`,
		userID,
	).Scan(&balance)

	if err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}
	return balance, nil
}

// DeductCredits atomically deducts credits from a user's account.
//
// ATOMIC means: this operation cannot be interrupted or cause race conditions.
//
// THE WRONG WAY (race condition):
//   balance = SELECT balance WHERE user_id = X   -- read: $10.00
//   balance = balance - cost                       -- calculate: $9.97
//   UPDATE SET balance = $9.97 WHERE user_id = X  -- write: $9.97
//   Problem: if TWO requests run at the same time, both read $10.00,
//   both deduct, both write $9.97 — one deduction is LOST.
//
// THE RIGHT WAY (atomic):
//   UPDATE credit_accounts
//   SET balance = balance - $cost
//   WHERE user_id = $1 AND balance >= $cost
//   RETURNING balance;
//
//   PostgreSQL handles this as ONE operation — no other query can
//   read or write this row in between. If balance < cost, 0 rows are
//   updated, which means "insufficient credits."
func (s *Store) DeductCredits(ctx context.Context, userID uuid.UUID, amount float64) error {
	var newBalance float64
	err := s.db.QueryRow(ctx,
		`UPDATE credit_accounts
		 SET balance = balance - $1, updated_at = NOW()
		 WHERE user_id = $2 AND balance >= $1
		 RETURNING balance`,
		amount, userID,
	).Scan(&newBalance)

	if err != nil {
		// err will be "no rows in result set" if balance < amount
		return fmt.Errorf("insufficient credits")
	}
	return nil
}
