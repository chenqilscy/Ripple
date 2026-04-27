# 青萍 (Ripple) · 水文生态创意系统

> 把"灵感捕捉、织网、沉淀、分享"设计成**水的循环**的 AI 创作系统。

**当前进度：M1~M12 已完成（2026-Q2）** — 已覆盖协作、AI 编织、空间与权限、通知、组织、模板、分享、Three.js 动效与部署链路；当前进入 **Phase 13 启动测试与发布准备**。

---

## 技术栈

| 层 | 选型 |
|----|------|
| 后端 | Go 1.23 · chi/v5 · pgx/v5 · neo4j-go-driver/v5 · go-redis/v9 · zerolog · nhooyr.io/websocket |
| 前端 | React 18 · TypeScript · Vite · Three.js（动效预留） |
| 关系存储 | PostgreSQL 16+（用户 / 成员 / 邀请 / outbox / cloud_tasks / llm_calls / node_revisions / spaces） |
| 图存储 | Neo4j 5+（Lake / Node / Edge） |
| 缓存 / 消息 | Redis 7+（Presence / WS Pub/Sub / 限流） |
| 对象存储 | MinIO / S3（M3 起 PERMA 长文 / 多媒体） |

详见 [系统约束规约](docs/system-design/系统约束规约.md)。

---

## 快速上手

新手 30 分钟跑通：[docs/快速上手.md](docs/快速上手.md)。

最简版（已装 Go 1.23 / Node 20 / 中间件）：

```powershell
# 1. 后端
cd backend-go
copy .env.example .env   # 编辑 PG/Neo4j/Redis 连接 + JWT_SECRET
psql ...  -f migrations/0001_init.up.sql   # 依次跑 0001..0006
go run ./cmd/server

# 2. 前端
cd ..\frontend
npm install
npm run dev

# 浏览器打开 http://localhost:5173 注册账号即可
```

### Phase 13 联调环境（一键）

已装 Docker 的机器可直接使用：

```powershell
# 先在当前 PowerShell 注入必要变量
$env:PG_PASSWORD="replace-me"
$env:NEO4J_PASSWORD="replace-me"
$env:REDIS_PASSWORD="replace-me"
$env:JWT_SECRET="replace-me-with-at-least-32-chars"

# 拉起 staging 环境并执行冒烟
./scripts/bootstrap-staging.ps1

# 回收环境（默认删卷）
./scripts/teardown-staging.ps1
```

启动后默认地址：前端 `http://127.0.0.1:14173`，后端 `http://127.0.0.1:18000`。

---

## 核心能力（已交付）

### M1 · 知识基础设施
- 用户/JWT 鉴权 · Lake/Node/Edge 三层数据 · 节点状态机（DROP→MIST→CLOUD→PERMA）
- Outbox + Saga：PG → Neo4j 跨库一致性
- WebSocket 实时事件（NodeCreated / EdgeCreated / NodeStateChanged 等）

### M2 · 协作 + AI
- **F2 节点版本与回滚**：每次内容变更落 `node_revisions`，支持任意 rev 回滚
- **F3 Lake 邀请系统**：Owner 生成邀请 token，链接式接受加入（NAVIGATOR/PASSENGER/OBSERVER 角色）
- **F4 Presence**：Redis 集合维护当前在 Lake 的用户 + 心跳过期清理 + 头像条
- **造云**：用户在 MIST 节点上发起，由 AIWeaver worker 池调用 LLM Router 落 CLOUD 节点
- **LLM Provider 路由**：智谱 / DeepSeek / OpenAI / 火山 / MiniMax / OpenAI 兼容（Ollama 等） / **Claude Code CLI**（订阅制，本机 `claude` 调用）
- **流式输出**：OpenAI 兼容客户端实现 `StreamProvider`（SSE 解析）
- **全局速率限制**：每个 Provider 套 token bucket（`RIPPLE_LLM_RPS` / `_BURST`）

### M3-S1 · Space 工作空间（进行中）
- spaces / space_members 表 + Space CRUD + 成员管理（OWNER/EDITOR/VIEWER）
- M3-S2 起：PERMA 凝结 / 反馈事件 / 用户偏好画像

完整路线见 [M3 设计白皮书](docs/system-design/设计白皮书-M3.md)。

---

## 文档地图

```
docs/
├── README.md                                文档导航 + 术语
├── 快速上手.md                              新人 30 分钟跑通
├── pinpai/                                  品牌定位
├── user-story/story.md                      10 个用户故事（最高判据）
└── system-design/
    ├── 系统约束规约.md                       硬约束（技术栈/版本/安全）
    ├── 约束变更日志.md                       重大决策记录
    ├── 自学习日志.md                         代理沉淀
    ├── 收官总结-M2.md                        M2 全交付摘要
    ├── 设计白皮书-M3.md                      M3 5 sprints 蓝图
    ├── 技术债清单.md                         TD-001..TD-013（含已偿还）
    ├── LLM-Provider-接入手册.md              新增 provider 步骤 / Claude Code 启用 / 流式 / 速率限制
    └── ...                                   其它专题（架构、Prompt 工程、动效引擎等）
```

---

## 团队协作

本项目采用**十角色 AI 团队流程**，详见 [`AGENTS.md`](AGENTS.md)。功能需求从需求创造师 → PM → 接口设计 → 正反方辩论 → 决策者裁决 → 实现 → 五轮代码审查 → QA → 体验方 → PM 验收闭环。

提交规范：`<type>: <中文描述>`（type ∈ feat/fix/docs/chore/refactor/test/perf）。

---

## CI

GitHub Actions（`.github/workflows/ci.yml`）：每次 push/PR 跑后端 `go vet + build + test -race`、前端 `tsc + build`，以及 Phase 13 联调中间件集成测试矩阵。

---

## License

TBD（待商业化模型敲定）。
