# Phase 14 Staging Smoke 数据清理验收报告

- **执行时间**：2026-04-28
- **执行人**：Autopilot（Round 4 / T30）
- **目标环境**：fn.cky / ripple-staging-postgres + ripple-staging-neo4j
- **脚本**：`scripts/smoke/staging-cleanup-smoke.ps1`

## 1. 候选数据（Dry-run）

PG `users` 表命中 3 条 `phase13+` 前缀 smoke 用户：

| user_id | email |
|---------|-------|
| `b758eca5-9b3c-48fc-a7a3-ea7ca41860fc` | `phase13+1777312777@ripple.local` |
| `e1f52496-863e-4eb4-9def-a2f487139710` | `phase13+1777312828@ripple.local` |
| `92bfd896-ac3d-42c8-b175-6f0337eb18ff` | `phase13+1777316275@ripple.local` |

Neo4j `Lake` 节点匹配 `ws-smoke-/ws-curl-/ws-origin-/phase13-smoke-` 前缀：**0 条**。

## 2. 关联数据预扫

| 表 | 命中行数 | FK ondelete |
|----|---------|-------------|
| graylist_entries (created_by) | 0 | RESTRICT |
| audit_events (actor_id) | 0 | NO ACTION |
| node_revisions (editor_id) | 3 | RESTRICT |
| organizations (owner_id) | 3 | RESTRICT |
| platform_admins | 0 | CASCADE/SET NULL |
| perma_nodes / spaces / lake_invites | 0 | CASCADE |

## 3. 脚本修复

- **#1** PG 表名：`graylist_emails` → `graylist_entries`（schema 与脚本默认值不一致）。
- **#2** RESTRICT 拦路虎：在 `DELETE users` 之前显式删除 `node_revisions` 与 `organizations` 中由 smoke 用户产生的行。
- **#3** 事务包裹：`BEGIN ... COMMIT;` 保证任何子句失败即回滚。

## 4. -Apply 执行结果

```
BEGIN
DELETE 0   -- graylist_entries (email 前缀，无命中)
DELETE 0   -- graylist_entries (created_by 子查询)
DELETE 0   -- audit_events
DELETE 3   -- node_revisions
DELETE 3   -- organizations
DELETE 3   -- users
COMMIT

Neo4j: deleted 0
```

## 5. 验证

```sql
SELECT id, email FROM users
WHERE email LIKE 'phase13+%'
   OR email LIKE 'ws_smoke_%'
   OR email LIKE 'phase14_smoke_%';
-- (0 rows)
```

清理彻底，未触发任何 FK 错误，未影响真实用户数据。

## 6. 后续建议

- 将本脚本纳入 `scripts/smoke/phase14-6-acceptance.ps1` 的 `-IncludeStagingCleanup` 流程默认 dry-run，`-StagingCleanupApply` 显式触发真实清理。
- Phase 15 起 staging smoke 命名前缀建议统一为 `phase15+` / `ws-phase15-` 以便后续按版本批量清理。
- 若引入 lakes 表（当前湖只在 Neo4j），需重新评估 FK 链路。
