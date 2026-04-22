-- P8-A：Yjs 节点文档状态快照存储。
-- 独立表不修改现有 nodes 表，双写架构：
--   yjs-bridge 将 Y.Doc encoded state vector 定时写入此表；
--   Neo4j nodes.content 通过 outbox dispatcher 异步更新（最终一致）。
-- 大小限制：应用层拒绝 >1MB 的快照（见 handlers_doc_state.go）。
CREATE TABLE IF NOT EXISTS node_doc_states (
    node_id    TEXT        PRIMARY KEY,   -- 对应 Neo4j 节点 ID（UUID）
    state      BYTEA       NOT NULL,      -- Y.Doc encodeStateAsUpdate 二进制
    version    BIGINT      NOT NULL DEFAULT 1,  -- 乐观锁版本号
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE  node_doc_states           IS 'Yjs Y.Doc 状态快照（P8-A）';
COMMENT ON COLUMN node_doc_states.node_id   IS '对应 Neo4j 节点 ID';
COMMENT ON COLUMN node_doc_states.state     IS 'Y.Doc encodeStateAsUpdate 二进制，≤1MB';
COMMENT ON COLUMN node_doc_states.version   IS '乐观锁；PUT 时 version++ 防并发覆盖';
