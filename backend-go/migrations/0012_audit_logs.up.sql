-- P10-B：操作审计日志表
-- 保留最近 30 天；删除由 cmd/server 启动时触发一次清理（或 pg_cron）。
CREATE TABLE IF NOT EXISTS audit_logs (
    id            TEXT PRIMARY KEY,
    actor_id      TEXT NOT NULL,           -- 操作者 user_id（或 API key owner_id）
    action        TEXT NOT NULL,           -- 动作：node.create / node.delete / edge.create ...
    resource_type TEXT NOT NULL,           -- 资源类型：node / edge / lake_member / ...
    resource_id   TEXT NOT NULL,           -- 资源 ID
    detail        JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS audit_logs_resource_idx  ON audit_logs (resource_type, resource_id);
CREATE INDEX IF NOT EXISTS audit_logs_actor_idx     ON audit_logs (actor_id);
CREATE INDEX IF NOT EXISTS audit_logs_created_idx   ON audit_logs (created_at);
