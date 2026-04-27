// Ripple Neo4j Schema · 约束与索引
// 来源：Python `backend/app/core/db.py` init_db
// 执行：cypher-shell -a $RIPPLE_NEO4J_URI -u $RIPPLE_NEO4J_USER -p $RIPPLE_NEO4J_PASS < neo4j_constraints.cypher
// 幂等：所有语句使用 IF NOT EXISTS

// ============================================================================
// 唯一性约束
// ============================================================================
CREATE CONSTRAINT lake_id_unique IF NOT EXISTS
    FOR (l:Lake) REQUIRE l.id IS UNIQUE;

CREATE INDEX lake_org IF NOT EXISTS
    FOR (l:Lake) ON (l.org_id);

CREATE CONSTRAINT node_id_unique IF NOT EXISTS
    FOR (n:Node) REQUIRE n.id IS UNIQUE;

CREATE CONSTRAINT iceberg_id_unique IF NOT EXISTS
    FOR (i:Iceberg) REQUIRE i.id IS UNIQUE;

// ============================================================================
// 索引（性能关键查询）
// ============================================================================

// 同湖节点 + 状态过滤（用于 weaver、列表）
CREATE INDEX node_lake_state IF NOT EXISTS
    FOR (n:Node) ON (n.lake_id, n.state);

// TTL 扫描（cron 清理 MIST/VAPOR 过期）
CREATE INDEX node_ttl IF NOT EXISTS
    FOR (n:Node) ON (n.ttl_at);

// owner 查询（迷雾区）
CREATE INDEX node_owner IF NOT EXISTS
    FOR (n:Node) ON (n.owner_id);

// Lake.is_public 过滤（公开发现）
CREATE INDEX lake_public IF NOT EXISTS
    FOR (l:Lake) ON (l.is_public);

// ============================================================================
// 验证：列出当前所有约束与索引
// ============================================================================
// 手动执行确认：
//   SHOW CONSTRAINTS;
//   SHOW INDEXES;
