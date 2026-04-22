-- M3-S2: perma_nodes 索引表
-- 凝结后的"晶体节点"在 Neo4j 中是 :Perma 节点；本表存 PG 索引 + 来源 mist 列表 + 摘要 + 来源数量
-- 设计目的：
--   1. 不在 Neo4j 中冗余存大文本（content/summary 都在 PG，Neo4j 只存引用 ID）；
--   2. 支持按 lake/owner 检索 perma 节点列表（Neo4j 适合关系查，PG 适合 OLTP 列表）；
--   3. source_node_ids 用 UUID[] 数组，便于反查"这个 mist 被凝结过几次"。

CREATE TABLE IF NOT EXISTS perma_nodes (
    id UUID PRIMARY KEY,
    lake_id UUID NOT NULL,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    -- 来源 mist 节点 ID 列表（保留原始顺序）
    source_node_ids UUID[] NOT NULL DEFAULT '{}'::UUID[],
    -- LLM 凝结所用 provider（用于审计 + 复盘 cost）
    llm_provider VARCHAR(64) NOT NULL DEFAULT '',
    llm_cost_tokens BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- lake 内列表
CREATE INDEX IF NOT EXISTS idx_perma_nodes_lake ON perma_nodes(lake_id, created_at DESC);
-- owner 内列表（用户跨湖回看）
CREATE INDEX IF NOT EXISTS idx_perma_nodes_owner ON perma_nodes(owner_id, created_at DESC);
-- 反查：某个 mist 节点被哪些 perma 引用
CREATE INDEX IF NOT EXISTS idx_perma_nodes_source ON perma_nodes USING GIN(source_node_ids);

CREATE TRIGGER trg_perma_nodes_updated_at
BEFORE UPDATE ON perma_nodes
FOR EACH ROW EXECUTE FUNCTION trg_set_updated_at();
