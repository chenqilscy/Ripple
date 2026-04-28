-- P14.5: platform admin RBAC bootstrap table
CREATE TABLE IF NOT EXISTS platform_admins (
    user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'ADMIN' CHECK (role IN ('ADMIN', 'OWNER')),
    note       TEXT NOT NULL DEFAULT '',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_platform_admins_active ON platform_admins (created_at DESC) WHERE revoked_at IS NULL;
