# Ripple 后端 · Go 主线

> 状态：**已落地主线实现**，当前进入 **Phase 13 启动测试与发布准备**。  
> 目标：提供稳定的 HTTP / WS / 协作 / 检索 / 批量导入 / 组织能力，并完成联调、冒烟、回滚、灰度前验证。

---

## 当前能力

- 鉴权：注册 / 登录 / JWT / API Key
- 核心域：Lake / Node / Edge / 邀请 / 成员 / Org
- 协作：WebSocket 实时事件 + 独立 `yjs-bridge`
- 搜索：Neo4j fulltext
- 批量导入：CSV / JSON 1000 行上限
- 审计与可观测性：`/metrics`、审计日志、连接池指标
- 联调入口：`docker-compose.staging.yml` + `scripts/bootstrap-staging.ps1`

---

## 目录结构（当前）

```text
backend-go/
├── cmd/                 # server / migrate / migrate-seed / yjs-bridge / loadtest
├── internal/            # api / service / store / domain / config / llm / metrics ...
├── migrations/          # PG schema migrations
├── scripts/             # Neo4j constraints / smoke helpers
├── Dockerfile
├── docker-compose.dev.yml
├── Makefile
└── .env.example
```

---

## 本地与联调

### 1. 本地 Go 开发

```powershell
cd backend-go
copy .env.example .env
go run ./cmd/server
```

### 2. Phase 13 staging 联调

在仓库根目录执行：

```powershell
$env:PG_PASSWORD="replace-me"
$env:NEO4J_PASSWORD="replace-me"
$env:REDIS_PASSWORD="replace-me"
$env:JWT_SECRET="replace-me-with-at-least-32-chars"

./scripts/bootstrap-staging.ps1
./scripts/smoke/phase13-smoke.ps1 -Base http://127.0.0.1:18000
```

回收环境：

```powershell
./scripts/teardown-staging.ps1
```

---

## 常用命令

```powershell
cd backend-go

# 构建
go build ./...

# race 测试
$env:GOTOOLCHAIN="local"
go test ./internal/... -count=1 -race

# HTTP 集成测试
$env:RIPPLE_INTEGRATION="1"
go test ./internal/api/http/... -run TestIntegration -count=1 -v

# PG migrations
go run ./cmd/migrate
```

---

## 当前联调相关文档

- [../docs/快速上手.md](../docs/快速上手.md)
- [../docs/ops/Phase13-联调与回滚手册.md](../docs/ops/Phase13-联调与回滚手册.md)
- [../docs/launch/Phase13-准入清单.md](../docs/launch/Phase13-准入清单.md)
- [../docs/system-design/整体任务清单.md](../docs/system-design/整体任务清单.md)
