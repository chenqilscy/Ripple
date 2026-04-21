# Ripple 后端 · Go 实现

> **状态**：骨架预览（占位）。等待老板提供中间件信息后启动正式实现。
> **参考**：`docs/system-design/M1-Python-五轮审查报告.md` §Go 重写 12 项必做改进

## 目录结构（计划）

```
backend-go/
├── cmd/
│   └── server/
│       └── main.go              # 入口（HTTP + WS + lifecycle）
├── internal/
│   ├── api/                     # transport 层
│   │   ├── http/
│   │   │   ├── auth_handler.go
│   │   │   ├── lake_handler.go
│   │   │   ├── node_handler.go
│   │   │   ├── cloud_handler.go
│   │   │   ├── middleware/
│   │   │   │   ├── auth.go
│   │   │   │   ├── ratelimit.go
│   │   │   │   ├── request_id.go
│   │   │   │   └── recovery.go
│   │   │   └── router.go
│   │   └── ws/
│   │       └── lake_socket.go
│   ├── service/                 # 业务编排
│   │   ├── auth/
│   │   ├── lake/
│   │   ├── node/                # 含状态机
│   │   ├── cloud/               # 迷雾凝露
│   │   ├── share/
│   │   └── audit/
│   ├── domain/                  # 领域模型（纯 struct + 不变量方法）
│   │   ├── user.go
│   │   ├── lake.go
│   │   ├── node.go              # State 枚举 + 状态转换合法性
│   │   ├── membership.go
│   │   └── errors.go
│   ├── store/                   # 持久化适配器
│   │   ├── pg/
│   │   │   ├── user_repo.go
│   │   │   ├── membership_repo.go
│   │   │   ├── audit_repo.go
│   │   │   ├── outbox_repo.go
│   │   │   └── conn.go
│   │   ├── neo4j/
│   │   │   ├── lake_repo.go
│   │   │   ├── node_repo.go
│   │   │   └── conn.go
│   │   └── redis/
│   │       └── conn.go
│   ├── ai/
│   │   ├── embeddings.go        # 本地 hash embedding（M1 同口径）
│   │   └── weaver.go
│   ├── ws/                      # 广播层（接口 + 实现）
│   │   ├── broker.go            # interface Broker
│   │   ├── memory_broker.go     # 单进程实现
│   │   └── redis_broker.go      # M2：Redis Pub/Sub
│   ├── worker/                  # 后台任务
│   │   ├── outbox_dispatcher.go
│   │   └── weaver_consumer.go
│   ├── config/
│   │   └── config.go            # envconfig
│   └── platform/
│       ├── logger.go            # zerolog
│       ├── jwt.go
│       ├── password.go          # bcrypt + UTF-8 安全截断
│       └── ids.go               # uuid 生成
├── migrations/                  # PG schema (sql files for atlas/migrate)
│   ├── 0001_init.up.sql
│   └── 0001_init.down.sql
├── api/
│   └── openapi.yaml             # 单一契约源
├── scripts/
│   ├── neo4j_constraints.cypher
│   └── seed_dev.sh
├── tests/
│   ├── unit/                    # 纯函数测试
│   ├── integration/             # 真 PG/Neo4j（docker）
│   └── e2e/                     # HTTP 黑盒
├── .env.example
├── Dockerfile
├── docker-compose.dev.yml
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## 设计要点（呼应五轮审查报告）

1. **三层模型**：`domain.Node` ↔ `dto.NodeResponse` ↔ `store.NodeRow`，禁止跨层泄漏
2. **Repository 接口**：`service` 依赖接口 `pg.UserRepo`，不依赖 `*pgx.Conn`
3. **Outbox Saga**：Lake/Node 创建跨库时用 outbox 解耦
4. **Broker 接口**：`ws.Broker` 抽象，本地 + Redis 双实现，启动时注入
5. **Worker 隔离**：Weaver 不在请求路径上跑，消费 outbox.node.condensed 事件
6. **强类型 Principal**：`type Principal struct { UserID uuid.UUID; Email string }`
7. **Permission 注释**：service 函数顶部 `// Permission: NAVIGATOR+`

## 约束遵循矩阵

| 约束规约条款 | Go 实现位置 |
|------------|------------|
| §3.1 bcrypt cost ≥ 12 | `platform/password.go` |
| §3.3 数据隔离 | `service/*` 必受 `Principal` 参数 |
| §3.4 Cypher 参数化 | `store/neo4j/*` 全用 `tx.Run(query, params)` |
| §5 可观测性 X-Request-ID | `api/http/middleware/request_id.go` |
| §7 技术栈锁定 | `go.mod` 不允许引入禁用项 |
| §8.5 反成瘾设计 | API 层不提供"在线人数 / 排行榜" 端点 |

## 启动条件（待老板提供）

```bash
# .env.example 中需填充的真实值（本仓库内仅占位）
RIPPLE_PG_DSN=postgres://USER:PASS@HOST:5432/ripple?sslmode=require
RIPPLE_NEO4J_URI=bolt://HOST:7687
RIPPLE_NEO4J_USER=neo4j
RIPPLE_NEO4J_PASS=...
RIPPLE_REDIS_ADDR=HOST:6379
RIPPLE_REDIS_PASS=...
RIPPLE_S3_ENDPOINT=...
RIPPLE_S3_BUCKET=...
RIPPLE_JWT_SECRET=...   # ≥ 32 字节随机
```

## 何时开工

✅ 老板提供以上 6 项配置任意可访问的开发环境
✅ 决策者最终批准本骨架结构
✅ 接口设计师产出 `api/openapi.yaml` 完整版

✋ 在以上三项之前，本目录仅保持骨架文档状态。
