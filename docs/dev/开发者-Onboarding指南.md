# 开发者 Onboarding 指南

**版本：** v0.2  
**目标读者：** 新加入青萍 (Ripple) 的 Go / 前端 / 联调工程师  
**预期时长：** 30 分钟完成一次 staging 拉起与冒烟

---

## 一、当前项目状态

Ripple 当前主线已切换到 **Go + React + Neo4j + PostgreSQL + Redis**，M1 ~ M12 已完成，正在推进 **Phase 13 启动测试与发布准备**。

你加入后的第一目标不是“理解历史 Python 原型”，而是：

1. 能拉起 staging 环境；
2. 能跑通 `phase13-smoke.ps1`；
3. 知道联调、回滚、故障演练入口在哪。

---

## 二、代码仓结构（当前）

```
Ripple/
├── AGENTS.md
├── docker-compose.yml
├── docker-compose.staging.yml          # Phase 13 联调环境
├── README.md
├── backend-go/                         # Go 后端主线
│   ├── cmd/
│   ├── internal/
│   ├── migrations/
│   ├── scripts/
│   └── .env.example
├── frontend/                           # React + Vite 前端
├── scripts/
│   ├── bootstrap-staging.ps1
│   ├── teardown-staging.ps1
│   ├── drill-staging.ps1
│   └── smoke/
└── docs/
    ├── README.md
    ├── 快速上手.md
    ├── launch/
    ├── ops/
    ├── dev/
    └── system-design/
```

---

## 三、第一天推荐路径

1. 先读 [../README.md](../README.md) 了解总入口。
2. 再读 [../快速上手.md](../快速上手.md) 走一遍当前启动路径。
3. 然后读 [../ops/Phase13-联调与回滚手册.md](../ops/Phase13-联调与回滚手册.md)。
4. 最后看 [../launch/Phase13-准入清单.md](../launch/Phase13-准入清单.md) 知道“什么算可发布”。

---

## 四、联调环境启动

### 4.1 前置要求

| 工具 | 版本 | 用途 |
|------|------|------|
| Go | 1.23+ | 后端构建 / 测试 |
| Node.js | 20+ | 前端构建 |
| Docker + Compose | 最新 | Phase 13 staging |
| PowerShell | 7+ 或 Windows PowerShell | 启动脚本 |

### 4.2 一键拉起

```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass

$env:PG_PASSWORD="replace-me"
$env:NEO4J_PASSWORD="replace-me"
$env:REDIS_PASSWORD="replace-me"
$env:JWT_SECRET="replace-me-with-at-least-32-chars"

./scripts/bootstrap-staging.ps1
```

默认地址：

- 前端：`http://127.0.0.1:14173`
- 后端：`http://127.0.0.1:18000`
- Yjs：`ws://127.0.0.1:17790/yjs`

### 4.3 回收环境

```powershell
./scripts/teardown-staging.ps1
```

### 4.4 故障演练

```powershell
./scripts/drill-staging.ps1 -Scenario redis -DurationSeconds 15
```

---

## 五、最小验证命令

```powershell
# 后端 race 测试
cd backend-go
$env:GOTOOLCHAIN="local"
go test ./internal/... -count=1 -race

# HTTP 集成测试
$env:RIPPLE_INTEGRATION="1"
go test ./internal/api/http/... -run TestIntegration -count=1 -v

# 前端构建
cd ..\frontend
npm ci
npm run build

# staging 冒烟
cd ..
./scripts/smoke/phase13-smoke.ps1 -Base http://127.0.0.1:18000
```

---

## 六、开发纪律

1. 改接口前先看 `api/openapi.yaml` 与系统设计文档。
2. 改节点状态、权限、检索、协作前，先确认对应设计文档已更新。
3. 任何收口前都遵守 [../../AGENTS.md](../../AGENTS.md) 和 `copilot.instructions.md` 的流程约束。
4. Phase 13 期间优先保证联调、稳定性、回滚，而不是继续堆新功能。

---

## 七、常见问题

| 现象 | 排查 |
|------|------|
| `docker` 命令不存在 | 当前机器不适合做 Phase 13 联调，改用联调机 |
| `healthz` 不绿 | 看 `docker compose -f docker-compose.staging.yml logs backend` |
| Neo4j 可连但搜索无结果 | 检查 `neo4j-init` 是否执行，fulltext index 是否存在 |
| 冒烟脚本失败 | 先确认 `/healthz`、再确认迁移、最后看后端日志 |

---

## 八、继续阅读

1. [../快速上手.md](../快速上手.md)
2. [../ops/Phase13-联调与回滚手册.md](../ops/Phase13-联调与回滚手册.md)
3. [../launch/Phase13-准入清单.md](../launch/Phase13-准入清单.md)
4. [../system-design/整体任务清单.md](../system-design/整体任务清单.md)
5. [../../AGENTS.md](../../AGENTS.md)
