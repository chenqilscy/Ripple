-- Ripple PG · Lake Invites (M2-F2)
-- 用途：湖主/写者签发邀请 token，受邀者凭 token 加入。
-- 约束规约：docs/system-design/路线图-M2.md §F2

BEGIN;

CREATE TABLE IF NOT EXISTS lake_invites (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lake_id      VARCHAR(64)  NOT NULL,
    token        VARCHAR(64)  NOT NULL UNIQUE,         -- crypto/rand 32B base64url
    created_by   UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role         VARCHAR(16)  NOT NULL CHECK (role IN ('NAVIGATOR','PASSENGER','OBSERVER')),
    max_uses     INTEGER      NOT NULL CHECK (max_uses > 0),
    used_count   INTEGER      NOT NULL DEFAULT 0 CHECK (used_count >= 0),
    expires_at   TIMESTAMPTZ  NOT NULL,
    revoked_at   TIMESTAMPTZ  NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_lake_invites_lake_active
  ON lake_invites(lake_id) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_lake_invites_creator
  ON lake_invites(created_by);

COMMIT;
