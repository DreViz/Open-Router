-- Users table: stores account info.
-- Password is NEVER stored as plaintext — only the bcrypt hash.
-- bcrypt is a one-way hash: you can verify a password against it, but never reverse it.
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);
