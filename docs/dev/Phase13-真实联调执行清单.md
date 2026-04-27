# Phase 13 真实联调执行清单

> 用途：在有 Docker 的联调机上，执行第一次真实 `bootstrap-staging` 并把结果回填。

---

## 1. 执行前确认

1. 机器已安装 Docker。
2. 端口 `14173`、`15432`、`16379`、`17474`、`17687`、`17790`、`18000` 未被占用。
3. 当前工作区已拉到 `main` 最新提交。
4. PowerShell 会话已注入：`PG_PASSWORD`、`NEO4J_PASSWORD`、`REDIS_PASSWORD`、`JWT_SECRET`。

## 2. 执行步骤

```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass

git pull --ff-only origin main

./scripts/bootstrap-staging.ps1
./scripts/smoke/phase13-smoke.ps1 -Base http://127.0.0.1:18000
./scripts/drill-staging.ps1 -Scenario redis -DurationSeconds 15
./scripts/drill-staging.ps1 -Scenario neo4j -DurationSeconds 15
./scripts/drill-staging.ps1 -Scenario yjs-bridge -DurationSeconds 15
```

## 3. 回填项目

| 项目 | 结果 |
|------|------|
| bootstrap 是否成功 |  |
| smoke 是否全绿 |  |
| redis 演练结果 |  |
| neo4j 演练结果 |  |
| yjs-bridge 演练结果 |  |
| 发现的问题 |  |
| 对应 commit |  |

## 4. 回填位置

1. `docs/system-design/整体任务清单.md`
2. `docs/launch/Phase13-准入清单.md`
3. 如有故障，补一条修复任务或技术债条目