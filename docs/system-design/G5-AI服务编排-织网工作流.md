# G5 · AI 服务编排（织网工作流）

**版本：** v1.0
**日期：** 2026‑04‑21
**适用对象：** AI 算法 / 后端
**地位：** 升级 [AI Prompt 工程规范](./系统%20·%20AI%20服务%20Prompt%20工程规范.md)：从单次调用 → 多 Agent 编排。借鉴"青萍与马具"反思的红蓝军对抗模式。

---

## 一、设计哲学

> "如果让一个 AI 既当运动员又当裁判，结果往往是平庸的妥协。"

把 LLM 拆成**四个角色**，让它们彼此打磨、对抗、收敛：

| Agent | 职责 | 类比 | 模型推荐 |
| :--- | :--- | :--- | :--- |
| **织网者 (Weaver)** | 探测节点间的暗流（关系判定） | 主笔 | GPT-4o / Claude Sonnet |
| **探源者 (Diver)** | 向量召回 + 跨湖检索，给织网者提供上下文 | 研究员 | 不调 LLM，纯向量+图查询 |
| **沉淀者 (Curator)** | 把生成结果转为最终结构化输出 | 主编 | GPT-4o-mini |
| **评审员 (Critic)** | 红蓝军，给织网者打分 / 拒绝 / 重写 | 编辑 | Claude Haiku（成本优先） |

---

## 二、典型工作流：节点凝露后的"织网"

```
         [Node 凝露事件]
                │
                ▼
     ┌──────────────────────┐
     │  Diver               │  ← 召回 top-K 邻居 (向量+标签+时间)
     │  - 同湖近邻 5         │
     │  - 跨湖公开/团队 3    │
     │  - 同作者历史 2       │
     └──────────┬───────────┘
                │ candidates
                ▼
     ┌──────────────────────┐
     │  Weaver  (round 1)   │  ← 给候选打分，输出 RELATES_TO 提议
     └──────────┬───────────┘
                │ proposals
                ▼
     ┌──────────────────────┐
     │  Critic              │  ← 评审：合格 / 弱 / 拒绝
     └──────────┬───────────┘
                │
        ┌───────┴────────┐
        │ 合格 ≥ 2        │ 全弱
        ▼                ▼
     ┌──────────────────────┐
     │  Weaver (round 2)    │  ← 仅在全弱时重写 (max 1 轮)
     └──────────┬───────────┘
                ▼
     ┌──────────────────────┐
     │  Curator             │  ← 写入 Neo4j；通过 WS 推送
     └──────────────────────┘
```

**最大对抗轮次：** 2（成本控制）。Weaver round 2 仍未通过则降级为"潜在暗流"（虚线显示）。

---

## 三、Agent 职责详解

### 3.1 Weaver（织网者）

System Prompt 沿用 [AI Prompt 规范 §二](./系统%20·%20AI%20服务%20Prompt%20工程规范.md)，但增加：
- 上下文注入：候选节点列表、湖泊主题、用户最近 5 个节点
- 输出 schema：批量 JSON `[{target_id, relation_type, reasoning, strength}]`

### 3.2 Diver（探源者）

无 LLM 调用，纯检索逻辑：
```python
def dive(node, k_same=5, k_cross=3, k_history=2):
    same_lake = vector_search(lake_id=node.lake_id, top=k_same)
    cross_lake = vector_search(
        lake_filter=lambda l: l.visibility != 'PRIVATE'
                              or user_in_lake(node.created_by, l),
        top=k_cross
    )
    history = recent_nodes(user_id=node.created_by, top=k_history)
    return dedupe_by_id(same_lake + cross_lake + history)
```

### 3.3 Critic（评审员）

System Prompt:
```yaml
你是「青萍」AI 织网评审员。你的职责是审视织网者输出的暗流提议，剔除以下劣质关系：
1. 牵强附会（仅词面相似但语义无关）
2. 同义重复（与已有 RELATES_TO 类型重叠）
3. 隐喻陈词（"水"和"流"硬套）

对每条提议输出 JSON：
{ "id": "...", "verdict": "PASS|WEAK|REJECT", "reason": "..." }

阈值：strength < 0.6 直接 REJECT；reasoning 不足 15 字 WEAK。
```

### 3.4 Curator（沉淀者）

把通过评审的提议持久化：
- Neo4j 创建 `[:RELATES_TO]` 关系
- 标注 `source = 'AI'`、`weight = strength`、`reasoning = ...`
- WebSocket 推送 `EDGE_CREATE` 事件

---

## 四、其他工作流场景

| 场景 | Agent 链 |
| :--- | :--- |
| 单节点生根 (分蘖) | Diver → Weaver → Critic → Curator |
| 双节点用户引流 | Weaver (单次) → Curator |
| 文档炸开 | Splitter (规则) → Weaver (并发批量) → Critic (抽样) → Curator |
| 探源查询 | Diver only（不需生成） |
| 湍流（随机扰动） | Weaver (高 temperature) → Critic (放宽) → Curator |

---

## 五、降级链路（与故事 9 对齐）

| 状态 | Weaver 可用 | Critic 可用 | 行为 |
| :--- | :-: | :-: | :--- |
| **在线** | ✅ | ✅ | 完整工作流 |
| **半在线**（Critic 故障） | ✅ | ❌ | 跳过 Critic，Weaver 输出全部以**潜在暗流**（虚线）落库 |
| **半在线**（Weaver 故障，仅 Embedding 可用） | ❌ | — | "人工降雨"模式：仅 Diver 检索，无 AI 生成 |
| **离线** | ❌ | ❌ | 全部 AI 按钮置灰；本地 fallback 词库支持初次 onboarding |
| **恢复** | ✅ | ✅ | 触发**追溯补建**（含限流，详见故事 9 硬约束） |

---

## 五·补 · 输出强校验（MVP M1 必做）

所有 LLM 输出必须经过 **JSON Schema 严格校验**，违规处理策略：

```python
def call_llm_with_schema(prompt: str, schema: dict, max_retries: int = 1) -> dict:
    for attempt in range(max_retries + 1):
        raw = llm.invoke(prompt)
        try:
            data = json.loads(strip_codeblock(raw))
            jsonschema.validate(data, schema)
            return data
        except (json.JSONDecodeError, jsonschema.ValidationError) as e:
            if attempt < max_retries:
                prompt = prompt + f"\n\n上一次输出格式错误：{e}。请严格按 Schema 返回。"
                continue
            log_ai_failure(prompt, raw, e)
            return None  # 静默丢弃，不在前端生成连线
```

约束：
- 失败重试上限 **1 次**；二次失败必须静默丢弃，不允许向前端推送脏数据
- 校验失败计数纳入 [G6 Metrics](G6-可观测性与监控.md) `ripple_ai_schema_violations_total`
- 长期失败率 > 5% 触发 Prompt 调优工单

## 六、Token 预算与成本

| 项 | 预算 |
| :--- | :--- |
| 单节点凝露的总 Token | ≤ 3K input + 500 output |
| 单文档炸开（10 页） | ≤ 30K input + 5K output |
| 用户日 AI 调用上限（免费层） | 100 次 |
| 用户日上限（付费层） | 5000 次 |
| 团队湖月度上限 | 见 SaaS 套餐 |

成本看板：实时统计 input/output token，按 user/lake 维度归集，超阈值告警。

---

## 七、事件驱动统一（修复 R3.1 #4）

收敛之前文档的 AI 触发时机不一致：

```
[任意写操作]
    │
    ▼
[Outbox Pattern]
  - 同事务写入 outbox 表
  - 不直接调 AI
    │
    ▼
[Outbox Worker]
  - 转发到 Kafka / Redis Stream
    │
    ▼
[AI Workflow Consumer]
  - 触发上述 Agent 编排
  - 完成后 WebSocket 推送
```

**好处：** AI 调用与业务事务解耦；失败可重试；流量可削峰。

---

## 八、可观测性

每条 AI 调用必须埋点：
```json
{
  "trace_id": "...",
  "workflow": "weave_on_condense",
  "agent": "weaver",
  "model": "gpt-4o",
  "lake_id": "...",
  "node_id": "...",
  "input_tokens": 1234,
  "output_tokens": 456,
  "latency_ms": 890,
  "verdict": "PASS|WEAK|REJECT"
}
```

接 OpenTelemetry，看板见 [G6](./G6-可观测性与监控.md)。

---

## 九、与其他文档的关系

- 单次 Prompt 模板：[AI Prompt 工程规范](./系统%20·%20AI%20服务%20Prompt%20工程规范.md)
- 节点状态触发：[G1 §三状态机](./G1-数据模型与权限设计.md)
- 凝露事件源：[G2 §五](./G2-云霓-灵感采集模块设计.md)
- 文档炸开：[G4 §四](./G4-文件存储与导入流水线.md)
- 降级故事：[场景 9 潮汐异常](../user-story/story.md#场景九潮汐异常--ai-罢工时的体面)

---

**文档状态：** 待反方二次审查
**下一步：** 更新 README 文档地图，进入 PM 验收
