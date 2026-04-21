# D7 · AI 服务 Prompt 工程规范

**版本：** v1.1（消化整合版）
**日期：** 2026‑04‑21
**适用对象：** AI 算法工程师 / 后端 / Prompt 调优师
**地位：** 单次 LLM 调用的 Prompt 标准；多 Agent 编排逻辑见 [G5](G5-AI服务编排-织网工作流.md)。

> 本文档由原 `系统 · AI 服务 Prompt 工程规范.md` (v1.0) 消化整合而成。

---

## 一、总则：水文地质学家的思维模式

### 1.1 核心定位

AI 不是通用聊天机器人，而是**水文地质学家** + **文学评论家**。

两大能力：
1. **洞察水流（逻辑分析）**：识别概念间的深层逻辑（因果、层级、对立）
2. **感知波纹（文学修辞）**：识别隐喻、象征、情感色彩

### 1.2 交互原则

- **严禁**直接输出结论
- **必须**输出结构化 JSON
- **必须**遵循"暗流"隐喻：不直接干预用户，只提供潜藏选项

---

## 二、通用 System Prompt

```yaml
role: system
content: |
  你现在是「青萍」创意系统中的高级水文地质学家。
  你的职责是分析用户创建的"灵感节点"，并探测它们之间的"地下暗河"。

  核心任务：
  1. 语义分析：深入理解节点的字面意思和引申义。
  2. 关系探测：判断两个节点之间是否存在逻辑或修辞上的关联。
  3. 结构化输出：你的回答必须严格遵守 JSON 格式，不要包含 Markdown 标记 (```json)，
     也不要说多余的话。

  分析维度（优先级从高到低）：
  - METAPHOR (隐喻):    两者是否在比喻意义上相通？
  - SIMILAR_TO (相似):  两者是否在主题、情感或概念上相似？
  - CAUSES (因果):      A 是否导致 B？
  - PART_OF (组成):     A 是否是 B 的一部分？
  - CONTRASTS (对立):   A 和 B 是否形成鲜明对比？

  如果不存在任何有价值的关联，请返回空数组或 null。
```

---

## 三、核心场景 Prompt 模板

### 3.1 双节点织网

**触发：** 用户手动连接两个节点，或 AI 自动推理。

```yaml
role: user
content: |
  请分析以下两个灵感节点之间的关系：
  节点 A: "{{ node_A_content }}"
  节点 B: "{{ node_B_content }}"

  按以下 JSON Schema 返回：
  {
    "relation_type": "METAPHOR | SIMILAR_TO | CAUSES | CONTRASTS | null",
    "reasoning": "string (≤ 50 字)",
    "strength": "float (0.0~1.0)"
  }
```

### 3.2 单节点生根 / 分蘖

**触发：** 用户点击"分蘖"或"探源"。

```yaml
role: user
content: |
  当前灵感节点："{{ node_content }}"

  请基于此节点发散思考，生成 3 个新的子节点建议：
  1. 其中一个必须是"对立面"（反向思考）
  2. 另外两个必须是"延伸面"（深度挖掘或横向联想）
  3. 每个建议不超过 10 字

  返回 JSON 数组：
  [
    {"content": "...", "type": "contrast"},
    {"content": "...", "type": "extension"},
    {"content": "...", "type": "extension"}
  ]
```

### 3.3 文档炸开

**触发：** 用户导入 PDF/Word/Markdown。

```yaml
role: user
content: |
  以下文本片段来自一篇文档：
  ---
  {{ document_chunk_text }}
  ---

  请提取其中 3-5 个核心概念或关键句。
  忽略过渡性语句和废话。
  返回 JSON 数组：["概念1", "概念2", "概念3"]
```

---

## 四、上下文注入

### 4.1 湖泊上下文

```json
{
  "lake_name": "西湖",
  "lake_theme": "文艺与生活美学"
}
```

效果：玄武湖（历史档案）→ AI 偏向严肃历史类比；西湖 → 偏向浪漫比喻。

### 4.2 邻近节点上下文

```json
{
  "neighbor_nodes": ["风", "水纹", "涟漪", "古琴"]
}
```

效果：帮助 AI 理解当前画布"氛围"，避免格格不入。

---

## 五、输出质量控制

### 5.1 格式清洗

```python
def clean_llm_output(raw_text: str) -> dict:
    text = raw_text.replace("```json", "").replace("```", "")
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        raise ValueError("AI 输出格式错误，非有效 JSON")
```

### 5.2 拒绝回答处理

`relation_type == null` 时**静默处理**，不在前端生成连线，维持"暗流"状态。

---

## 六、模型参数

| 参数 | 推荐 | 原因 |
| :--- | :--- | :--- |
| Temperature | 0.3 ~ 0.5 | 稳定性与逻辑性 |
| Top-P | 0.9 | 留一定创造性 |
| Max Tokens | 500 | 足够 JSON，不浪费 |

> "湍流"模式可临时提升至 Temperature=0.9。

---

## 七、与多 Agent 编排的关系

本文档定义**单次调用**的 Prompt。多 Agent 之间的协作、对抗、降级、Token 预算等编排逻辑详见 [G5 AI 服务编排](G5-AI服务编排-织网工作流.md)。

---

## 八、相关文档

- 多 Agent 编排：[G5](G5-AI服务编排-织网工作流.md)
- 节点 schema：[G1](G1-数据模型与权限设计.md)
- 文档炸开管线：[G4 §四](G4-文件存储与导入流水线.md)

---

**文档状态：** 定稿
**版本来源：** 整合自原 `系统 · AI 服务 Prompt 工程规范.md` (v1.0)，原文件已删除。
