-- P14-A: organization quotas and usage snapshots
CREATE TABLE IF NOT EXISTS org_quotas (
    org_id          TEXT        PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    max_members     BIGINT      NOT NULL DEFAULT 20 CHECK (max_members >= 1),
    max_lakes       BIGINT      NOT NULL DEFAULT 50 CHECK (max_lakes >= 0),
    max_nodes       BIGINT      NOT NULL DEFAULT 10000 CHECK (max_nodes >= 0),
    max_attachments BIGINT      NOT NULL DEFAULT 1000 CHECK (max_attachments >= 0),
    max_api_keys    BIGINT      NOT NULL DEFAULT 10 CHECK (max_api_keys >= 0),
    max_storage_mb  BIGINT      NOT NULL DEFAULT 1024 CHECK (max_storage_mb >= 0),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS org_usage_snapshots (
    id               TEXT        PRIMARY KEY,
    org_id           TEXT        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    members_used     BIGINT      NOT NULL DEFAULT 0 CHECK (members_used >= 0),
    lakes_used       BIGINT      NOT NULL DEFAULT 0 CHECK (lakes_used >= 0),
    nodes_used       BIGINT      NOT NULL DEFAULT 0 CHECK (nodes_used >= 0),
    attachments_used BIGINT      NOT NULL DEFAULT 0 CHECK (attachments_used >= 0),
    api_keys_used    BIGINT      NOT NULL DEFAULT 0 CHECK (api_keys_used >= 0),
    storage_mb_used  BIGINT      NOT NULL DEFAULT 0 CHECK (storage_mb_used >= 0),
    captured_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_org_usage_snapshots_org ON org_usage_snapshots (org_id, captured_at DESC);
