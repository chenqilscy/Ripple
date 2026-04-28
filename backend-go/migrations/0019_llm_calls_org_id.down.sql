-- Phase 15 · 0019 rollback
DROP INDEX IF EXISTS idx_llm_calls_org_created;
ALTER TABLE llm_calls DROP COLUMN IF EXISTS org_id;
