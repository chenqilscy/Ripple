-- Phase 15 · 0022: 组织套餐订阅。
-- 部分唯一索引确保一个 org 同时只有一个 active 订阅（允许历史记录保留）。

CREATE TABLE IF NOT EXISTS org_subscriptions (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          TEXT        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    plan_id         TEXT        NOT NULL CHECK (plan_id IN ('free', 'pro', 'team')),
    status          TEXT        NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active', 'expired', 'cancelled')),
    billing_cycle   TEXT        NOT NULL DEFAULT 'monthly'
                        CHECK (billing_cycle IN ('monthly', 'annual')),
    stub            BOOLEAN     NOT NULL DEFAULT FALSE,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 每个组织同时最多一个 active 订阅（反方审查意见 #3）
CREATE UNIQUE INDEX IF NOT EXISTS org_subscriptions_active_idx
    ON org_subscriptions (org_id)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_org_subscriptions_org
    ON org_subscriptions (org_id, created_at DESC);
