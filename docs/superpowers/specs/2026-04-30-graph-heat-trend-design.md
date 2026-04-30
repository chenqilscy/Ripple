# 图谱热度趋势设计文档

**日期：** 2026-04-30
**归属：** Phase 3-B（知识增长）· 3-B.3（热度趋势）

## 一、概述

### 1.1 目标

在图谱中引入"热度"概念，让用户感知近 7 天内最活跃的知识节点。本周热点通过面板排行和图谱热力标记双模式呈现，帮助用户快速找到自己最近关注最多的想法，以及被持续连接的知识。

### 1.2 核心定义

| 维度 | 权重 | 数据源 | 含义 |
|------|------|--------|------|
| 编辑热度 | 0.6 | `node_revisions` 表 | 节点内容被修改的频率 |
| 关联热度 | 0.4 | `edges` 表 | 节点被建立关联的频率 |

**综合热度公式：**
```
heat_score = editing_score × 0.6 + association_score × 0.4
```

**归一化：** max-min normalization，将原始计数映射到 [0, 1] 区间，便于前端渲染。

**时间窗口：** 近 7 天（不含今天，即当天 00:00:00 之前 7×24 小时）。

### 1.3 设计原则

- **轻量实现**：实时聚合，无需预计算或后台任务
- **渐进增强**：V1 使用现有数据源，V2 增加浏览热度跟踪
- **无侵入**：不影响现有图谱渲染管线，热力标记叠加在现有 LOD 系统之上
- **Graceful degradation**：湖泊节点数 <3 时不展示热点（数据无意义）

---

## 二、数据层

### 2.1 编辑热度

从 `node_revisions` 表查询节点近 7 天内的 revision 数量：

```sql
SELECT node_id, COUNT(*) as edit_count
FROM node_revisions
WHERE lake_id = $lake_id
  AND created_at >= NOW() - INTERVAL '7 days'
GROUP BY node_id
```

### 2.2 关联热度

从 `edges` 表查询节点近 7 天内作为 src 或 dst 的新建边数量：

```sql
SELECT node_id, COUNT(*) as edge_count FROM (
    SELECT src_node_id as node_id FROM edges
    WHERE lake_id = $lake_id
      AND deleted_at IS NULL
      AND created_at >= NOW() - INTERVAL '7 days'
    UNION ALL
    SELECT dst_node_id as node_id FROM edges
    WHERE lake_id = $lake_id
      AND deleted_at IS NULL
      AND created_at >= NOW() - INTERVAL '7 days'
) combined
GROUP BY node_id
```

### 2.3 归一化

在内存中对两个维度的原始计数做 max-min normalization：

```
norm(x) = (x - min) / (max - min)
```

若某维度所有节点计数相同（无区分度），该维度权重在归一化后均分为 0，不影响总分。

---

## 三、后端 API

### 3.1 端点设计

**GET `/api/v1/lakes/{lake_id}/heat-trend`**

返回近 7 天综合热度 top N 的节点列表。

**请求参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | int | 10 | 返回热点节点数量 |

**响应结构：**

```json
{
  "heat_nodes": [
    {
      "node_id": "uuid-xxx",
      "content": "系统可用性 扩容 容错",
      "content_preview": "系统可用性 扩容...",
      "heat_score": 0.85,
      "editing_score": 0.9,
      "association_score": 0.75,
      "edit_count": 12,
      "edge_count": 5,
      "rank": 1
    }
  ],
  "window_days": 7,
  "computed_at": "2026-04-30T10:00:00Z"
}
```

**错误处理：**

| 场景 | HTTP 状态码 | 说明 |
|------|-------------|------|
| lake_id 为空或无效 | 400 | 参数校验失败 |
| lake 不存在 | 404 | 湖泊不存在 |
| lake 内节点数 <3 | 200（空列表） | 数据量过少，不展示热点 |
| 内部错误 | 500 | 返回 `{"error": "..."}` |

### 3.2 Handler 实现

在 `backend-go/internal/api/http/handlers_graph.go` 中新增 `GetHeatTrend` 方法：

1. 从 `node_revisions` 表查询编辑计数
2. 从 `edges` 表查询关联计数
3. 合并两个 map，对每个节点计算综合热度
4. 按热度降序排序，取 top N
5. 构造 `content_preview`（前 50 字符）
6. 返回 JSON 响应

### 3.3 路由注册

在 `router.go` 的图谱分析端点注册段添加：

```go
r.Get("/lakes/{id}/heat-trend", graphH.GetHeatTrend)
```

注意：`GraphAnalysisHandlers` 当前已有 `Nodes` 和 `Edges` 服务注入，足够实现此功能，无需新增依赖。

---

## 四、前端 API 层

### 4.1 API 方法

在 `frontend/src/api/client.ts` 的 `api` 对象中添加：

```typescript
// 热度趋势
getHeatTrend(lakeId: string, limit = 10): Promise<{
  heat_nodes: HeatNode[]
  window_days: number
  computed_at: string
}>
```

类型定义追加到 `frontend/src/api/types.ts`：

```typescript
export interface HeatNode {
  node_id: string
  content: string
  content_preview: string
  heat_score: number    // 0-1，归一化分数
  editing_score: number // 0-1，归一化编辑分
  association_score: number // 0-1，归一化关联分
  edit_count: number
  edge_count: number
  rank: number
}
```

---

## 五、面板展示（DiscoveryPanel）

### 5.1 热力 Tab 设计

在 DiscoveryPanel 中新增「热点」Tab，与现有的「推荐」「探索」Tab 并列：

```
[推荐] [探索] [热点]
```

Tab 点击切换内容区。

### 5.2 热点列表 UI

**布局：** 垂直列表，每项包含：

- **排名徽章**：1-10 数字，左侧显示，Top 3 用主题色高亮
- **内容预览**：节点内容前 40 字符，字体加粗，超长截断
- **热力条**：横向进度条，宽度 = `heat_score × 100%`，颜色从暗到亮渐变（低热灰色，高热橙色）
- **分项指标**（次要信息）：编辑次数 + 关联次数，小字灰色

**空状态：** 当 `heat_nodes` 为空时，显示：

```
本周暂无热点
继续添加想法和关联，知识网络会越来越活跃
```

### 5.3 交互行为

**点击节点项：**
1. 图谱 `LakeGraph` 执行 `panToNode(nodeId)` — 将该节点居中
2. 图谱高亮该节点及其直接关联边（发光效果，持续 3 秒后淡出）
3. 面板保持展开状态

**Hover 节点项：**
- 背景色变浅
- 鼠标变为 `pointer`

### 5.4 技术实现

在 `frontend/src/components/graph/DiscoveryPanel.tsx` 中：

1. 新增 `activeTab` state，默认 `'recommendations'`
2. Tab 栏增加「热点」Tab
3. 新增 `HeatList` 子组件（可内联或拆分）
4. `useEffect` 监听 Tab 切换到「热点」时，调用 `api.getHeatTrend(lakeId)`
5. `HeatList` 渲染 `heat_nodes` 列表

---

## 六、图谱热力标记（LakeGraph）

### 6.1 叠加层设计

热力标记作为叠加层（overlay），不影响现有 LOD 和 Web Worker 渲染管线。具体方案：

**方案 A（推荐）：在现有节点渲染上叠加热力效果**

在 `LakeGraph.tsx` 中，`SpringNodes` 组件渲染节点时，根据节点 `heat_score` 调整：

- **大小缩放**：`baseRadius × (1 + heat_score × 0.3)` — 热节点略大
- **亮度叠加**：在 LOD "normal" 模式下，通过 `emissive` 叠加橙色光晕
- **边框颜色**：热节点边框从灰色切换为橙色（`#f59e0b`）

**LOD 层级的热力处理：**

| LOD 层级 | 节点形状 | 热力表现 |
|---------|---------|---------|
| `demand`（极远） | 2D 圆形 | 无热力，灰色 |
| `simple`（远） | 简化 mesh | 热力仅通过颜色深浅体现 |
| `normal`（近） | 带内容文字 mesh | 颜色 + 边框 + 微弱发光 |

### 6.2 热力数据注入

`LakeGraph` 组件新增可选 prop：

```typescript
interface HeatNode {
  node_id: string
  heat_score: number // 0-1
}

interface LakeGraphProps {
  // ... existing props
  heatNodes?: HeatNode[]  // 新增：热度数据
  onNodeClick?: (nodeId: string) => void
}
```

### 6.3 热力与 LOD 的叠加逻辑

```
// 在 SpringNodes 的节点渲染循环中
const heatScore = heatNodeMap.get(nodeId)?.heat_score ?? 0
const radiusMultiplier = 1 + heatScore * 0.3
const glowColor = heatScore > 0.5 ? '#f59e0b' : undefined
```

### 6.4 图谱与面板联动

当用户在面板「热点」Tab 点击某节点时：
- 调用 `panToNode(nodeId)` 高亮并居中
- 高亮效果：`highlightedNodeId` state，持续 3 秒后清除

---

## 七、热力颜色系统

### 7.1 热力梯度

| 分数区间 | 颜色 | 含义 |
|---------|------|------|
| 0.0 - 0.3 | `#6b7280`（灰色） | 冷节点 |
| 0.3 - 0.6 | `#f59e0b`（橙色） | 温热 |
| 0.6 - 0.8 | `#ef4444`（红色） | 热门 |
| 0.8 - 1.0 | `#dc2626`（深红 + 发光） | 极热 |

### 7.2 CSS 变量

在全局 CSS 中定义：

```css
:root {
  --heat-cold: #6b7280;
  --heat-warm: #f59e0b;
  --heat-hot: #ef4444;
  --heat-extreme: #dc2626;
  --heat-glow: rgba(245, 158, 11, 0.4);
}
```

---

## 八、数据流总览

```
用户打开「热点」Tab
        │
        ▼
DiscoveryPanel 调用 api.getHeatTrend(lakeId)
        │
        ▼
后端 GET /api/v1/lakes/{lake_id}/heat-trend
        │
        ├── 查询 node_revisions（近7天编辑计数）
        ├── 查询 edges（近7天关联计数）
        ├── 归一化 + 加权计算 heat_score
        └── 返回 top 10 heat_nodes
        │
        ▼
DiscoveryPanel 渲染「热点」Tab 列表
        │
        ▼
同时将 heatNodes 传入 LakeGraph
        │
        ▼
LakeGraph 在节点渲染中叠加热力效果
        │
        ▼
用户点击某热点节点 → panToNode + highlight
```

---

## 九、自检清单

- [ ] 规格覆盖：API 端点、后端 handler、面板列表、图谱热力标记全部覆盖
- [ ] 无占位符：所有字段名、颜色值、权重数值均已明确
- [ ] 类型一致：`HeatNode` 接口与后端响应结构一致
- [ ] LOD 兼容：热力标记在 demand/simple/normal 三个 LOD 层级均有合理表现
- [ ] 空状态处理：节点数 <3 或无热点时展示空状态提示
- [ ] 联动行为：面板点击能正确定位图谱节点

---

## 十、后续规划

| 版本 | 内容 |
|------|------|
| **V1（本版本）** | 编辑热度 + 关联热度，面板排行 + 图谱热力标记 |
| **V2** | 新增浏览热度（节点浏览事件跟踪），公式变为编辑×0.5 + 关联×0.3 + 浏览×0.2 |
| **V3** | 热度趋势历史对比（本周 vs 上周），增量变化指示器 |
