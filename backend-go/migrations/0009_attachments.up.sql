-- M4-B：节点附件元数据（本地 FS 存储）
-- 注意：nodes/lakes 实体在 Neo4j，本表的 node_id 仅做软关联，无 FK。
CREATE TABLE IF NOT EXISTS attachments (
    id           UUID PRIMARY KEY,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    node_id      VARCHAR(64), -- 软关联 Neo4j Node.id，可空
    mime         TEXT NOT NULL,
    size_bytes   BIGINT NOT NULL,
    file_path    TEXT NOT NULL, -- 相对 UploadDir 的相对路径
    sha256       TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_attachments_user ON attachments(user_id);
CREATE INDEX IF NOT EXISTS idx_attachments_node ON attachments(node_id) WHERE node_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_attachments_sha ON attachments(user_id, sha256);
