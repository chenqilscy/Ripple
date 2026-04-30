# 图谱热度趋势实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现"本周热点"功能，近 7 天综合热度 top 10 节点在面板排行和图谱热力标记双模式呈现。

**Architecture:**
- 后端：新增 `GetHeatTrend` handler，接入 `NodeRevisionRepository` 和 `EdgeService`，实时聚合计算热度分数
- 前端：DiscoveryPanel 新增「热点」Tab；LakeGraph 叠加热力发光效果（不改动现有 LOD 管线）
- 热力公式：综合热度 = 编辑热度×0.6 + 关联热度×0.4，归一化到 [0,1]

**Tech Stack:** Go chi router + pgx / React Three Fiber + TypeScript

---

## 一、文件结构

```
backend-go/internal/
├── store/
│   └── repo_node_revision.go          # 修改：新增 CountByNodeIDsSince
├── service/
│   └── heat.go                       # 创建：热度计算服务
├── api/http/
│   ├── handlers_graph.go             # 修改：新增 GetHeatTrend handler
│   └── router.go                     # 修改：注册 /heat-trend 路由

frontend/src/
├── api/
│   ├── types.ts                      # 修改：新增 HeatNode 类型
│   └── client.ts                     # 修改：新增 getHeatTrend 方法
└── components/
    ├── LakeGraph.tsx                 # 修改：heatNodes prop + 热力渲染
    └── graph/
        └── DiscoveryPanel.tsx        # 修改：新增「热点」Tab
```

---

## 二、任务详情

### Task 1: 后端 — 热力计算 + API 端点

**Files:**
- Modify: `backend-go/internal/store/repo_node_revision.go`
- Create: `backend-go/internal/service/heat.go`
- Modify: `backend-go/internal/api/http/handlers_graph.go`
- Modify: `backend-go/internal/api/http/router.go`

#### 步骤 1：新增 NodeRevisionRepository 方法

编辑 `backend-go/internal/store/repo_node_revision.go`，在文件末尾添加：

```go
// CountByNodeIDsSince 返回指定节点列表中，每个节点在 since 之后的 revision 数量。
// 返回 map[nodeID]count。
func (r *nodeRevRepoPG) CountByNodeIDsSince(ctx context.Context, nodeIDs []string, since time.Time) (map[string]int, error) {
    if len(nodeIDs) == 0 {
        return map[string]int{}, nil
    }
    query := `
        SELECT node_id, COUNT(*) as cnt
        FROM node_revisions
        WHERE node_id = ANY($1) AND created_at >= $2
        GROUP BY node_id
    `
    rows, err := r.pool.Query(ctx, query, nodeIDs, since)
    if err != nil {
        return nil, fmt.Errorf("count revisions by node ids since: %w", err)
    }
    defer rows.Close()
    out := make(map[string]int, len(nodeIDs))
    for rows.Next() {
        var nodeID string
        var cnt int
        if err := rows.Scan(&nodeID, &cnt); err != nil {
            return nil, fmt.Errorf("scan revision count: %w", err)
        }
        out[nodeID] = cnt
    }
    return out, rows.Err()
}
```

注意：在文件开头的 import 中已有 `"time"` 和 `"fmt"`，确认这两行存在。

#### 步骤 2：创建热度计算服务

创建 `backend-go/internal/service/heat.go`：

```go
package service

import (
    "context"
    "math"
    "time"

    "github.com/chenqilscy/ripple/backend-go/internal/domain"
)

// HeatService 计算知识节点的热度。
type HeatService struct {
    nodeRevRepo interface {
        CountByNodeIDsSince(ctx context.Context, nodeIDs []string, since time.Time) (map[string]int, error)
    }
    edgeRepo interface {
        ListByLake(ctx context.Context, actor *domain.User, lakeID string, includeDeleted bool) ([]domain.Edge, error)
    }
}

// NewHeatService 构造。
func NewHeatService(
    nodeRevRepo interface {
        CountByNodeIDsSince(ctx context.Context, nodeIDs []string, since time.Time) (map[string]int, error)
    },
    edgeRepo interface {
        ListByLake(ctx context.Context, actor *domain.User, lakeID string, includeDeleted bool) ([]domain.Edge, error)
    },
) *HeatService {
    return &HeatService{
        nodeRevRepo: nodeRevRepo,
        edgeRepo:    edgeRepo,
    }
}

// HeatNode 单个热度节点。
type HeatNode struct {
    NodeID             string
    Content            string
    ContentPreview     string
    HeatScore          float64 // 归一化 [0,1]
    EditingScore       float64
    AssociationScore   float64
    EditCount          int
    EdgeCount          int
    Rank               int
}

// ComputeHeat 计算 lakeID 下所有节点近 7 天热度。
func (s *HeatService) ComputeHeat(ctx context.Context, actor *domain.User, lakeID string, limit int) ([]HeatNode, error) {
    since := time.Now().UTC().AddDate(0, 0, -7)

    // 获取所有活跃节点
    nodes, err := s.nodeRevRepo.ListByLake(ctx, actor, lakeID, true)
    if err != nil {
        return nil, err
    }

    // 过滤活跃节点（排除 ERASED / GHOST）
    var activeNodes []domain.Node
    for _, n := range nodes {
        if n.State != domain.StateErased && n.State != domain.StateGhost {
            activeNodes = append(activeNodes, n)
        }
    }
    if len(activeNodes) < 3 {
        return []HeatNode{}, nil
    }

    nodeIDs := make([]string, len(activeNodes))
    nodeMap := make(map[string]string, len(activeNodes))
    for i, n := range activeNodes {
        nodeIDs[i] = n.ID
        content := n.Content
        if len(content) > 50 {
            content = content[:50] + "..."
        }
        nodeMap[n.ID] = content
    }

    // 编辑热度：查询 node_revisions
    editCounts, err := s.nodeRevRepo.CountByNodeIDsSince(ctx, nodeIDs, since)
    if err != nil {
        return nil, err
    }

    // 关联热度：查询 edges
    edges, err := s.edgeRepo.ListByLake(ctx, actor, lakeID, false)
    if err != nil {
        return nil, err
    }

    edgeCounts := make(map[string]int)
    for _, e := range edges {
        if e.DeletedAt != nil {
            continue
        }
        if e.CreatedAt.Before(since) {
            continue
        }
        edgeCounts[e.SrcNodeID]++
        edgeCounts[e.DstNodeID]++
    }

    // 构建节点热度列表（原始分数）
    type rawScore struct {
        editing float64
        assoc   float64
    }
    raw := make(map[string]rawScore)
    for _, nid := range nodeIDs {
        raw[nid] = rawScore{
            editing: float64(editCounts[nid]),
            assoc:   float64(edgeCounts[nid]),
        }
    }

    // max-min 归一化
    maxEdit, maxAssoc := 1.0, 1.0
    for _, v := range raw {
        if v.editing > maxEdit {
            maxEdit = v.editing
        }
        if v.assoc > maxAssoc {
            maxAssoc = v.assoc
        }
    }

    type scoredNode struct {
        nodeID  string
        content string
        editing float64
        assoc   float64
        heat    float64
    }
    var scored []scoredNode
    for _, nid := range nodeIDs {
        eScore := 0.0
        aScore := 0.0
        if maxEdit > 0 {
            eScore = raw[nid].editing / maxEdit
        }
        if maxAssoc > 0 {
            aScore = raw[nid].assoc / maxAssoc
        }
        heat := eScore*0.6 + aScore*0.4
        scored = append(scored, scoredNode{
            nodeID:  nid,
            content: nodeMap[nid],
            editing: eScore,
            assoc:   aScore,
            heat:    heat,
        })
    }

    // 按热度降序排序
    for i := 0; i < len(scored); i++ {
        for j := i + 1; j < len(scored); j++ {
            if scored[j].heat > scored[i].heat {
                scored[i], scored[j] = scored[j], scored[i]
            }
        }
    }

    if limit <= 0 {
        limit = 10
    }
    if limit > len(scored) {
        limit = len(scored)
    }

    out := make([]HeatNode, limit)
    for i := 0; i < limit; i++ {
        out[i] = HeatNode{
            NodeID:           scored[i].nodeID,
            Content:          scored[i].content,
            ContentPreview:   scored[i].content,
            HeatScore:        math.Round(scored[i].heat*100) / 100,
            EditingScore:     math.Round(scored[i].editing*100) / 100,
            AssociationScore: math.Round(scored[i].assoc*100) / 100,
            EditCount:        editCounts[scored[i].nodeID],
            EdgeCount:        edgeCounts[scored[i].nodeID],
            Rank:             i + 1,
        }
    }

    return out, nil
}
```

注意：`NewHeatService` 依赖 `NodeRevisionRepository` 和 `NodeService`（后者实现了 `ListByLake`）。在 NodeService 中查找 `ListByLake` 方法签名确认 `actor` 参数类型。

#### 步骤 3：新增 GetHeatTrend handler

编辑 `backend-go/internal/api/http/handlers_graph.go`，在文件末尾添加：

```go
// GetHeatTrend GET /api/v1/lakes/{lake_id}/heat-trend
func (h *GraphAnalysisHandlers) GetHeatTrend(w http.ResponseWriter, r *http.Request) {
    u, _ := CurrentUser(r.Context())
    lakeID := chi.URLParam(r, "lake_id")

    limit := 10
    if l := r.URL.Query().Get("limit"); l != "" {
        if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
            limit = parsed
        }
    }

    nodes, err := h.Heat.ComputeHeat(r.Context(), u, lakeID, limit)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to compute heat: "+err.Error())
        return
    }

    type heatNodeRes struct {
        NodeID             string  `json:"node_id"`
        Content            string  `json:"content"`
        ContentPreview     string  `json:"content_preview"`
        HeatScore          float64 `json:"heat_score"`
        EditingScore       float64 `json:"editing_score"`
        AssociationScore   float64 `json:"association_score"`
        EditCount          int     `json:"edit_count"`
        EdgeCount          int     `json:"edge_count"`
        Rank               int     `json:"rank"`
    }
    res := make([]heatNodeRes, len(nodes))
    for i, n := range nodes {
        preview := n.Content
        if len(preview) > 50 {
            preview = preview[:50] + "..."
        }
        res[i] = heatNodeRes{
            NodeID:           n.NodeID,
            Content:          n.Content,
            ContentPreview:   preview,
            HeatScore:        n.HeatScore,
            EditingScore:     n.EditingScore,
            AssociationScore: n.AssociationScore,
            EditCount:        n.EditCount,
            EdgeCount:        n.EdgeCount,
            Rank:             n.Rank,
        }
    }

    writeJSON(w, http.StatusOK, map[string]any{
        "heat_nodes":  res,
        "window_days": 7,
        "computed_at":  time.Now().Format(time.RFC3339),
    })
}
```

在文件顶部 import 中添加 `"strconv"`（如果尚不存在）。

#### 步骤 4：注入 HeatService

编辑 `backend-go/internal/api/http/handlers_graph.go`，修改 `GraphAnalysisHandlers` 结构体：

```go
type GraphAnalysisHandlers struct {
    Nodes      *service.NodeService
    Edges      *service.EdgeService
    Recommender *service.RecommenderService
    Heat       *service.HeatService // 新增
}
```

#### 步骤 5：注册路由

编辑 `backend-go/internal/api/http/router.go`，找到图谱分析端点注册段（around line 208-221）：

```go
if d.Nodes != nil && d.Edges != nil {
    graphH := &GraphAnalysisHandlers{Nodes: d.Nodes, Edges: d.Edges, Recommender: d.Recommender}
    // 新增一行：
    graphH.Heat = service.NewHeatService(d.NodeRevisions, d.Nodes)
```

注意：`Deps` 结构体中需要添加 `NodeRevisions store.NodeRevisionRepository` 字段。找到 `Deps` 定义，在 `Edges` 字段附近添加：

```go
NodeRevisions store.NodeRevisionRepository
```

同时在 `NewRouter` 的 `graphH` 初始化处添加 `Heat` 字段。

#### 步骤 6：Go 编译检查

```bash
cd backend-go && go build ./...
```

预期：无编译错误。如果有 `undefined: strconv`，在 `handlers_graph.go` import 添加 `"strconv"`。如果有 `d.NodeRevisions undefined`，在 `Deps` 结构体中添加该字段。

#### 步骤 7：提交

```bash
git add backend-go/internal/store/repo_node_revision.go
git add backend-go/internal/service/heat.go
git add backend-go/internal/api/http/handlers_graph.go
git add backend-go/internal/api/http/router.go
git commit -m "feat(graph): add heat trend API — weekly top hot nodes by editing+association"
```

---

### Task 2: 前端 — API 类型 + client 方法

**Files:**
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/api/client.ts`

#### 步骤 1：添加 HeatNode 类型

编辑 `frontend/src/api/types.ts`，在文件末尾（`export * from './types'` 之前）添加：

```typescript
// ---- 图谱热度趋势 (Phase 3-B.3) ----
export interface HeatNode {
  node_id: string
  content: string
  content_preview: string
  heat_score: number    // 0-1
  editing_score: number  // 0-1
  association_score: number // 0-1
  edit_count: number
  edge_count: number
  rank: number
}
```

#### 步骤 2：添加 getHeatTrend API 方法

编辑 `frontend/src/api/client.ts`，在 `api` 对象的图谱价值增强区域（找到 `getRecommendations` 方法附近）添加：

```typescript
// 热度趋势
getHeatTrend(lakeId: string, limit = 10): Promise<{
  heat_nodes: HeatNode[]
  window_days: number
  computed_at: string
}> {
  return request('GET', `/api/v1/lakes/${lakeId}/heat-trend?limit=${limit}`)
},
```

#### 步骤 3：TypeScript 编译检查

```bash
cd frontend && npx tsc --noEmit
```

预期：无编译错误。

#### 步骤 4：提交

```bash
git add frontend/src/api/types.ts frontend/src/api/client.ts
git commit -m "feat(frontend): add getHeatTrend API method and HeatNode type"
```

---

### Task 3: 前端 — DiscoveryPanel 热点 Tab

**Files:**
- Modify: `frontend/src/components/graph/DiscoveryPanel.tsx`

#### 步骤 1：扩展 props 接口

编辑 `DiscoveryPanel.tsx`，在 `DiscoveryPanelProps` 接口中添加：

```typescript
interface DiscoveryPanelProps {
  // ... existing props
  /** 图谱热度趋势 */
  heatNodes?: HeatNode[]
  loadingHeat?: boolean
  onTraceHeat?: (nodeId: string) => void
}
```

在文件顶部 import 中添加：

```typescript
import type { HeatNode } from '../../api/types'
```

#### 步骤 2：添加 Tab 状态

在组件内部（`export default function DiscoveryPanel` 内部，return JSX 之前）添加：

```typescript
const [activeTab, setActiveTab] = React.useState<'discover' | 'heat'>('discover')
```

#### 步骤 3：添加 Tab 栏

找到 Header 区域（第一个 `<div style={{...}}>`），在 "💡 发现关联" 文字处修改为 Tab 栏：

```typescript
<div style={{
  padding: '10px 14px 8px', borderBottom: '1px solid rgba(46,139,144,0.3)',
  display: 'flex', alignItems: 'center', justifyContent: 'space-between',
}}>
  <div style={{ display: 'flex', gap: 4 }}>
    <button
      onClick={() => setActiveTab('discover')}
      style={{
        ...tabStyle,
        background: activeTab === 'discover' ? 'rgba(46,139,144,0.3)' : 'transparent',
        color: activeTab === 'discover' ? '#9ec5ee' : '#6b7280',
      }}
    >
      发现
    </button>
    <button
      onClick={() => setActiveTab('heat')}
      style={{
        ...tabStyle,
        background: activeTab === 'heat' ? 'rgba(46,139,144,0.3)' : 'transparent',
        color: activeTab === 'heat' ? '#9ec5ee' : '#6b7280',
      }}
    >
      热点
    </button>
  </div>
  <button onClick={onClose} style={closeBtnStyle}>✕</button>
</div>
```

将现有的 `tabStyle` 定义（如果不存在）添加到组件中：

```typescript
const tabStyle: React.CSSProperties = {
  padding: '2px 8px',
  fontSize: 12,
  borderRadius: 4,
  border: 'none',
  cursor: 'pointer',
  transition: 'all 0.15s ease',
}
```

#### 步骤 4：条件渲染面板内容

找到当前的 `{loading && ...}` 和推荐列表渲染块，用条件包裹：

```typescript
{activeTab === 'discover' && (
  <>
    {/* 现有推荐列表内容 */}
    <div style={{ padding: '8px 0' }}>
      {loading && <div style={emptyStyle}>分析中...</div>}
      {!loading && pending.length === 0 && (
        <div style={emptyStyle}>暂无新发现<br /><span style={{ fontSize: 11, opacity: 0.6 }}>继续积累，关联会逐渐浮现</span></div>
      )}
      {/* ... 其余推荐列表 JSX 保持不变 */}
    </div>
  </>
)}

{activeTab === 'heat' && (
  <>
    <div style={{ padding: '8px 0' }}>
      {loadingHeat && <div style={emptyStyle}>加载中...</div>}
      {!loadingHeat && (!heatNodes || heatNodes.length === 0) && (
        <div style={emptyStyle}>本周暂无热点<br /><span style={{ fontSize: 11, opacity: 0.6 }}>继续添加想法和关联，知识网络会越来越活跃</span></div>
      )}
      {!loadingHeat && heatNodes && heatNodes.map(node => (
        <div
          key={node.node_id}
          onClick={() => onTraceHeat?.(node.node_id)}
          style={{
            margin: '0 10px 8px', padding: '8px 10px',
            background: 'rgba(46,139,144,0.1)', borderRadius: 6,
            border: '1px solid rgba(46,139,144,0.2)',
            cursor: 'pointer',
          }}
        >
          {/* Rank badge */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
            <span style={{
              display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
              width: 18, height: 18, borderRadius: '50%',
              fontSize: 10, fontWeight: 700,
              background: node.rank <= 3 ? '#f59e0b' : 'rgba(255,255,255,0.1)',
              color: node.rank <= 3 ? '#000' : '#9ec5ee',
            }}>
              {node.rank}
            </span>
            <span style={{ fontSize: 11, color: '#6b7280' }}>
              编辑 {node.edit_count} · 关联 {node.edge_count}
            </span>
          </div>
          {/* Content */}
          <div style={{ fontSize: 12, color: '#c0d8f0', marginBottom: 6, lineHeight: 1.5 }}>
            {node.content.length > 40 ? node.content.slice(0, 40) + '...' : node.content}
          </div>
          {/* Heat bar */}
          <div style={{ height: 4, borderRadius: 2, background: 'rgba(255,255,255,0.1)', overflow: 'hidden' }}>
            <div style={{
              height: '100%',
              width: `${node.heat_score * 100}%`,
              background: node.heat_score > 0.6 ? '#ef4444' : (node.heat_score > 0.3 ? '#f59e0b' : '#6b7280'),
              borderRadius: 2,
              transition: 'width 0.3s ease',
            }} />
          </div>
        </div>
      ))}
    </div>
  </>
)}
```

注意：移除旧的 `<div style={{ padding: '10px 14px 8px' ...` 中的 "💡 发现关联" 文字（已被新 Tab 栏替代）。

#### 步骤 5：处理热力 Tab 点击

当用户点击热点节点时，调用 `onTraceHeat(nodeId)` 回调，父组件（LakeGraph 的调用方）需要实现该回调来执行 `panToNode`。

#### 步骤 6：TypeScript 编译检查

```bash
cd frontend && npx tsc --noEmit
```

预期：无编译错误。

#### 步骤 7：提交

```bash
git add frontend/src/components/graph/DiscoveryPanel.tsx
git commit -m "feat(frontend): add heat trend tab to DiscoveryPanel — weekly hot nodes ranking"
```

---

### Task 4: 前端 — LakeGraph 热力标记

**Files:**
- Modify: `frontend/src/components/LakeGraph.tsx`

#### 步骤 1：添加 HeatNode 类型和 props

在 `LakeGraph.tsx` 顶部的 import 中添加 `HeatNode`：

```typescript
import type { EdgeItem, NodeItem, NodeState, Recommendation, PathResult, Cluster, PlanningSuggestion, HeatNode } from '../api/types'
```

找到 `LakeGraphProps` 接口（around line 938），添加：

```typescript
/** 图谱热度趋势：热度数据 */
heatNodes?: HeatNode[]
onTraceHeatNode?: (nodeId: string) => void
```

#### 步骤 2：构建 heatNodeMap

在组件内部（`useMemo` 区域，around line 803），找到 `highlightedIds` 之后添加：

```typescript
const heatNodeMap = useMemo(() => {
  if (!heatNodes) return new Map<string, HeatNode>()
  return new Map(heatNodes.map(n => [n.node_id, n]))
}, [heatNodes])
```

#### 步骤 3：在 AnimatedNode 渲染中叠加热力效果

找到 `SpringNodes` 渲染区域（around line 858-881），在节点渲染的 `position` 附近找到 `AnimatedNode` 的 `key={node.id}` 调用，在其参数中添加热力相关的 props。

查看 `AnimatedNode`（line 256）和 `AnimNodeProps`（line 223）定义，`AnimatedNode` 目前没有 `heatScore` prop。需要找到节点 mesh 渲染部分，修改其 `emissive` 和 `emissiveIntensity`。

在 `AnimatedNode` 函数体内，找到设置 emissive 的部分（around line 313）：

```typescript
emissive={multiSelected ? '#4ecdc4' : (selected ? color : (highlighted ? '#ffd700' : (hovered ? color : '#000000')))}
emissiveIntensity={multiSelected ? 0.7 : (selected ? 0.6 : (highlighted ? 0.8 : (hovered ? 0.25 : 0)))}
```

在 `AnimNodeProps` 中添加 `heatScore` 参数：

```typescript
heatScore?: number
```

然后修改 `AnimatedNode` 函数签名：

```typescript
function AnimatedNode({ node, position, selected, multiSelected, onClick, isNew, onDragStart, simNode, highlighted, isDragging, recCount = 0, dimmed = false, editingInfo, heatScore = 0 }: AnimNodeProps) {
```

在 emissive/Intensity 的三元表达式中，添加热力判断（热力优先于普通状态，但低于选中态）：

```typescript
emissive={multiSelected ? '#4ecdc4' : (selected ? color : (highlighted ? '#ffd700' : (heatScore > 0.5 ? '#f59e0b' : (hovered ? color : '#000000'))))}
emissiveIntensity={multiSelected ? 0.7 : (selected ? 0.6 : (highlighted ? 0.8 : (heatScore > 0.5 ? 0.4 + heatScore * 0.3 : (hovered ? 0.25 : 0))))}
```

同时调整节点大小（热力节点略大）。找到 `radius` 相关计算处，添加热力缩放：

```typescript
const radius = Math.max(4, Math.min(14, 8 * Math.sqrt(Math.max(0, (node.item.content.length / 50)))) * cameraScale * (1 + heatScore * 0.3))
```

#### 步骤 4：将 heatScore 传入 AnimatedNode

在 SpringNodes 渲染处（around line 874），找到传入 AnimatedNode 的 props，添加：

```typescript
animatedNodeProps={{
  // ... existing props
  heatScore: heatNodeMap.get(node.id)?.heat_score ?? 0,
}}
```

注意：需要查看 `SpringNodes` 组件如何接收和传递这些 props。如果 `SpringNodes` 没有直接暴露 `animatedNodeProps`，可能需要修改 `SpringNodes` 的 props 接口来传递 `heatScore`。

查看 `SpringNodes` 的 props（around line 576 `SceneProps`），找到：

```typescript
const AnimatedNode = React.memo(({ node, position, ... }: AnimNodeProps) => { ... })
```

在 JSX 中找到 `AnimatedNode` 的调用（应该在 SpringNodes 的 map 循环内），添加 `heatScore={heatNodeMap.get(node.id)?.heat_score ?? 0}`。

如果 AnimatedNode 在 Scene 内部，可以通过 sceneProps 传递。先确认当前 AnimatedNode 在哪里被调用：

```bash
grep -n "AnimatedNode" d:/work/local/Ripple/frontend/src/components/LakeGraph.tsx
```

如果 AnimatedNode 是在 SpringNodes 组件内部的 map 上调用的，直接在那里添加 `heatScore` prop 即可。

#### 步骤 5：支持热力节点点击定位

在 LakeGraph 组件中添加 `highlightedIds` 对热力节点的支持。当 `onTraceHeatNode` 被调用时，将对应 nodeId 添加到 `highlightedIds` 中。

找到 `highlightedIds` 的 useMemo（line 803），修改条件，使其同时包含搜索高亮和热力高亮：

```typescript
const highlightedIds = useMemo(() => {
  const ids = new Set<string>()
  if (searchQuery) {
    const q = searchQuery.toLowerCase()
    for (const n of displayNodes) {
      if (n.content.toLowerCase().includes(q)) ids.add(n.id)
    }
  }
  return ids
}, [searchQuery, displayNodes])
```

添加热力高亮 state：

```typescript
const [heatHighlightedId, setHeatHighlightedId] = React.useState<string | null>(null)
```

在 `DiscoveryPanel` 的调用处，传入 `onTraceHeat`：

```typescript
onTraceHeat={(nodeId) => {
  setHeatHighlightedId(nodeId)
  setTimeout(() => setHeatHighlightedId(null), 3000)
}}
```

然后修改 `highlightedIds` useMemo，加上热力高亮：

```typescript
const highlightedIds = useMemo(() => {
  const ids = new Set<string>()
  if (searchQuery) {
    const q = searchQuery.toLowerCase()
    for (const n of displayNodes) {
      if (n.content.toLowerCase().includes(q)) ids.add(n.id)
    }
  }
  if (heatHighlightedId) {
    ids.add(heatHighlightedId)
  }
  return ids
}, [searchQuery, displayNodes, heatHighlightedId])
```

同时将 `heatHighlightedId` 传入 `DiscoveryPanel`：

```typescript
heatHighlightedId={heatHighlightedId}
```

并在 `DiscoveryPanel` props 中添加 `heatHighlightedId?: string | null`。

#### 步骤 6：TypeScript 编译检查

```bash
cd frontend && npx tsc --noEmit
```

预期：无编译错误。

#### 步骤 7：提交

```bash
git add frontend/src/components/LakeGraph.tsx
git commit -m "feat(frontend): add heat markers to LakeGraph — hot nodes glow orange with size scaling"
```

---

## 三、自检清单

- [ ] **Spec 覆盖检查：** API handler ✓，热度计算（编辑+关联）✓，面板排行 ✓，图谱热力标记 ✓
- [ ] **Placeholder 检查：** 无 TBD/TODO，所有字段名、颜色值、权重数值已明确
- [ ] **类型一致性检查：** `HeatNode` 在 types.ts、client.ts、DiscoveryPanel.tsx、LakeGraph.tsx 中一致
- [ ] **依赖检查：** Task 2 依赖 Task 1（API 存在）；Task 3/4 依赖 Task 2（API 方法存在）
- [ ] **LOD 兼容：** 热力标记在 demand/simple/normal 三层均有表现（简单模式下通过颜色体现）

---

## 四、执行选择

**Plan complete and saved to `docs/superpowers/plans/2026-04-30-graph-heat-trend-plan.md`.**

**1. Subagent-Driven（推荐）** — 每个任务派发独立子 agent，两阶段审查（规格合规 → 代码质量）

**2. Inline Execution** — 本会话内顺序执行，阶段之间汇报进度

选择哪种方式执行？
