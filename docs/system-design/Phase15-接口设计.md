# Phase 15.1 · API 接口设计文档

> 状态：📐 接口设计阶段（2026-04-28）  
> 十角色流程位置：需求分析 ✅ → **接口设计（当前）** → 正方架构方案  
> 参考：docs/system-design/Phase15-需求分析.md  
> OpenAPI 草案将同步写入 api/openapi.yaml §Phase15

---

## 接口设计师讨论输出

### 一、设计原则（本期 API 共识）

1. **REST 路径遵循现有 /api/v1/** 前缀，不新开版本
2. **异步任务**（AI trigger）返回 `202 Accepted` + `Location` 头，不阻塞调用方
3. **错误码**统一遵循现有约定：422 = 业务拒绝 / 400 = 输入非法 / 401 = 未认证 / 403 = 无权限
4. **分页**：列表接口统一 `?limit=&offset=` 游标，返回 `{ "data": [...], "total": N }`
5. **Stub 支付**：Phase 15 不接真实支付网关，`subscription` 接口直接 `200 OK`（Stub 模式标记）

---

## 二、C.1 AI Trigger API（AI 节点触发）

### 2.1 触发 AI 填充

```
POST /api/v1/lakes/{lake_id}/nodes/{node_id}/ai_trigger
```

**请求头**：`Authorization: Bearer <jwt>`

**请求体**：
```json
{
  "prompt_template_id": "uuid",      // 必填，使用哪个模板
  "input_node_ids": ["uuid", ...],   // 可选，额外引用的节点作为上下文（最多 10 个）
  "override_vars": {                  // 可选，临时覆盖模板变量
    "custom_key": "custom_value"
  }
}
```

**响应 202 Accepted**：
```json
{
  "ai_job_id": "uuid",
  "status": "pending",
  "node_id": "uuid",
  "estimated_seconds": 15
}
```
+ `Location: /api/v1/lakes/{lake_id}/nodes/{node_id}/ai_status`

**错误码**：
| 状态码 | 场景 |
|--------|------|
| 400 | `prompt_template_id` 缺失或格式非法 |
| 403 | 当前用户对该节点无写权限 |
| 404 | lake / node / template 不存在 |
| 409 | 该节点已有进行中的 AI job（禁止并发触发） |
| 422 | 组织 LLM 配额已耗尽（`max_llm_calls` 字段，Phase 15.D 引入） |

---

### 2.2 查询 AI 状态

```
GET /api/v1/lakes/{lake_id}/nodes/{node_id}/ai_status
```

**响应 200**：
```json
{
  "ai_job_id": "uuid",
  "status": "pending | processing | done | failed",
  "progress_pct": 60,               // 0-100，近似值（LLM 流式时更新）
  "started_at": "2026-04-28T12:00:00Z",
  "finished_at": null,
  "error": null                      // failed 时为字符串描述
}
```

**流式替代**（Phase 15.2 可选，Phase 15.1 先用轮询）：
- `GET /api/v1/lakes/{lake_id}/nodes/{node_id}/ai_stream` → SSE，`event: progress`

---

### 2.3 重新生成

```
POST /api/v1/lakes/{lake_id}/nodes/{node_id}/ai_trigger
```
（与触发相同接口，若 `status = done | failed`，自动重置为 `pending` 并重新触发）

**业务约束**：若状态为 `processing`，返回 409（禁止中断进行中的任务）。

---

## 三、C.2 Prompt 模板库 API

### 3.1 创建模板

```
POST /api/v1/prompt_templates
```

**请求体**：
```json
{
  "name": "学习笔记整理",
  "description": "将选中节点内容整理为结构化学习笔记",
  "template": "将以下内容整理为学习笔记，包含摘要和要点：\n\n{{node_content}}\n\n湖名：{{lake_name}}",
  "scope": "private | org",          // private = 仅创建者可用；org = 组织内共享
  "org_id": "uuid"                   // scope=org 时必填
}
```

**响应 201 Created**：
```json
{
  "id": "uuid",
  "name": "学习笔记整理",
  "template": "...",
  "scope": "private",
  "created_by": "uuid",
  "created_at": "2026-04-28T12:00:00Z"
}
```

**支持变量**（模板引擎 Phase 15.1 仅支持基础替换）：
| 变量 | 替换值 |
|------|--------|
| `{{node_content}}` | 触发时目标节点内容（HTML 去标签后的纯文本） |
| `{{lake_name}}` | 当前湖名称 |
| `{{selected_nodes}}` | `input_node_ids` 中所有节点内容拼接（`\n---\n` 分隔） |
| `{{user_name}}` | 当前用户 display_name |

---

### 3.2 列表 / 详情 / 删除

```
GET    /api/v1/prompt_templates?scope=all|private|org&org_id=uuid&limit=20&offset=0
GET    /api/v1/prompt_templates/{id}
DELETE /api/v1/prompt_templates/{id}
PATCH  /api/v1/prompt_templates/{id}   // 更新 name/description/template/scope
```

**列表响应**：
```json
{
  "data": [
    { "id": "uuid", "name": "...", "scope": "private", "created_by": "uuid", "created_at": "..." }
  ],
  "total": 42
}
```

**权限规则**：
- `private` 模板：仅创建者可读 / 写 / 删
- `org` 模板：同 org 成员可读；创建者 + org OWNER/ADMIN 可写 / 删
- 平台管理员可读所有

---

## 四、D.1 套餐订阅 API

### 4.1 查看套餐列表

```
GET /api/v1/subscriptions/plans
```

**响应 200**（配置写死，Phase 16 再做可配置）：
```json
{
  "plans": [
    {
      "id": "free",
      "name": "免费版",
      "price_cny_monthly": 0,
      "quotas": { "max_members": 3, "max_lakes": 50, "max_nodes": 10000, "max_storage_mb": 1024 }
    },
    {
      "id": "pro",
      "name": "专业版",
      "price_cny_monthly": 29,
      "quotas": { "max_members": 20, "max_lakes": 500, "max_nodes": 100000, "max_storage_mb": 10240 }
    },
    {
      "id": "team",
      "name": "团队版",
      "price_cny_monthly": 99,
      "quotas": { "max_members": 100, "max_lakes": 5000, "max_nodes": 1000000, "max_storage_mb": 102400 }
    }
  ]
}
```

---

### 4.2 提交套餐选择（Stub 支付）

```
POST /api/v1/organizations/{org_id}/subscription
```

**请求体**：
```json
{
  "plan_id": "pro",
  "billing_cycle": "monthly | annual",
  "stub_confirm": true              // Phase 15 必须为 true（真实支付 Phase 16 引入）
}
```

**响应 200**（Stub 成功）：
```json
{
  "subscription_id": "uuid",
  "org_id": "uuid",
  "plan_id": "pro",
  "status": "active",
  "started_at": "2026-04-28T12:00:00Z",
  "expires_at": "2026-05-28T12:00:00Z",
  "stub": true
}
```

**副作用**：后端立即调用 `OrgService.UpdateQuota`，将 `org_quotas` 更新为目标套餐配额。

**错误码**：
| 状态码 | 场景 |
|--------|------|
| 400 | `stub_confirm` 不为 true（Phase 15 中非 stub 调用被拒绝） |
| 403 | 非 org OWNER 不可更改套餐 |
| 404 | org / plan 不存在 |
| 422 | 当前用量超出目标套餐限额（降级场景）：`{"error": "downgrade_blocked", "exceeded": ["max_nodes"]}` |

---

### 4.3 查询当前订阅

```
GET /api/v1/organizations/{org_id}/subscription
```

**响应 200**：
```json
{
  "subscription_id": "uuid",
  "plan_id": "free",
  "status": "active | expired | cancelled",
  "started_at": "...",
  "expires_at": null,
  "stub": false
}
```

---

## 五、D.2 LLM 调用账单 API

### 5.1 获取用量汇总

```
GET /api/v1/organizations/{org_id}/llm_usage?days=30
```

**响应 200**：
```json
{
  "org_id": "uuid",
  "period_days": 30,
  "total_calls": 1234,
  "total_estimated_cost_cny": 12.34,
  "by_provider": [
    {
      "provider": "zhipu",
      "calls": 800,
      "avg_duration_ms": 1200,
      "estimated_cost_cny": 8.00
    },
    {
      "provider": "deepseek",
      "calls": 434,
      "avg_duration_ms": 900,
      "estimated_cost_cny": 4.34
    }
  ],
  "by_day": [
    { "date": "2026-04-28", "calls": 45, "estimated_cost_cny": 0.45 }
  ]
}
```

**实现说明**：
- 数据来源：`llm_calls` 表（已有），按 `org_id` + `created_at` 聚合
- 费用估算：provider 单价配置（配置文件，Phase 15 写死，Phase 16 管理后台配置）
- `llm_calls` 现有字段：`id, provider, model, prompt_tokens, completion_tokens, duration_ms, created_at`；需增加 `org_id` 字段（migration 0019）

**错误码**：
| 状态码 | 场景 |
|--------|------|
| 400 | `days` 超出 [1, 90] 范围 |
| 403 | 非 org OWNER/ADMIN |

---

## 六、数据模型变更（需新增 migration）

| 编号 | 表变更 | 内容 |
|------|-------|------|
| 0019 | `llm_calls` + `org_id` | `ALTER TABLE llm_calls ADD COLUMN org_id UUID REFERENCES organizations(id) ON DELETE SET NULL;` |
| 0020 | 新增 `prompt_templates` | `id UUID PK, name TEXT, description TEXT, template TEXT, scope TEXT, org_id UUID nullable FK, created_by UUID FK users, created_at, updated_at` |
| 0021 | 新增 `ai_jobs` | `id UUID PK, node_id UUID FK, lake_id UUID FK, prompt_template_id UUID FK, status TEXT, progress_pct INT, input_node_ids UUID[], started_at, finished_at, error TEXT, created_at` |
| 0022 | 新增 `org_subscriptions` | `id UUID PK, org_id UUID FK UNIQUE, plan_id TEXT, status TEXT, billing_cycle TEXT, stub BOOL, started_at, expires_at, created_at` |

---

## 七、PM 确认（接口边界）

✅ Phase 15.1 交付范围：
- C.1：ai_trigger + ai_status（轮询，无 SSE）
- C.2：prompt_templates CRUD（scope: private only，org 共享 Phase 15.2）
- D.1：plans 列表 + subscription（stub 模式）
- D.2：llm_usage 汇总（by_provider + by_day）

⏸ 推迟到 Phase 15.2：
- AI 流式状态（SSE）
- Prompt 模板 org 共享
- 真实支付网关（Phase 16）
- LLM 配额上限执行（`max_llm_calls`）

---

## 八、反方审查意见（架构决定前）

**【反方（审查员）】**：

1. **Prompt 注入风险**：`override_vars` 允许任意 key-value，后端必须对模板替换后的最终 Prompt 做长度上限校验（建议 4000 tokens 前硬截断），并对 value 做 HTML 转义防 XSS（如果 Prompt 结果最终展示在前端）。

2. **ai_jobs 表并发安全**：`POST .../ai_trigger` 的 409 逻辑必须在数据库层实现（`SELECT ... FOR UPDATE`），不能仅依靠应用层检查，否则并发两次触发会产生两个 job。建议：trigger 插入 `ai_jobs` 时加 `ON CONFLICT (node_id) WHERE status IN ('pending','processing') DO NOTHING RETURNING id`，返回空则 409。

3. **org_subscriptions UNIQUE 约束**：一个 org 同时只能有一个 active subscription，UNIQUE 索引加 `WHERE status = 'active'`（部分唯一索引），允许历史记录保留。

4. **llm_calls.org_id 回填**：新增列后历史数据 `org_id = NULL`，前端账单 UI 需处理 NULL（展示为"未归属"而非崩溃）。

5. **stub 字段危险**：生产环境 `stub_confirm: true` 的路径如果没有 feature flag 保护，任何人可以免费升级套餐。建议：`stub_confirm` 只在 `RIPPLE_STUB_PAYMENT=true` 环境变量开启时生效，否则 501 Not Implemented。

**【正方（架构师）】响应**：已采纳全部 5 条，见 T53 架构方案文档。

---

## 九、决策者裁决

**【决策者（项目经理）】**：

接口设计整体通过，以下裁决：

1. **Phase 15.1 Sprint 1（3 周）**：实现 C.1 + D.1 + migrations 0019-0022
2. **Phase 15.1 Sprint 2（2 周）**：实现 C.2 + D.2
3. **反方 5 条审查意见全部纳入实现门槛**（不得作为 TODO 遗留）
4. **接口版本冻结**：本文档发布后，接口路径与请求/响应字段进入承诺期，变更须经本流程重走

**裁决签字**：决策者 ✅ 2026-04-28
