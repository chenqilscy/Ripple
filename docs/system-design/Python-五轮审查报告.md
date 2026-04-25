# M1 Python 实现 · 五轮代码审查报告

> **审查目的**：作为 Go 重写的设计参考资料，提取每一处可改进点。
> **审查对象**：commit `5d3b3e8` (`backend/` 全部源码)
> **审查者**：反方·审查员（AI 角色）
> **审查时间**：2025-Q4
> **审查标准**：`AGENTS.md` §五轮代码审查 + `docs/system-design/系统约束规约.md`

---

## 第 1 轮 · 逻辑正确性

### ✅ 通过项

- `node_service.create_drop_node`：Cypher 创建 + BELONGS_TO 边逻辑正确
- `lake_service.assert_access`：role rank 比较实现符合权限矩阵
- `weaver.weave_for_node`：top-K 截断 + cosine 阈值过滤逻辑正确
- 状态机 `evaporate_node`：仅 DROP/FROZEN 可蒸发，VAPOR 二次蒸发会抛错（隐式 0 行更新）

### ⚠️ 问题

| ID | 文件 | 问题 | 严重度 | Go 重写时修正 |
|----|------|------|-------|--------------|
| L1-01 | `node_service.evaporate_node` | 状态校验仅靠 Cypher WHERE，未返回明确错误码 | 中 | 显式 `if !found { return ErrInvalidState }` |
| L1-02 | `node_service.condense_mists` | 批量更新无事务，部分失败时数据不一致 | 高 | Go 用 `tx, _ := session.BeginTransaction()` 包裹 |
| L1-03 | `weaver.weave_for_node` | 同湖查询返回 `result.records` 但未限定语言/类型，长内容嵌入耗时 | 中 | Go 加 `LIMIT 200` + 内容截断到 2KB |
| L1-04 | `auth_service.register_user` | `IntegrityError` → 409 但未区分是 email 重复还是其他约束 | 低 | Go 显式查询 `errors.Is(err, ErrUniqueViolation)` 并检查列名 |
| L1-05 | `cloud.condense` Pydantic body | `target_lake_id` 未校验权限，直接进 service | 高 | Go 在 handler 层先 `assertAccess(min: PASSENGER)` |

### 🔴 阻塞性 Bug（必须 Go 修复）

无（M1 已通过 9/9 单测）

---

## 第 2 轮 · 边界条件与异常处理

### ⚠️ 问题

| ID | 文件 | 问题 | 严重度 | Go 重写时修正 |
|----|------|------|-------|--------------|
| L2-01 | `password.hash_password` | 截断 72 字节是字节级，UTF-8 中文可能截断到字符中间 → 后续解码异常 | 高 | Go 用 `len([]byte(s))` 检查并向前回退到字符边界 |
| L2-02 | `embeddings.embed` | 空字符串输入 → `''.encode()` → 全零向量，cosine = NaN | 高 | Go 加 `if text == "" { return zeroVec }` 并跳过织网 |
| L2-03 | `node_service.create_mist_node` | TTL 用 `datetime.utcnow() + timedelta(days=7)` → 时区裸值，Neo4j 序列化为 UTC 但客户端可能误解 | 中 | Go 全程 `time.UTC` 显式 |
| L2-04 | `weaver.weave_for_node` | 若同湖节点为空，返回 `[]`；但若 Neo4j 临时不可达，异常会冒泡到 BackgroundTask 静默失败 | 高 | Go 用 `defer recover()` + 写入 `outbox.weaver.failed` 事件 |
| L2-05 | `ws/hub.broadcast` | 死连接清理是先收集再删，并发 join 时可能漏清理 | 中 | Go 用 `sync.Map` + `range` 删除模式 |
| L2-06 | `lake_service.create_lake` | Neo4j 创建成功后 PG 写入失败 → 出现"孤儿湖"（图有节点但无 owner） | 高 | Go 用 outbox + saga：先 PG INSERT，事件驱动 Neo4j 创建 |
| L2-07 | `nodes.create_node` 端点 | BackgroundTask 异常默默吞噬 | 中 | Go 用专用 worker pool + 错误指标 |

---

## 第 3 轮 · 架构一致性与代码规范

### ⚠️ 问题

| ID | 范围 | 问题 | Go 重写时修正 |
|----|------|------|--------------|
| L3-01 | `app/services/*` | service 直接接收 `AsyncSession`，违反"领域服务不知道数据源"原则 | Go 引入 `Repository` 接口，service 依赖接口而非具体实现 |
| L3-02 | `app/api/v1/nodes.py` | handler 内嵌 `_weave_and_broadcast`，业务逻辑泄漏到 transport | Go 抽到 `service/node/weave.go` |
| L3-03 | `app/models/*` SQLAlchemy | ORM 模型 = API 响应 = 持久化 = 领域模型，四合一 | Go 严格分层：`domain.Node` ↔ `dto.NodeResponse` ↔ `store.NodeRow`，用 mapper |
| L3-04 | `core/security.py` | `get_current_user` 返回 `dict`，类型丢失 | Go 用 `type Principal struct { UserID uuid.UUID; Email string }` |
| L3-05 | `core/db.py` | 三个 DB（PG/Neo4j/Redis）init/close 在一个文件，违反单一职责 | Go 拆 `internal/store/{pg,neo4j,redis}/` 各自管理生命周期 |
| L3-06 | 命名 | `actor_id` / `user_id` / `owner_id` 在不同 service 混用 | Go 统一 `actor` (操作者) / `subject` (被操作对象) |
| L3-07 | 文档 | service 函数缺 docstring 注明权限要求 | Go 用 `// Permission: NAVIGATOR+` 顶注 |

---

## 第 4 轮 · 安全性与数据隔离

### 🔴 高危问题

| ID | 文件 | 问题 | Go 重写时修正 |
|----|------|------|--------------|
| L4-01 | `nodes.evaporate` | 任何登录用户都能调 evaporate，未校验是否为节点 owner 或湖 owner | Go handler 层 `if node.OwnerID != actor && lakeRole < OWNER { return 403 }` |
| L4-02 | `nodes.list_by_lake` | 仅校验 OBSERVER，未校验私有湖的成员关系（公开湖也无访客限制） | Go 加 `if !lake.IsPublic && !isMember { return 403 }` |
| L4-03 | `cloud.condense` | 没校验 mist_ids 是否全部归属当前用户（可能凝露他人草稿） | Go 在 condense 前 `WHERE owner_id = $actor` |
| L4-04 | `weaver.weave_for_node` | `:RELATES_TO {by:'weaver'}` 边没有 `created_by_user`，无法审计 | Go 写入 `{by:'weaver', actor: actor_id}` |
| L4-05 | `core/security.create_access_token` | iat / exp 用 `datetime.utcnow()` 但未带 nbf，无防重放 | Go 加 `nbf` + 黑名单（Redis）支持 |
| L4-06 | `core/config.SECRET_KEY` | 默认值是开发占位，生产环境若忘改会静默使用弱密钥 | Go 启动时 `if env == "production" && key == default { panic }` |
| L4-07 | 全局 | 无 rate limit 中间件实现（README 提到但代码缺失） | Go 第一阶段就加，用 `ulule/limiter` |
| L4-08 | `auth_service.register_user` | email 校验仅靠 Pydantic `EmailStr`，无 MX 真实性校验 | Go M2 加 SMTP 验证 + 邮件确认链接 |

### ⚠️ 中危

| ID | 问题 | Go 修正 |
|----|------|---------|
| L4-09 | CORS `allow_origins=settings.CORS_ORIGINS` 默认 `["*"]` 风险 | Go 强制配置项不允许 `*` |
| L4-10 | WebSocket 鉴权 token 通过 query string → 进访问日志 | Go 改用握手期 Sec-WebSocket-Protocol 子协议 + Bearer |

---

## 第 5 轮 · 性能与资源管理

### ⚠️ 问题

| ID | 文件 | 问题 | Go 重写时修正 |
|----|------|------|--------------|
| L5-01 | `embeddings.embed` | sha256 迭代 256 次 → 每次嵌入 ~150μs，N 节点 weaver 是 O(N) 同步循环 | Go 用 goroutine pool 并发计算 + 缓存（content hash → vec） |
| L5-02 | `weaver.weave_for_node` | 同湖查询每次创建 session，无连接复用上下文 | Go 用 `driver` 全局单例 + `session.WithContext` |
| L5-03 | `ws/hub.LakeHub` | 内存 dict，单进程 → 横向扩展失败；M2 必须 Redis Pub/Sub | Go 设计时即定义 `interface Broker { Publish/Subscribe }`，本地实现 + Redis 实现可切换 |
| L5-04 | `core/db.py` PG | `Base.metadata.create_all` 在 lifespan 中同步调用 → 启动慢 | Go 用 migrate/atlas 单独迁移命令，启动只验证 schema 版本 |
| L5-05 | SQLAlchemy `selectinload` 缺失 | N+1 查询风险 | Go sqlc 生成的查询天然单 SQL |
| L5-06 | `nodes.create_node` | weave 用 BackgroundTask（同进程），峰值时阻塞 event loop | Go 用独立 worker（`internal/worker/weaver`）消费 outbox |
| L5-07 | Neo4j driver | Python async driver 4.x 有已知性能瓶颈 | Go neo4j-go-driver/v5 性能 + 类型双优 |
| L5-08 | 日志 | 每请求都 `log.info` 全量 payload → 高 QPS 下 IO 阻塞 | Go zerolog + sampling（`zerolog.Sample`） |

---

## 总结：Go 重写 12 项必做改进

| # | 改进 | 来源 | 优先级 |
|---|------|------|--------|
| 1 | 引入 Repository 接口分层（领域 / DTO / 持久化） | L3-01, L3-03 | P0 |
| 2 | Lake 创建用 outbox saga 模式（PG → Neo4j） | L2-06 | P0 |
| 3 | 节点蒸发权限严格校验（owner 或湖 owner） | L4-01 | P0 |
| 4 | 私有湖访问校验成员关系 | L4-02 | P0 |
| 5 | Weaver 抽出独立 worker（消费 outbox） | L2-04, L2-07, L5-06 | P0 |
| 6 | UTF-8 安全的密码截断 | L2-01 | P1 |
| 7 | 空字符串嵌入特判 | L2-02 | P1 |
| 8 | Broker 接口设计（本地 + Redis 双实现） | L5-03 | P1 |
| 9 | 启动期密钥强校验（生产禁用默认值） | L4-06 | P1 |
| 10 | Rate limit 中间件 first-class | L4-07 | P1 |
| 11 | WebSocket 子协议鉴权（不走 query） | L4-10 | P2 |
| 12 | 迁移命令独立（不在 lifespan） | L5-04 | P2 |

---

## 审查结论

**Python M1 状态**：作为**验证性原型**合格，作为**生产实现**有 8 项 P0/P1 缺陷。
**Go 重写策略**：以本报告为蓝图，先解决 P0 五项，再迭代 P1 六项。
**技术债清单**：保留本文档作为 Go 实现完成后的"账本核对单"。

**反方签字**：审查完成 · 移交决策者
**项目经理裁决**（决策者）：批准 Go 重写按本清单执行 · Python 资产打 tag 归档
