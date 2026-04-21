-- Ripple PG Schema · Initial Migration
-- 来源：Python `backend/app/models/` 推导
-- 工具：兼容 golang-migrate / atlas / 手动 psql
-- 约束规约：docs/system-design/系统约束规约.md §2.1

BEGIN;

-- ============================================================================
-- 用户表
-- ============================================================================
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    display_name    VARCHAR(64)  NOT NULL,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email_active ON users(email) WHERE is_active = TRUE;

-- ============================================================================
-- 湖泊成员关系（Lake 实体本身在 Neo4j）
-- ============================================================================
CREATE TABLE IF NOT EXISTS lake_memberships (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    lake_id     VARCHAR(64)  NOT NULL,
    role        VARCHAR(16)  NOT NULL CHECK (role IN ('OWNER','NAVIGATOR','PASSENGER','OBSERVER')),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, lake_id)
);

CREATE INDEX IF NOT EXISTS idx_lake_memberships_lake ON lake_memberships(lake_id);
CREATE INDEX IF NOT EXISTS idx_lake_memberships_user ON lake_memberships(user_id);

-- ============================================================================
-- 审计日志（90 天保留 → 后台清理 cron）
-- ============================================================================
CREATE TABLE IF NOT EXISTS audit_events (
    id              BIGSERIAL    PRIMARY KEY,
    actor_id        UUID         NOT NULL REFERENCES users(id),
    action          VARCHAR(64)  NOT NULL,
    resource_type   VARCHAR(32)  NOT NULL,
    resource_id     VARCHAR(64)  NOT NULL,
    lake_id         VARCHAR(64),
    metadata        JSONB,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_actor_time ON audit_events(actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_events(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_lake_time ON audit_events(lake_id, created_at DESC) WHERE lake_id IS NOT NULL;

-- ============================================================================
-- Outbox 事件（跨库 Saga + Worker 投递）
-- ============================================================================
CREATE TABLE IF NOT EXISTS outbox_events (
    id              BIGSERIAL   PRIMARY KEY,
    event_type      VARCHAR(64) NOT NULL,
    payload         JSONB       NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','processing','done','failed')),
    retry_count     INT         NOT NULL DEFAULT 0,
    last_error      TEXT,
    processed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending
    ON outbox_events(status, created_at)
    WHERE status IN ('pending', 'failed');
CREATE INDEX IF NOT EXISTS idx_outbox_event_type ON outbox_events(event_type);

-- ============================================================================
-- updated_at 自动维护触发器
-- ============================================================================
CREATE OR REPLACE FUNCTION trg_set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS users_updated_at ON users;
CREATE TRIGGER users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION trg_set_updated_at();

DROP TRIGGER IF EXISTS lake_memberships_updated_at ON lake_memberships;
CREATE TRIGGER lake_memberships_updated_at BEFORE UPDATE ON lake_memberships
    FOR EACH ROW EXECUTE FUNCTION trg_set_updated_at();

DROP TRIGGER IF EXISTS outbox_events_updated_at ON outbox_events;
CREATE TRIGGER outbox_events_updated_at BEFORE UPDATE ON outbox_events
    FOR EACH ROW EXECUTE FUNCTION trg_set_updated_at();

COMMIT;
