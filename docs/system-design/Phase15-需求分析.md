# Phase 15 · 需求分析文档

> 状态：📝 需求分析阶段（2026-04-28）
> 范围：候选 C（AI Workflow 扩展）+ 候选 D（商业化与计量）
> 流程位置：需求创造师 → PM 评估（等待接口设计师）
> 配套：docs/launch/Phase15-立项调研.md

---

## 十角色讨论记录

### 需求创造师

**C · AI Workflow 扩展**

Ripple 当前已有 LLM 调用能力（6 家 Provider、Weaver worker pool），但 AI 功能仅在「Condense 编织」一处可见——用户无法感知 AI 在帮他做什么，也无法主动触发。真正的 AI 应用痛点是：

1. **"我选了几个节点，想让 AI 帮我整理，但不知道怎么操作"**——AI 能力入口不可发现
2. **"AI 生成的内容我想改，但改完下次又被覆盖"**——没有可控的 AI 节点类型
3. **"我想重复用某个 Prompt，但每次都要重写"**——没有 Prompt 模板库

核心需求：**让用户主动触发 AI、看到 AI 的过程、控制 AI 的输出**。

**D · 商业化与计量**

Phase 14 已有 `org_quotas` 表和计量基础，但没有"付钱"路径。真正的商业化阻塞是：

1. **"我用完配额了，但不知道怎么升级"**——没有套餐选择 + 支付路径
2. **"我不知道自己用了多少 LLM 调用，也不知道会不会超支"**——`llm_calls` 表有数据但无 UI
3. **"团队多人用，但只有一个账单地址"**——Org 级账单尚未设计

核心需求：**订阅套餐选择 → 支付 → 配额解锁 → 用量账单 → 续费提醒**。

---

### PM 评估（优先级与边界）

**Phase 15 边界声明**：

| 范围 | 纳入 Phase 15 | 排除 |
|------|-------------|------|
| C：AI Workflow | AI 节点类型（触发 AI、查看结果）+ Prompt 模板库（创建/保存/使用）+ 多节点 AI 整理 | AI Agent（自主跨节点推理）、LLM Router 可视化（Phase 16） |
| D：商业化 | 套餐页面（免费/专业/团队）+ Stripe/国内支付 Stub + org_quotas 升级 API + LLM 调用账单 UI | 真实支付网关接入（需法务）、发票系统、欠费处理自动化 |

**PM 价值评估**：

- C 的核心价值：产品差异化，提升留存（用了 AI 功能的用户 DAU 预计高 2-3x）
- D 的核心价值：验证 PMF，开始有收入（哪怕 Stub 支付，也能量化转化率）
- C + D 联动：AI 功能是付费升级的"诱饵"——高用量 AI 用户最有动力升级套餐

**优先级建议**：

1. C.1 AI 节点类型（前端触发入口 + 后端 trigger API）
2. D.1 套餐页面 + `org_quotas` 升级接口
3. C.2 Prompt 模板库
4. D.2 LLM 调用账单 UI（关联 `llm_calls` 表）
5. C.3 多节点 AI 整理（选多个节点 → AI 生成摘要 / 连线）

---

### AI 应用专家评估

Phase 15 的 AI Workflow 必须解决**可控性**问题，否则用户会失去信任：

- AI 节点需要有"重新生成"按钮（用户不满意可以重跑）
- Prompt 模板库需要支持变量占位符（`{{node_content}}`、`{{lake_name}}`）
- 多节点整理的结果要以 diff 形式展示（让用户决定是否接受），不能直接覆盖

---

### 体验方（真实用户视角）

"我最想要的功能：选几个节点，告诉 AI 我的意图（比如'帮我整理成学习笔记'），AI 直接给我一个新节点。整个过程我能看到进度，结果不满意能重试。"

---

## 需求规格（Phase 15 C+D）

### C.1 AI 节点类型

**用户故事**：作为用户，我可以在湖中创建一个 AI 节点，选择一个 Prompt 模板，AI 自动填充内容，我可以查看生成过程，满意后保存。

**功能边界**：
- 新增节点类型 `ai_generated`（字段：`prompt_template_id`、`ai_status: pending|processing|done|failed`、`ai_input_node_ids`）
- `POST /api/v1/lakes/{lake_id}/nodes/{id}/ai_trigger`：触发 AI 填充
- `GET /api/v1/lakes/{lake_id}/nodes/{id}/ai_status`：轮询状态（或 SSE）
- 前端：AI 节点展示 spinner → 结果 → 重新生成按钮

**验收标准**：
- 创建 AI 节点，触发，5s 内状态变为 `processing`，30s 内变为 `done`，节点内容已更新
- `failed` 时展示错误原因，提供重试按钮
- 并发 10 个 AI 节点触发，全部最终完成（利用现有 Weaver worker pool）

---

### C.2 Prompt 模板库

**用户故事**：作为用户，我可以创建和保存 Prompt 模板，在创建 AI 节点时从模板列表中选择。

**功能边界**：
- 新增表 `node_templates`（已存在 migration？需确认）+ `prompt_templates`（新）
- `GET/POST /api/v1/prompt_templates`，`DELETE /api/v1/prompt_templates/{id}`
- 模板支持变量：`{{node_content}}`、`{{lake_name}}`、`{{selected_nodes}}`
- 前端：模板管理页面（Settings 新增 Tab）+ AI 节点选择模板下拉

**验收标准**：
- CRUD 通过集成测试
- 模板变量替换正确（单元测试覆盖 3 种变量）
- 10 个 Prompt 模板列表加载 < 500ms

---

### D.1 套餐页面与配额升级

**用户故事**：作为用户，我可以查看当前套餐，选择升级，配额立即生效（Stub 支付，不接真实网关）。

**功能边界**：
- 新增套餐配置（写死 3 个套餐：免费 / 专业 ¥29/月 / 团队 ¥99/月）
- `POST /api/v1/organizations/{id}/subscription`：提交套餐选择（Stub 直接成功）
- `PATCH /api/v1/organizations/{id}/quota`（已有）：后端根据套餐更新 `org_quotas`
- 前端：Billing Tab（Settings 新增），展示当前套餐 + 升级按钮 + "Stub 支付" 页

**验收标准**：
- 免费 → 专业升级后，`max_nodes` 由 10000 变为 100000
- Stub 支付页显示订单摘要，点击确认后跳转回 Settings，配额已更新
- 降级：专业 → 免费时，若当前用量超出免费限额，返回 422 并提示超出项

---

### D.2 LLM 调用账单 UI

**用户故事**：作为 Org 管理员，我可以查看本组织近 30 天的 LLM 调用次数 / 费用估算，知道哪些操作消耗了 AI 额度。

**功能边界**：
- `GET /api/v1/organizations/{id}/llm_usage?days=30`：聚合 `llm_calls` 表（按 provider / 按日聚合）
- 前端：Billing Tab 增加"AI 用量"子页，表格 + 折线图
- 费用估算：按 provider 单价（配置文件写死，Phase 16 再做可配置化）

**验收标准**：
- 30 天内调用记录正确聚合（单元测试 + 集成测试）
- UI 表格展示 provider / call_count / estimated_cost
- 折线图按日展示趋势

---

## 非功能需求

| 项 | 要求 |
|----|------|
| 性能 | AI trigger API 响应 < 200ms（触发即返回，异步执行） |
| 安全 | Prompt 模板内容做 XSS 清洗（HTML 转义）；AI 输出写入 node 前做内容长度校验 |
| 数据隔离 | Prompt 模板按 user_id 隔离（私有）或 org_id 共享（需明确选择） |
| 兼容性 | 不破坏现有 REST API；新增接口版本不变（/api/v1/） |
| 测试 | 所有新 handler 必须有单元测试；AI trigger 异步路径需 race test |

---

## 下一步（等待十角色继续）

- [ ] **接口设计师**：设计 `ai_trigger` / `prompt_templates` / `subscription` / `llm_usage` 4 组 API 的请求/响应模型、错误码、分页策略
- [ ] **正方（架构师）**：提出 AI 节点异步执行方案（是否复用 Weaver、或新 worker pool）+ 套餐配置存储方案
- [ ] **反方（审查员）**：审查 AI 节点并发、Prompt 注入安全风险、Stub 支付合规边界
- [ ] **决策者裁决**：确认 C+D 边界 + Sprint 排期（建议 Phase 15.1 = C.1 + D.1，共 3 周）
