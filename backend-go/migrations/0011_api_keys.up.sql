-- P10-A：API Key 表
CREATE TABLE IF NOT EXISTS api_keys (
    id           TEXT PRIMARY KEY,
    owner_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    key_prefix   TEXT NOT NULL UNIQUE,    -- 16 hex chars (8 random bytes)，用于前缀快速查找
    key_hash     TEXT NOT NULL,           -- SHA-256(key_salt + raw_secret)，hex 编码
    key_salt     TEXT NOT NULL,           -- 32 hex chars (16 random bytes)
    scopes       TEXT[] NOT NULL DEFAULT '{}',
    last_used_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS api_keys_owner_idx  ON api_keys (owner_id);
CREATE INDEX IF NOT EXISTS api_keys_prefix_idx ON api_keys (key_prefix);
