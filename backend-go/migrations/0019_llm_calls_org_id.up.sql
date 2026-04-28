-- Phase 15 · 0019: llm_calls 增加 org_id 列，支持组织维度用量聚合。
-- 历史记录 org_id 保持 NULL，前端展示为"未归属（迁移前）"。

ALTER TABLE llm_calls
    ADD COLUMN IF NOT EXISTS org_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_llm_calls_org_created
    ON llm_calls (org_id, created_at DESC)
    WHERE org_id IS NOT NULL;
