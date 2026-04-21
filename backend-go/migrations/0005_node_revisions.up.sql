-- M2-F3：节点编辑历史（事件溯源）。
-- 每次 content 变更写入一条 revision；rev_number 在单节点范围内单调递增（从 1 开始）。
-- 节点实体存于 Neo4j，但 revision 作为审计/回溯数据存于 PG（便于范围查询与分页）。

CREATE TABLE node_revisions (
    id           TEXT PRIMARY KEY,
    node_id      TEXT NOT NULL,
    rev_number   INT  NOT NULL CHECK (rev_number > 0),
    content      TEXT NOT NULL,
    title        TEXT NOT NULL DEFAULT '',
    editor_id    UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    edit_reason  TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(node_id, rev_number)
);

-- 按节点时间倒序检索（最常用的"时间轴"查询）。
CREATE INDEX idx_node_revisions_node_time
    ON node_revisions(node_id, created_at DESC);

-- 按编辑者审计。
CREATE INDEX idx_node_revisions_editor
    ON node_revisions(editor_id);
