-- Phase 15.2: ai_jobs 添加优先级字段，支持批量拉取时按优先级排序
ALTER TABLE ai_jobs ADD COLUMN IF NOT EXISTS priority INT NOT NULL DEFAULT 0;

-- 重建 pending 索引，纳入 priority 排序
DROP INDEX IF EXISTS idx_ai_jobs_pending_created;
CREATE INDEX idx_ai_jobs_pending_priority ON ai_jobs (priority DESC, created_at ASC) WHERE status = 'pending';
