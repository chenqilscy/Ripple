-- Phase 15 · 0020: Prompt 模板库。
-- scope: private = 仅创建者可用；org = 组织内共享（Phase 15.2 启用）。

CREATE TABLE IF NOT EXISTS prompt_templates (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    description TEXT        NOT NULL DEFAULT '',
    template    TEXT        NOT NULL CHECK (length(template) BETWEEN 1 AND 10000),
    scope       TEXT        NOT NULL DEFAULT 'private' CHECK (scope IN ('private', 'org')),
    org_id      TEXT        REFERENCES organizations(id) ON DELETE CASCADE,
    created_by  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_prompt_templates_owner
    ON prompt_templates (created_by, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_prompt_templates_org
    ON prompt_templates (org_id, created_at DESC)
    WHERE org_id IS NOT NULL;
