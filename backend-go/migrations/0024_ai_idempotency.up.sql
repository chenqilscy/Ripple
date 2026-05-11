-- Phase 15.1: AI节点幂等性表
-- 用于防止重复触发AI任务，60分钟内相同key返回409

CREATE TABLE IF NOT EXISTS ai_node_idempotency_keys (
    lake_id UUID NOT NULL,
    idempotency_key VARCHAR(64) NOT NULL,
    ai_node_id UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (lake_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_ai_idempotency_created_at ON ai_node_idempotency_keys(created_at);

COMMENT ON TABLE ai_node_idempotency_keys IS 'AI节点幂等性key存储，60分钟后自动过期';
COMMENT ON COLUMN ai_node_idempotency_keys.lake_id IS '湖泊ID';
COMMENT ON COLUMN ai_node_idempotency_keys.idempotency_key IS '幂等性key（客户端生成）';
COMMENT ON COLUMN ai_node_idempotency_keys.ai_node_id IS '关联的AI节点ID';
COMMENT ON COLUMN ai_node_idempotency_keys.created_at IS '创建时间，用于60分钟TTL';
