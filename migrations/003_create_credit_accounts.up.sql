-- Credit Accounts: tracks how much money each user has.
--
-- balance uses NUMERIC(12,6) — 12 digits total, 6 after the decimal.
-- This handles USD amounts like $0.000015 per token accurately.
-- Never use FLOAT for money — floating point math loses precision.
--
-- user_id is UNIQUE — each user has exactly one credit account.
CREATE TABLE credit_accounts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    balance    NUMERIC(12,6) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
