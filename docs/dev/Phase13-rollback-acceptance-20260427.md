# Phase 13 回滚验收记录（2026-04-27）

## 目标

完成 Phase 13 最后一项灰度准入：应用级回滚与数据库迁移 down/up 演练，并验证恢复后 smoke 全绿。

## 环境

- Host: `fn.cky`
- Backend base: `http://fn.cky:18000`
- 回滚旧提交：`a9834d0`
- 恢复当前提交：`c6648a3`
- 端口覆盖：
  - Backend `18000`
  - Yjs `17790`
  - Frontend `14173`
  - PostgreSQL `25432`
  - Redis `26379`
  - Neo4j HTTP `17474`
  - Neo4j Bolt `17687`

## 执行说明

远端直接 `git clone https://github.com/chenqilscy/Ripple.git` 时出现 TLS 握手失败：

```text
fatal: unable to access 'https://github.com/chenqilscy/Ripple.git/': gnutls_handshake() failed: The TLS connection was non-properly terminated.
```

因此本次改用本地 `git archive` 生成干净源码包，通过 `scp` 上传远端执行：

```powershell
git archive --format=tar -o ripple-a9834d0.tar a9834d0
git archive --format=tar -o ripple-c6648a3.tar c6648a3
scp ripple-a9834d0.tar ripple-c6648a3.tar admin@fn.cky:/home/admin/
```

## 应用级回滚

### 1. 回退到上一稳定提交 `a9834d0`

- 停止并清理当前 staging 容器与卷；
- 解包 `ripple-a9834d0.tar`；
- 构建 backend 镜像；
- 使用固定 `COMPOSE_PROJECT_NAME=ripple` 拉起 staging；
- 复用已有 frontend 镜像，避免远端 nginx metadata 拉取卡住。

结果：容器全部启动，Postgres / Redis / Neo4j 为 healthy。

Smoke：

```text
OK phase13 smoke passed
lake_id=26777077-6423-4d22-9f32-d5b991fc5e93
node_id=3a75150c-ca89-4e1c-b7e9-2dbafdaabad5
org_id=b5f19414-9231-4931-b4a5-33b3ffe4f37d
```

### 2. 恢复到当前提交 `c6648a3`

- 停止并清理旧提交 staging 容器与卷；
- 解包 `ripple-c6648a3.tar`；
- 构建 backend 镜像；
- 重新拉起 staging。

结果：容器全部启动，Postgres / Redis / Neo4j 为 healthy。

Smoke：

```text
OK phase13 smoke passed
lake_id=3507b715-3a55-4d2a-9cce-5a2b34e5f4a0
node_id=a35e4a0e-596a-4b4c-8814-7af7bbd93111
org_id=425e373b-168b-4dc4-b0e7-ed439a6e2ec3
```

## 数据库迁移 down/up 演练

执行：

```text
/usr/local/go/bin/go run ./cmd/migrate down
/usr/local/go/bin/go run ./cmd/migrate
```

结果：

```text
==> migrations/0016_p18_features.down.sql
OK · down applied=1
SKIP migrations/0001_init.up.sql (already applied)
...
SKIP migrations/0015_node_tags.up.sql (already applied)
==> migrations/0016_p18_features.up.sql
OK · up applied=1
```

演练后 smoke：

```text
OK phase13 smoke passed
lake_id=f15ad595-64e1-4d50-bfb3-853edc8b4f88
node_id=21d90e1e-636e-42e3-8ecd-72da21c641e4
org_id=e5491186-2bce-41c2-b4bd-23fa52ed7388
```

## CI / GitHub 状态

GitHub CLI 未安装，改用 GitHub REST API 查询。最新 5 个 CI 均成功：

| Commit | Workflow | Status | Conclusion | URL |
|--------|----------|--------|------------|-----|
| `c6648a3` | CI | completed | success | https://github.com/chenqilscy/Ripple/actions/runs/25005027971 |
| `a9834d0` | CI | completed | success | https://github.com/chenqilscy/Ripple/actions/runs/25004852189 |
| `33f1b0a` | CI | completed | success | https://github.com/chenqilscy/Ripple/actions/runs/25004239142 |
| `a637ece` | CI | completed | success | https://github.com/chenqilscy/Ripple/actions/runs/25003761242 |
| `c1f3b47` | CI | completed | success | https://github.com/chenqilscy/Ripple/actions/runs/25003434878 |

## 遗留说明

尝试把 `/home/admin/Ripple` 原地替换为当前提交时，遇到 root-owned 文件权限：

```text
rm: cannot remove 'Ripple/backend/tests/test_embeddings.py': Permission denied
rm: cannot remove 'Ripple/backend/Dockerfile': Permission denied
rm: cannot remove 'Ripple/docker-compose.staging.yml': Permission denied
```

为保证服务恢复，本次最终从 `/home/admin/Ripple-rollback-current` 拉起 staging。该路径对应当前提交 `c6648a3`，主链路 smoke 已通过。后续若要恢复 canonical 路径，需要用具备权限的清理命令处理 `/home/admin/Ripple`。

## 结论

Phase 13 回滚验收通过：

- 应用级回滚到上一稳定提交后 smoke 全绿；
- 恢复到当前提交后 smoke 全绿；
- 数据库迁移 down/up 演练通过；
- 演练后 smoke 全绿；
- 最新 CI 为绿色。
