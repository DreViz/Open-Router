-- API Keys table: stores hashed keys for programmatic access.
--
-- Why hash API keys?
--   Same reason as passwords — if your database is breached, the attacker
--   can't use the keys because they only have the hashes.
--   The real key is shown to the user ONCE when created, then discarded.
--
-- key_hash:   SHA256 of the full API key (used for lookup on each request)
-- key_prefix: First 12 characters of the key (shown in dashboard so user
--             can identify which key is which: "sk-vh-a3b1c5d7...")
CREATE TABLE api_keys (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    key_hash   TEXT UNIQUE NOT NULL,
    key_prefix TEXT NOT NULL,
    is_active  BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for fast lookups: on every request we search by key_hash.
-- Without an index, PostgreSQL scans every row (slow with millions of keys).
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
