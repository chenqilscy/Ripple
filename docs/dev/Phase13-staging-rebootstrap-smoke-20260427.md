# Phase 13 Staging Rebootstrap + Smoke（2026-04-27）

## 目标

在完成实际回收后，重新拉起远端 staging，并执行 Phase 13 smoke 验收。

## 环境

- Host: `fn.cky`
- Directory: `/home/admin/Ripple`
- Compose file: `docker-compose.staging.yml`
- Backend base: `http://fn.cky:18000`

## 过程记录

### 1. 首次 rebootstrap

直接使用 `docker compose up -d --build` 时，frontend build 卡在基础镜像元数据解析阶段；改用已缓存镜像 `up -d` 继续。

### 2. 端口冲突

远端存在共享/历史容器占用默认端口：

- `redis16379` 占用 `16379`
- 远端共享 Postgres 占用 `15432`
- 历史前端测试容器 `ripple-frontend-test-run` 占用 `14173`

处理方式：

- 不停止共享 Redis / PostgreSQL；
- 使用 `STAGING_REDIS_PORT=26379`；
- 使用 `STAGING_PG_PORT=25432`；
- 清理无状态历史前端测试容器 `ripple-frontend-test-run`；
- 后续代码修复：`docker-compose.staging.yml` 的 backend / yjs / frontend 端口也改为环境变量可覆盖，`bootstrap-staging.ps1` 同步读取端口覆盖。

### 3. 最终容器状态

```text
ripple-staging-postgres     Up (healthy)   0.0.0.0:25432->5432/tcp
ripple-staging-redis        Up (healthy)   0.0.0.0:26379->6379/tcp
ripple-staging-frontend     Up             0.0.0.0:14173->80/tcp
ripple-staging-yjs-bridge   Up             0.0.0.0:17790->7790/tcp
ripple-staging-backend      Up             0.0.0.0:18000->8000/tcp
ripple-staging-neo4j        Up (healthy)   0.0.0.0:17474->7474/tcp, 0.0.0.0:17687->7687/tcp
```

### 4. Smoke

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke\phase13-smoke.ps1 -Base http://fn.cky:18000
```

结果：

```text
== health ==
== register ==
== login ==
== register invited user ==
== create lake ==
== create node ==
== search ==
== batch import ==
== api key ==
== org create + invite by email ==
== audit logs ==
OK phase13 smoke passed
lake_id=b79ceba4-8a02-4670-b388-8e7f6182e85e node_id=fd740c67-9f72-43e9-957e-14b0b445c709 org_id=80cb1b65-0709-44d2-8986-916ae2f64a15
```

## 结论

Staging 实际回收后的重新拉起与 smoke 验收通过。当前远端 staging 已重新可用。
