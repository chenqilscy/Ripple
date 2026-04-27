# Phase 13 Staging Teardown Acceptance（2026-04-27）

## 目标

执行一次真实 staging 回收窗口，验证 `docker compose down --remove-orphans -v` 能清理 Phase 13 staging 容器、网络与卷。

## 环境

- Host: `fn.cky`
- Directory: `/home/admin/Ripple`
- Compose file: `docker-compose.staging.yml`
- Operator: `admin`

## 执行前状态

```text
ripple-staging-yjs-bridge   Up 9 hours
ripple-staging-backend      Up 9 hours
ripple-staging-postgres     Up 9 hours (healthy)
ripple-staging-neo4j        Up 9 hours (healthy)
ripple-staging-redis        Up 9 hours (healthy)
```

执行前卷：

```text
ripple_staging_neo4j_data
ripple_staging_pg_data
ripple_staging_redis_data
ripple_staging_upload_data
```

## 执行命令

远端当前目录没有 `.env`，因此本次只为 compose 插值校验注入 dummy 变量；`down` 仅按 compose project/container label 清理资源，不依赖这些值连接服务。

```bash
PG_PASSWORD=dummy \
NEO4J_PASSWORD=dummy \
REDIS_PASSWORD=dummy \
JWT_SECRET=dummy-dummy-dummy-dummy-dummy-dummy-32 \
docker compose -f docker-compose.staging.yml down --remove-orphans -v
```

## 结果

- `ripple-staging-frontend` removed
- `ripple-staging-yjs-bridge` removed
- `ripple-staging-backend` removed
- `ripple-staging-migrate` removed
- `ripple-staging-neo4j-init` removed
- `ripple-staging-redis` removed
- `ripple-staging-postgres` removed
- `ripple-staging-neo4j` removed
- `ripple_staging_pg_data` removed
- `ripple_staging_neo4j_data` removed
- `ripple_staging_upload_data` removed
- `ripple_staging_redis_data` removed
- `ripple_default` network removed

执行后检查：

```text
== containers after ==
== volumes after ==
```

## 结论

Staging 实际回收验收通过。当前远端 staging 已处于回收后状态；如需继续联调，需要重新执行 bootstrap。
