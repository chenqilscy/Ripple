-- Phase 15.1: 回滚AI节点幂等性表

DROP INDEX IF EXISTS idx_ai_idempotency_created_at;
DROP TABLE IF EXISTS ai_node_idempotency_keys;