# D4 · API 网关设计

**版本：** v5.3（消化整合版）
**日期：** 2026‑04‑21
**核心隐喻：** 水坝与分洪闸（Dam & Diversion Gate）
**职责：** 流量控制、身份认证、请求路由、安全防护

> 本文档由原 `系统技术架构：API 网关设计.md` (v5.2) 消化整合而成。

---

## 一、总体架构

```
[ Client Apps ]  Web / Mobile / Third-party
       │
       │ HTTPS (REST / WebSocket)
       ▼
[  API Gateway  ]  ← 本文档核心
       │
       │ gRPC / HTTP（内部）
       ▼
+──────────────────────────────────────────+
| AuthSvc | GraphSvc | AISvc | StorageSvc  |
| CloudSvc | IcebergSvc | RealtimeSvc      |
+──────────────────────────────────────────+
```

### 选型建议

- **高性能：** Kong (Nginx/Lua) 或 Envoy
- **轻量快速：** FastAPI 自身作为网关 + HTTPProxy 中间件
- **推荐：** 初期 FastAPI，规模上来后无缝迁移至 Kong

---

## 二、路由设计

### 2.1 RESTful（无状态）

| Endpoint Prefix | 转发服务 | 文档 |
| :--- | :--- | :--- |
| `/api/v1/auth/*` | Auth | G1 §五 |
| `/api/v1/lakes/*` | Graph | G1 / D6 |
| `/api/v1/graph/*` | Graph | G1 |
| `/api/v1/cloud/*` | Cloud | G2 |
| `/api/v1/icebergs/*` | Iceberg | G3 |
| `/api/v1/assets/*` | Storage | G4 |
| `/api/v1/ai/*` | AI | G5 |
| `/api/v1/search/*` | Graph | G1 |

### 2.2 WebSocket（有状态）

- Endpoint：`/ws/{lake_id}`
- 同 `lake_id` 的连接绑定到同一后端实例（或通过 Redis Pub/Sub 广播）
- 详见 [D5 多人实时协作](D5-多人实时协作.md)

---

## 三、安全与认证

### 3.1 JWT 认证

- 流程：登录 → Auth Svc 签发 → 客户端 Header `Authorization: Bearer <token>`
- 网关验证签名与有效期；JWT Claims 见 [G1 §5.3](G1-数据模型与权限设计.md)

### 3.2 鉴权粒度

- **湖泊级：** 网关查询 PostgreSQL `lake_memberships`，判断用户是否属于 `lake_id`
- **节点级：** 转发到后端，后端按 [G1 §五权限矩阵](G1-数据模型与权限设计.md) 校验
- **拒绝文案：** "**此湖域未对外开放。**" → `403 Forbidden`

### 3.3 速率限制

| 类型 | 默认 |
| :--- | :--- |
| 普通 API | 100 req/min/IP |
| WebSocket | 10 msg/s/user |
| AI 调用 | 见 [G5 §六 Token 预算](G5-AI服务编排-织网工作流.md) |

文案：**"水流过急，请稍后再试。"**

---

## 四、流量治理

### 4.1 请求/响应转换

- 前端 snake_case ↔ 后端 camelCase 由网关统一处理
- 时间戳统一为 ISO 8601 UTC

### 4.2 熔断与降级

- AI 服务错误率 > 50% 触发熔断
- 降级响应文案：**"潮汐异常，AI 暂歇，请稍候。"**
- 详细降级行为：见 [G5 §五](G5-AI服务编排-织网工作流.md)

---

## 五、日志与监控

### 5.1 日志结构

```json
{
  "timestamp": "...",
  "client_ip": "1.1.1.1",
  "user_id": "uuid",
  "method": "POST",
  "path": "/api/v1/graph/node",
  "latency_ms": 150,
  "status_code": 200,
  "trace_id": "..."
}
```

### 5.2 关键指标

- QPS（每秒查询）
- P99 Latency
- Active WebSocket Connections（同舟人数）
- 详见 [G6 可观测性](G6-可观测性与监控.md)

---

## 六、示例：一次"投石"请求的旅程

1. **Frontend (Delta View)**：双击画布 → `POST /api/v1/graph/nodes`
   `{ "lake_id":"west_lake", "content":"新灵感", "position":{...} }`
2. **API Gateway**：拦截 → 校验 JWT → 鉴权（`west_lake` 写权限）→ 限流 → 转发
3. **Graph Service**：写 Neo4j → 触发 Outbox 事件（异步生成 Embedding 与 AI 织网，详见 G5）→ 返回 `node_id`
4. **Gateway Response**：转换 JSON 格式返回前端
5. **WebSocket Broadcast（并行）**：通知同 `lake_id` 所有 WS 连接

---

## 七、相关文档

- 鉴权数据：[G1 §五](G1-数据模型与权限设计.md)
- WebSocket 协作：[D5](D5-多人实时协作.md)
- AI 降级链：[G5 §五](G5-AI服务编排-织网工作流.md)

---

**文档状态：** 定稿
**版本来源：** 整合自原 `系统技术架构：API 网关设计.md` (v5.2)，原文件已删除。
