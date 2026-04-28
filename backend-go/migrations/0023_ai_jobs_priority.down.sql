ALTER TABLE ai_jobs DROP COLUMN IF EXISTS priority;
CREATE INDEX IF NOT EXISTS idx_ai_jobs_pending_created ON ai_jobs (created_at ASC) WHERE status = 'pending';
DROP INDEX IF EXISTS idx_ai_jobs_pending_priority;
