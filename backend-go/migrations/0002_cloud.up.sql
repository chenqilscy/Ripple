-- 0002_cloud · 造云任务表（AI Weaver 持久化队列）
-- M1 暂不做分区；任务量 < 1k/day。

CREATE TABLE IF NOT EXISTS cloud_tasks (
  id            UUID PRIMARY KEY,
  owner_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  lake_id       UUID,                       -- 可空：未指定则节点暂留 MIST 不归湖
  prompt        TEXT NOT NULL,
  n             INT  NOT NULL CHECK (n > 0 AND n <= 10),
  node_type     VARCHAR(16) NOT NULL DEFAULT 'TEXT',
  status        VARCHAR(16) NOT NULL DEFAULT 'queued'
                CHECK (status IN ('queued','running','done','failed')),
  retry_count   INT NOT NULL DEFAULT 0,
  last_error    TEXT,
  result_node_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  started_at    TIMESTAMPTZ,
  completed_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS ix_cloud_tasks_status_created
  ON cloud_tasks(status, created_at)
  WHERE status IN ('queued','running');

CREATE INDEX IF NOT EXISTS ix_cloud_tasks_owner_created
  ON cloud_tasks(owner_id, created_at DESC);
