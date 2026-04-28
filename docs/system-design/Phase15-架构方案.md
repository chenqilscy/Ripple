# Phase 15 · 架构方案文档

> 状态：📐 架构设计阶段（2026-04-28）  
> 十角色流程位置：接口设计 ✅ → **正方架构方案（当前）** → 反方审查 → 实现  
> 参考：docs/system-design/Phase15-接口设计.md  
> 参考：docs/system-design/Phase15-需求分析.md

---

## 【正方（架构师）】方案

---

## 一、AI 节点异步执行架构

### 1.1 总体设计

```
HTTP Handler                  DB / Neo4j              Worker Pool
─────────────────────         ──────────────────────  ──────────────────────────
POST .../ai_trigger  ──→      ai_jobs INSERT (pending)
                              ↓ (channel notify)
                     ←── 202 Accepted + job_id
                                                       AiJobWorker.Run()
                                                        ├── SELECT FOR UPDATE SKIP LOCKED (ai_jobs WHERE status='pending')
                                                        ├── 更新 status='processing'
                                                        ├── 渲染 Prompt（模板变量替换）
                                                        ├── LLM Router.Complete(prompt)
                                                        │     └── Weaver 现有路由（按 Modality TEXT）
                                                        ├── 写回 node.content（Neo4j MATCH SET）
                                                        └── 更新 ai_jobs status='done'/'failed'

轮询 GET .../ai_status ──→   SELECT ai_jobs WHERE node_id=...
                    ←── 200 { status, progress_pct }
```

### 1.2 为什么复用现有 Weaver worker pool

**现有 Weaver**（`internal/service/weaver.go`）：
- 3 worker goroutine，`FOR UPDATE SKIP LOCKED` 从 `cloud_tasks` 拉任务
- 已对接 LLM Router，调用成功率 > 99%
- 已有 CallRecorder 写 `llm_calls` 表

**Phase 15 方案**：**不复用 Weaver，新建 `AiJobWorker` pool**

理由：
1. Weaver 处理的是 `cloud_tasks`（Condense / Crystallize），绑定了特定业务逻辑；AI 节点触发是独立任务类型
2. 独立 pool 可以独立配置 worker 数（Phase 15 默认 3，可配置）
3. 避免 AI 节点任务排队阻塞 Condense 任务（Weaver 是用户体验关键路径）

**实现复用点**：
- 复用 `internal/llm/router.go`（LLM Provider 路由）
- 复用 `internal/store/repo_llm_calls.go`（写 llm_calls 记录）
- 参照 Weaver `FOR UPDATE SKIP LOCKED` 模式

### 1.3 并发安全（对应反方审查意见 #2）

```sql
-- ai_trigger 接口：INSERT with conflict check
INSERT INTO ai_jobs (id, node_id, lake_id, prompt_template_id, status, ...)
VALUES ($1, $2, $3, $4, 'pending', ...)
ON CONFLICT (node_id) WHERE status IN ('pending', 'processing')
DO NOTHING
RETURNING id;
```

如果 `RETURNING id` 为空 → 409 Conflict（节点已有进行中任务）。

需要在 `ai_jobs(node_id)` 上创建部分唯一索引：
```sql
CREATE UNIQUE INDEX ai_jobs_node_active_idx ON ai_jobs(node_id)
WHERE status IN ('pending', 'processing');
```

### 1.4 进度追踪

LLM 调用目前是同步的（等待完整 response）。Phase 15.1 `progress_pct` 的实现：
- `0%`：pending
- `30%`：worker 已拾取，开始渲染 Prompt
- `60%`：LLM 调用已发出（等待中）
- `100%`：写回 node.content，status=done

Phase 15.2 可升级为 LLM 流式 + SSE，实时更新百分比。

---

## 二、Prompt 模板库架构

### 2.1 模板渲染器

```go
// internal/service/prompt_renderer.go
type PromptRenderer struct {
    MaxOutputLen int // 默认 4000 字符（对应反方审查 #1 长度上限）
}

func (r *PromptRenderer) Render(tmpl string, vars map[string]string) (string, error) {
    // 1. 替换所有 {{var}} 占位符
    // 2. 对每个 var value 做 HTML 转义（防 XSS）
    // 3. 如果最终 Prompt > MaxOutputLen，硬截断并加 "\n[内容已截断]"
    // 4. 返回最终 Prompt 字符串
}
```

**支持的变量**（Phase 15.1）：
```go
vars := map[string]string{
    "node_content":     plainText(targetNode.Content),
    "lake_name":        lake.Name,
    "selected_nodes":   joinNodes(inputNodes, "\n---\n"),
    "user_name":        user.DisplayName,
}
```

**未知变量**：保留原文（不报错），方便用户自定义扩展。

### 2.2 模板数据隔离（对应反方审查 #3 和需求 scope）

```go
// GET /api/v1/prompt_templates 查询逻辑
func (s *PromptTemplateService) ListTemplates(userID, orgID uuid.UUID, scope string) []PromptTemplate {
    // Phase 15.1：只返回 private（created_by = userID）
    // Phase 15.2：+ scope=org 且 org_id = orgID 的记录
    // 平台管理员：全量（分页）
}
```

---

## 三、套餐订阅架构（Stub 支付）

### 3.1 套餐配置（写死 + 环境变量可覆盖）

```go
// internal/config/plans.go
type Plan struct {
    ID            string
    NameZH        string
    PriceCNYMonth int
    Quotas        OrgQuota
}

var BuiltinPlans = map[string]Plan{
    "free": { ... },
    "pro":  { PriceCNYMonth: 29, Quotas: OrgQuota{MaxMembers: 20, MaxLakes: 500, MaxNodes: 100000, MaxStorageMB: 10240} },
    "team": { PriceCNYMonth: 99, Quotas: OrgQuota{MaxMembers: 100, ...} },
}
```

### 3.2 Stub 安全（对应反方审查 #5）

```go
// internal/api/http/handler_subscription.go
func (h *SubscriptionHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
    if !h.cfg.StubPaymentEnabled { // RIPPLE_STUB_PAYMENT=true
        http.Error(w, "real payment not implemented", http.StatusNotImplemented)
        return
    }
    // ... stub 逻辑
}
```

`RIPPLE_STUB_PAYMENT` 默认 `false`。staging `.env` 开启 `true`，生产不开启。

### 3.3 降级校验（422 场景）

```go
func (s *OrgService) ValidateDowngrade(orgID uuid.UUID, targetPlan Plan) error {
    usage, _ := s.GetOrgUsage(orgID)
    if usage.NodeCount > targetPlan.Quotas.MaxNodes {
        return ErrDowngradeBlockedNodes
    }
    // ... 其他字段
    return nil
}
```

---

## 四、LLM 账单架构

### 4.1 llm_calls.org_id 回填策略（对应反方审查 #4）

**不做历史回填**（Phase 15.1）：`org_id` 列 DEFAULT NULL，历史记录为 NULL。

前端 by_provider 聚合 SQL：
```sql
SELECT provider, COUNT(*) as calls, AVG(duration_ms)::int as avg_duration_ms
FROM llm_calls
WHERE org_id = $1
  AND created_at > NOW() - INTERVAL '$2 days'
GROUP BY provider
ORDER BY calls DESC;
```

NULL 数据在账单 UI 中展示为"未归属记录（迁移前）"。

### 4.2 费用估算（写死单价配置）

```go
// internal/config/llm_pricing.go
var ProviderPricing = map[string]float64{
    "zhipu":    0.01, // 元/千 tokens（估算，非合同价）
    "deepseek": 0.008,
    "openai":   0.02,
    "volc":     0.012,
    // ...
}
```

注释中注明"仅估算，非合同价"，前端账单页也显示"估算费用（仅供参考）"。

---

## 五、存储层变更汇总

### 5.1 新增 Repository 接口

```go
// internal/store/repo_ai_job.go
type AiJobRepo interface {
    CreateWithConflictCheck(ctx, job AiJob) (AiJob, bool, error) // bool=true表示创建成功，false表示已存在
    UpdateStatus(ctx, id uuid.UUID, status string, fields ...) error
    GetByNodeID(ctx, nodeID uuid.UUID) (AiJob, error)
    ListPending(ctx, limit int) ([]AiJob, error) // FOR UPDATE SKIP LOCKED
}

// internal/store/repo_prompt_template.go
type PromptTemplateRepo interface {
    Create(ctx, t PromptTemplate) (PromptTemplate, error)
    List(ctx, createdBy uuid.UUID, limit, offset int) ([]PromptTemplate, int, error)
    GetByID(ctx, id uuid.UUID) (PromptTemplate, error)
    Update(ctx, id uuid.UUID, fields PromptTemplateUpdate) error
    Delete(ctx, id uuid.UUID) error
}

// internal/store/repo_subscription.go
type SubscriptionRepo interface {
    UpsertActive(ctx, sub OrgSubscription) (OrgSubscription, error) // INSERT + 旧记录 cancelled
    GetActiveByOrgID(ctx, orgID uuid.UUID) (OrgSubscription, error)
}
```

### 5.2 AiJobWorker（新服务）

```go
// internal/service/ai_job_worker.go
type AiJobWorker struct {
    repo      AiJobRepo
    nodeRepo  NodeRepo      // 写回 node.content
    renderer  PromptRenderer
    router    llm.Router
    recorder  *llm.CallRecorder
    workerN   int           // 默认 3，RIPPLE_AI_WORKER_N 可配置
    pollTick  time.Duration // 默认 2s，RIPPLE_AI_POLL_TICK
}
```

注册到 `cmd/server/main.go` 的 graceful shutdown 组。

---

## 六、反方审查意见回应

| 审查意见 | 架构响应 | 实现位置 |
|---------|---------|---------|
| #1 Prompt 注入 / 长度 | `PromptRenderer.MaxOutputLen=4000` + HTML 转义 | `internal/service/prompt_renderer.go` |
| #2 并发触发 409 | 部分唯一索引 + `ON CONFLICT DO NOTHING RETURNING` | migration 0021 + `repo_ai_job.CreateWithConflictCheck` |
| #3 org_subscriptions 部分唯一索引 | `WHERE status='active'` partial unique index | migration 0022 |
| #4 llm_calls.org_id 历史 NULL | 前端处理 NULL，UI 显示"迁移前记录" | `frontend/src/components/BillingUsage.tsx` |
| #5 stub 支付 feature flag | `RIPPLE_STUB_PAYMENT=true` 环境变量门控 | `internal/config/config.go` + `handler_subscription.go` |

---

## 七、反方审查（第二轮：架构方案本身）

**【反方（审查员）】**：

1. **AiJobWorker 与 Weaver 共享 LLM Router**：两个 worker pool 并发调用同一个 Router（DefaultRouter 无并发保护吗？）→ 确认 `DefaultRouter` 是无状态的 / 线程安全的。

2. **ai_jobs 表 `input_node_ids UUID[]`**：PostgreSQL array 类型，pgx 支持 scan 到 `[]uuid.UUID`，但要注意 NULL vs 空数组的区别。建议：nil → 空数组，在 repo 层处理。

3. **`AiJobWorker` 重启时 `processing` 状态悬挂**：服务重启时，状态为 `processing` 的 job 会永远卡住（不会被 `FOR UPDATE SKIP LOCKED` 重新拾取，因为已被锁定）。需要：启动时将 `processing` 的旧 job 改为 `failed`（error: "server restart"）。

4. **Prompt 长度截断 vs tokens**：`MaxOutputLen=4000` 是字符数，但 LLM API 的限制是 tokens（通常 1 字符 ≈ 0.5-1 token）。Phase 15.1 先用字符数估算可以接受，但需要在代码中留 TODO 注释。

**【正方（架构师）】响应**：

1. `DefaultRouter` 是无状态只读路由（选 provider），Provider 的 HTTP client 也是线程安全的（`sync.Once` init）。✅ 无并发问题。

2. pgx array NULL vs 空数组：在 `repo_ai_job.go` 的 Scan 层做 nil-check，nil → `[]uuid.UUID{}`。✅ 已纳入实现规范。

3. 启动恢复：`AiJobWorker.Start()` 时先执行 `UPDATE ai_jobs SET status='failed', error='server restart' WHERE status='processing'`。✅ 已纳入实现规范。

4. 字符数截断 TODO 已在 `prompt_renderer.go` 注释中标注。✅

---

## 八、QA 验证计划

| 测试类型 | 用例 | 通过条件 |
|---------|------|---------|
| 单元测试 | `PromptRenderer.Render` 各变量替换 + 长度截断 + HTML 转义 | 100% pass |
| 单元测试 | `OrgService.ValidateDowngrade` 超限/不超限两条路径 | 100% pass |
| 集成测试（`-race`） | `POST ai_trigger` 并发 10 次同一 node_id，最终只有 1 个 job | race detector 0 data race |
| 集成测试 | `AiJobWorker` 启动恢复：pre-seed `processing` job → 重启 → 变 `failed` | 状态正确 |
| 集成测试 | 套餐升级：free → pro → quota 更新 → downgrade 422 | 3 个用例全通过 |
| 集成测试 | llm_usage API：pre-seed `llm_calls` 数据 → 响应 by_provider 聚合正确 | 精确匹配 |

---

## 九、体验方评估（Phase 15 架构）

**【体验方（真实用户）】**：

"我理解的流程：选一个节点 → 选一个模板 → 点触发 → 看进度条 → 满意就保存。这个设计 OK，但要注意：

1. 轮询间隔建议 2-3 秒，不要太频繁（浪费流量）也不要太慢（等待感强）。
2. `failed` 的错误信息要对用户友好（不要直接显示 LLM 的 HTTP 错误）。
3. Prompt 模板选择的 UI 要有预览功能（不然用户不知道选哪个）。"

→ 纳入 Phase 15.1 前端 PRD。

---

## 十、PM 验收

**【PM】**：

架构方案与接口设计一致，体验方反馈已纳入前端 PRD，QA 计划覆盖核心路径。

**Phase 15.1 准入条件**（在实现前确认）：
- [ ] migration 0019-0022 SQL 由架构师评审
- [ ] `RIPPLE_STUB_PAYMENT` 环境变量加入 `.env.example`
- [ ] `AiJobWorker` worker 数量加入 `RIPPLE_AI_WORKER_N` 配置

**PM 签字**：✅ 2026-04-28
