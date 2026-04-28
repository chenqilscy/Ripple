-- Phase 15 · 0021: AI 节点异步任务队列。
-- 部分唯一索引确保同一节点同时最多只有一个 pending/processing 任务。

CREATE TABLE IF NOT EXISTS ai_jobs (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id             TEXT        NOT NULL,
    lake_id             TEXT        NOT NULL,
    prompt_template_id  UUID        REFERENCES prompt_templates(id) ON DELETE SET NULL,
    status              TEXT        NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    progress_pct        INT         NOT NULL DEFAULT 0 CHECK (progress_pct BETWEEN 0 AND 100),
    input_node_ids      TEXT[]      NOT NULL DEFAULT '{}',
    override_vars       JSONB       NOT NULL DEFAULT '{}',
    started_at          TIMESTAMPTZ,
    finished_at         TIMESTAMPTZ,
    error               TEXT        NOT NULL DEFAULT '',
    created_by          UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 并发安全：同节点只能有一个进行中任务（反方审查意见 #2）
CREATE UNIQUE INDEX IF NOT EXISTS ai_jobs_node_active_idx
    ON ai_jobs (node_id)
    WHERE status IN ('pending', 'processing');

-- Worker 拉取待处理任务的索引
CREATE INDEX IF NOT EXISTS idx_ai_jobs_pending_created
    ON ai_jobs (created_at ASC)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_ai_jobs_node_id
    ON ai_jobs (node_id, created_at DESC);
