-- P18-C: node_templates
CREATE TABLE IF NOT EXISTS node_templates (
    id          TEXT        PRIMARY KEY,
    name        TEXT        NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    description TEXT        NOT NULL DEFAULT '',
    content     TEXT        NOT NULL,
    tags        TEXT[]      NOT NULL DEFAULT '{}',
    created_by  TEXT        NOT NULL DEFAULT '',
    is_system   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- P18-D: lake_snapshots
CREATE TABLE IF NOT EXISTS lake_snapshots (
    id          TEXT        PRIMARY KEY,
    lake_id     TEXT        NOT NULL,
    name        TEXT        NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    layout      JSONB       NOT NULL CHECK (octet_length(layout::text) < 65536),
    created_by  TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_lake_snapshots_lake ON lake_snapshots (lake_id, created_at DESC);

-- P18-B: node_shares
CREATE TABLE IF NOT EXISTS node_shares (
    id          TEXT        PRIMARY KEY,
    node_id     TEXT        NOT NULL,
    token       TEXT        NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ,
    revoked     BOOLEAN     NOT NULL DEFAULT FALSE,
    created_by  TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_node_shares_node  ON node_shares (node_id);
CREATE INDEX IF NOT EXISTS idx_node_shares_token ON node_shares (token);
