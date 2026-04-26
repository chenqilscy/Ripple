# 开发者 Onboarding 指南

**版本：** v0.1
**目标读者：** 新加入青萍 (Ripple) 后端 / 全栈团队的工程师
**预期时长：** 30 分钟本地跑通

---

## 一、项目概览

**青萍 (Ripple)** 是一个"水文生态"隐喻的创意系统 —— 把灵感捕捉、织网、沉淀、分享设计成一条"水的循环"。

### 核心概念一分钟
| 概念 | 对应产品面 | 对应技术面 |
| :--- | :--- | :--- |
| 湖泊 (Lake) | 一个创作工作区 | Neo4j `(:Lake)` + PG `lake_memberships` |
| 浮萍 (Node) | 一个灵感节点 | Neo4j `(:Node {state})`，状态机见 G1 §三 |
| 暗流 (Edge) | 节点间的关系 | Neo4j `[:RELATES_TO]` |
| 云霓 (Cloud) | 移动端碎片采集 | RN + SQLite Outbox + `MIST` 态 |
| 冰山 (Iceberg) | 成品快照 | Neo4j `(:Iceberg)` + S3 snapshots 桶 |
| 决堤口 (Publish) | 分享三出口 | 入海/抛饵/入流（见 G3 §六） |

---

## 二、代码仓结构

```
Ripple/
├── AGENTS.md                     # 十角色团队流程
├── docker-compose.yml            # 本地开发栈
├── README.md                     # 项目门户
├── backend/                      # FastAPI 后端
│   ├── pyproject.toml
│   ├── Dockerfile
│   ├── .env.example
│   ├── app/
│   │   ├── main.py               # FastAPI 入口
│   │   ├── core/
│   │   │   ├── config.py         # Settings
│   │   │   ├── db.py             # Neo4j + PG + Redis
│   │   │   ├── security.py       # JWT
│   │   │   └── logging.py
│   │   └── api/v1/               # 路由：auth/lakes/nodes/cloud/iceberg/shares
│   └── tests/
└── docs/                         # 全部设计文档（D0-D7 + G1-G11 + MVP + 商业化）
    ├── README.md                 # 文档地图
    ├── pinpai/                   # 品牌层
    ├── system-design/            # 技术层
    ├── user-story/               # 用户故事（最高判据）
    └── dev/                      # 开发者指南（本文所在）
```

**每个设计文档 = 一份契约**。写代码前先读对应的 D / G 文档，尤其：
- 改 API → 先看 [D4-API网关设计](../system-design/API网关设计.md)
- 改数据模型 → 先看 [G1-数据模型与权限设计](../system-design/数据模型与权限设计.md)
- 改 AI → 先看 [G5-AI服务编排](../system-design/AI服务编排-织网工作流.md)
- 任何功能 → 最终能被 [user-story/story.md](../user-story/story.md) 10 个故事中的一个解释

---

## 三、本地环境搭建

### 3.1 前置要求

| 工具 | 版本 | 说明 |
| :--- | :--- | :--- |
| Python | ≥ 3.11 | 后端语言 |
| Docker + Compose | 最新 | 本地服务栈 |
| Git | 任意 | 版本控制 |
| uv（推荐）| 最新 | Python 包管理 `pip install uv` |

### 3.2 启动本地服务栈

```powershell
# 克隆仓库后
cd Ripple
docker compose up -d neo4j postgres redis minio
# 检查状态
docker compose ps
```

服务端点：
- Neo4j Browser: http://localhost:7474 （用户 `neo4j` / 密码 `ripple_dev_pwd`）
- Postgres: `localhost:5432` （用户 `ripple` / 库 `ripple`）
- Redis: `localhost:6379`
- MinIO Console: http://localhost:9001 （`minioadmin` / `minioadmin`）

### 3.3 后端启动（方式 A · 本地 Python）

```powershell
cd backend
uv venv
.venv\Scripts\Activate.ps1
uv pip install -e ".[dev]"
Copy-Item .env.example .env
# 将 .env 中 neo4j/postgres/redis/minio 的 host 改为 localhost
uvicorn app.main:app --reload --port 8000
```

访问 http://localhost:8000/docs 看 OpenAPI 文档。

### 3.4 后端启动（方式 B · 整体容器化）

```powershell
docker compose up --build backend
```

### 3.5 冒烟测试

```powershell
curl http://localhost:8000/health
# 期望：{"status":"ok","service":"ripple-backend","version":"0.1.0"}
```

---

## 四、开发工作流

### 4.1 Git 提交规范

严格参照 [`AGENTS.md`](../../AGENTS.md) §Git 规范。格式：`<type>: <中文描述>`。

```
feat: 实现节点凝露 API
fix: 修复 MIST TTL 被凝露后未清空
refactor: 拆分 Weaver 模块
```

### 4.2 代码规范

```powershell
cd backend
ruff check .        # Lint
ruff format .       # 格式化
mypy app            # 类型检查
pytest              # 单测
pytest --cov=app    # 覆盖率（目标 ≥ 70%）
```

### 4.3 加一个新 API 的标准流程

1. 在对应 D/G 文档里找到契约（字段、错误码、限流）
2. 在 `backend/app/api/v1/<module>.py` 新增路由
3. 写 Pydantic 模型（请求/响应分离）
4. 在 `tests/` 加测试
5. 更新 OpenAPI 描述 + 关联故事编号（注释）
6. 提交 PR，按 `AGENTS.md` 五轮代码审查

### 4.4 节点状态机变更的硬要求

**所有涉及节点 state 的改动**必须：
1. 先更新 [G1 §三状态机图](../system-design/数据模型与权限设计.md)
2. 能回答：此状态的前驱 / 后继 / TTL / 可见性 / 可编辑性
3. 写覆盖全部转换的测试
4. 在 [G6](../system-design/可观测性与监控.md) 加对应 metric

---

## 五、常见问题

**Q: Neo4j 起不来？**
A: 检查 7474/7687 端口占用 `netstat -ano | findstr 7687`；或清掉 volume `docker volume rm ripple_neo4j_data`。

**Q: 本地连不上 Neo4j？**
A: 在 `.env` 里把 `NEO4J_URI` 从 `bolt://neo4j:7687` 改为 `bolt://localhost:7687`。容器化启动时走服务名，本地启动时走 localhost。

**Q: 我想改 AI Agent，从哪开始？**
A: 必读 [G5 §2.1 时序图](../system-design/AI服务编排-织网工作流.md) + [G5 §6 成本压缩](../system-design/AI服务编排-织网工作流.md) + [D7 Prompt 规范](../system-design/AI-Prompt工程规范.md)。不熟这三份文档不准碰 AI 层。

**Q: 我想加一个新业务概念？**
A: 先问："这概念用水文隐喻怎么说？" 如果答不出，不是技术问题，是产品问题。回 [pinpai/01-品牌与产品概念](../pinpai/01-品牌与产品概念.md) 或找 PM。

---

## 六、下一步阅读清单

按优先级：
1. [用户故事 10 篇](../user-story/story.md) ← **必读**
2. [D0 技术架构总览](../system-design/技术架构总览.md)
3. [G1 数据模型与权限](../system-design/数据模型与权限设计.md)
4. [MVP 范围与里程碑](../MVP-范围与里程碑.md)
5. [AGENTS.md 十角色流程](../../AGENTS.md)

读完这 5 份你就能正常参与团队讨论了。

---

**欢迎来到青萍。愿你写下的每一行代码，都如风过湖面，起一圈合情合理的涟漪。**
