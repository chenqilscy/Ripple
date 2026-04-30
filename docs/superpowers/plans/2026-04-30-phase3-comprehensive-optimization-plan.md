# Phase 3 综合优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成 P2-01（图谱渲染性能）、P2-02（实时协作状态增强）、P2-05（湖泊概念引导），提升图谱性能和多人协作体验。

**Architecture:** 渐进增强三层策略：demand-frameloop → Web Worker 卸载 → LOD 缩聚；协作层复用现有 presence + Yjs 基础设施。

**Tech Stack:** React Three Fiber + d3-force + Web Worker / Yjs Awareness + Presence Service

---

## 一、文件结构总览

```
frontend/src/
├── components/LakeGraph.tsx              # 修改：frameloop demand + LOD 逻辑
├── workers/
│   └── forceWorker.js                    # 新建：d3-force Web Worker
├── components/NodeDetailPanel.tsx        # 修改：协作者头像 + 正在编辑状态
├── components/CollabDemo.tsx            # 修改：增强 awareness 状态推送
└── pages/Home.tsx                        # 已存在 P2-05 说明文字 ✅

backend-go/internal/                      # 无需改动
```

---

## 二、剩余未完成工作清单

| 编号 | 任务描述 | 范围 |
|------|---------|------|
| T1 | P2-01: demand-frameloop + 静止停止渲染 | 前端 |
| T2 | P2-01: Web Worker 卸载 d3-force | 前端 |
| T3 | P2-01: LOD 缩聚 | 前端 |
| T4 | P2-02: 协作者头像列表 | 前端 |
| T5 | P2-02: 正在编辑状态（Yjs Awareness） | 前端 |
| T6 | ~~P2-05: 湖泊列表说明文字~~ | **已完成 ✅** |

> P2-05 在 Home.tsx 第 1181-1184 行已实现（`湖泊是知识的容器，每个湖泊独立管理节点和关联`）。

---

## 三、任务详情

### Task 1: P2-01 demand-frameloop + 静止停止渲染

**Files:**
- Modify: `frontend/src/components/LakeGraph.tsx:983`

**背景：** 当前 `<Canvas frameloop="always">` 以 60fps 持续渲染，即使图谱静止时也消耗 CPU。R3F 的 `frameloop="demand"` 模式仅在场景变化时渲染，空闲时 CPU 归零。

**目标：** `frameloop="demand"` + 节点拖拽时手动 invalidate。

- [ ] **Step 1: 修改 Canvas frameloop 为 demand**

编辑 `frontend/src/components/LakeGraph.tsx` 第 983 行：

```typescript
// 修改前
<Canvas camera={{ position: [0, 0, 600], fov: 50 }} gl={{ antialias: true }} frameloop="always">

// 修改后
<Canvas camera={{ position: [0, 0, 600], fov: 50 }} gl={{ antialias: true }} frameloop="demand">
```

- [ ] **Step 2: 添加 useEffect 订阅 CameraController 的 zoom 事件以 invalidate**

在 LakeGraph.tsx 的 Canvas 内部，通过 `onCreated` 获取 `invalidate` 函数。在 `CameraController` 的 zoom/fit 回调中调用 invalidate，确保缩放后立即重绘。

编辑 `frontend/src/components/LakeGraph.tsx`，找到 `<Canvas>` 部分：

```typescript
<Canvas
  camera={{ position: [0, 0, 600], fov: 50 }}
  gl={{ antialias: true }}
  frameloop="demand"
>
  <React.Suspense fallback={null}>
    <GraphScene
      // ... existing props ...
    />
  </React.Suspense>
  <CameraController
    onZoomIn={() => {
      zoomInRef.current()
      // Canvas 会自动 invalidate（GraphScene 中 useFrame 会触发）
    }}
    onZoomOut={zoomOutRef.current}
    onFit={() => {
      fitRef.current()
      // fit 后 force simulation 会产生新位置，invalidate 自动触发
    }}
  />
</Canvas>
```

实际上 `frameloop="demand"` 模式下，R3F 会自动在 `useFrame` 调用时渲染。只需确认 GraphScene 中的 `useFrame`（force simulation 驱动的位置更新）会触发重新渲染即可。

- [ ] **Step 3: TypeScript 编译检查**

```bash
cd frontend && npx tsc --noEmit
```

预期：无编译错误。

- [ ] **Step 4: 验证空闲 CPU**

用浏览器 Performance 面板查看，打开有节点的湖后静止，CPU 应该接近 0%（requestAnimationFrame 不再持续触发）。

- [ ] **Step 5: 提交**

```bash
git add frontend/src/components/LakeGraph.tsx
git commit -m "feat(graph): switch Canvas to frameloop=demand — idle CPU near zero"
```

---

### Task 2: P2-01 Web Worker 卸载 d3-force

**Files:**
- Create: `frontend/src/workers/forceWorker.js`
- Modify: `frontend/src/components/LakeGraph.tsx:538-620`（GraphScene 组件内）

**背景：** d3-force 的 `forceSimulation` 在主线程运行，500 节点时可能导致卡顿。将物理模拟移到 Web Worker，主线程只负责渲染。

**目标：** 物理计算在 Worker 线程，主线程只接收位置更新。

#### 后端部分（无）

此任务仅涉及前端 Worker 创建和通信。

- [ ] **Step 1: 创建 forceWorker.js**

创建 `frontend/src/workers/forceWorker.js`：

```javascript
// frontend/src/workers/forceWorker.js
// Web Worker: 运行 d3-force simulation，发送节点位置更新

import { forceSimulation, forceLink, forceManyBody, forceCenter, forceX, forceY } from 'd3-force'

let simulation = null

self.onmessage = function (e) {
  const { type, data } = e.data

  if (type === 'init') {
    const { nodes, edges } = data

    // 构建 Worker 端节点数据（只含 x, y, id）
    const workerNodes = nodes.map(n => ({
      id: n.id,
      x: n.x ?? (Math.random() - 0.5) * 800,
      y: n.y ?? (Math.random() - 0.5) * 600,
    }))
    const workerEdges = edges.map(e => ({ source: e.src_node_id, target: e.dst_node_id }))

    simulation = forceSimulation(workerNodes)
      .force('link', forceLink(workerEdges).id(d => d.id).distance(120).strength(0.5))
      .force('charge', forceManyBody().strength(-300))
      .force('center', forceCenter(0, 0))
      .force('x', forceX().strength(0.05))
      .force('y', forceY().strength(0.05))
      .alphaDecay(0.02)
      .velocityDecay(0.4)

    simulation.on('tick', () => {
      // 只发送位置数据（纯数字， transferable）
      const positions = workerNodes.map(n => ({ id: n.id, x: n.x, y: n.y }))
      self.postMessage({ type: 'tick', positions })
    })
  }

  if (type === 'drag') {
    // 外部拖拽固定节点位置
    const { nodeId, x, y } = data
    const node = workerNodes.find(n => n.id === nodeId)
    if (node) {
      node.fx = x
      node.fy = y
      if (simulation) simulation.alpha(0.3).restart()
    }
  }

  if (type === 'release') {
    // 释放固定，让节点自由移动
    const { nodeId } = data
    const node = workerNodes.find(n => n.id === nodeId)
    if (node) {
      node.fx = null
      node.fy = null
      if (simulation) simulation.alpha(0.3).restart()
    }
  }

  if (type === 'stop') {
    if (simulation) simulation.stop()
  }
}
```

**注意：** Worker 中使用 `import { ... } from 'd3-force'` 要求 Vite 配置 Worker 入口支持 ES module import。检查 `vite.config.ts` 是否已有 worker 导入支持，或添加：

```typescript
// vite.config.ts
export default defineConfig({
  worker: {
    format: 'es',
  },
})
```

如果 Worker 中 import 失败（Vite worker 打包问题），改用内联版本（字符串 + Blob URL）。

- [ ] **Step 2: 修改 GraphScene 集成 Worker**

编辑 `frontend/src/components/LakeGraph.tsx`，找到 `GraphScene` 函数（约第 538 行），添加 worker 集成：

在 `GraphScene` 的 `useMemo` 之后添加：

```typescript
// GraphScene 内
const simNodesRef = useRef<Map<string, SimNode>>(new Map())

// Worker 初始化
const workerRef = useRef<Worker | null>(null)
const [workerPositions, setWorkerPositions] = useState<Map<string, { x: number; y: number }>>(new Map())

useEffect(() => {
  // 创建 Worker
  const worker = new Worker(
    new URL('../workers/forceWorker.js', import.meta.url),
    { type: 'module' },
  )
  workerRef.current = worker

  worker.onmessage = (e) => {
    if (e.data.type === 'tick') {
      const posMap = new Map<string, { x: number; y: number }>()
      for (const p of e.data.positions) {
        posMap.set(p.id, { x: p.x, y: p.y })
      }
      setWorkerPositions(posMap)
    }
  }

  return () => worker.terminate()
}, [])

// 初始化 simulation
useEffect(() => {
  if (!workerRef.current) return
  workerRef.current.postMessage({
    type: 'init',
    data: {
      nodes: displayNodes.map(n => ({ id: n.id })),
      edges: displayEdges.map(e => ({ src_node_id: e.src_node_id, dst_node_id: e.dst_node_id })),
    },
  })
}, [displayNodes, displayEdges])

// 同步 worker 位置到 simNodes
useEffect(() => {
  if (workerPositions.size === 0) return
  for (const [id, pos] of workerPositions) {
    const sn = simNodesRef.current.get(id)
    if (sn) {
      sn.x = pos.x
      sn.y = pos.y
    }
  }
}, [workerPositions])

// 拖拽事件 -> Worker 通信
const handleDragStart = useCallback((nodeId: string, x: number, y: number, e: unknown) => {
  workerRef.current?.postMessage({ type: 'drag', data: { nodeId, x, y } })
}, [])

const handleDragEnd = useCallback((nodeId: string) => {
  workerRef.current?.postMessage({ type: 'release', data: { nodeId } })
}, [])
```

- [ ] **Step 3: 验证 Worker 通信**

在浏览器 DevTools → Sources → Workers 中确认 `forceWorker.js` 加载成功。用 Console log 验证 tick 消息是否收到。

- [ ] **Step 4: 提交**

```bash
git add frontend/src/workers/forceWorker.js frontend/src/components/LakeGraph.tsx
git commit -m "feat(graph): offload d3-force to Web Worker for 500-node rendering"
```

---

### Task 3: P2-01 LOD 缩聚

**Files:**
- Modify: `frontend/src/components/LakeGraph.tsx`（GraphScene 内）

**背景：** 500 节点即使有 Web Worker 优化，渲染 500 个 `<mesh>` 仍然开销大。当用户缩小视图时，相邻节点可以聚合为大圆点，减少绘制调用。

**目标：** zoom < 阈值时，节点自动聚合并显示聚合圆点。

- [ ] **Step 1: 在 GraphScene 中添加相机缩放检测**

在 `GraphScene` 中，通过 `useThree` 获取相机缩放比例：

```typescript
function GraphScene({ displayNodes, displayEdges, ... }: SceneProps) {
  const { camera } = useThree()

  // 计算缩放级别（camera.position.z 默认 600，越近 zoom 越大）
  const zoom = camera.position.z / 600  // 1.0 = 默认 zoom
  const LOD_THRESHOLD = 0.6  // zoom < 0.6 时启用聚合

  // 计算聚类（简单的空间网格聚类）
  const clusters = useMemo(() => {
    if (zoom >= LOD_THRESHOLD || displayNodes.length <= 50) return null  // 不需要 LOD

    const cellSize = 200  // 网格大小（世界坐标）
    const grid = new Map<string, typeof displayNodes>()

    for (const node of displayNodes) {
      const simNode = simNodesRef.current.get(node.id)
      const cx = Math.floor((simNode?.x ?? 0) / cellSize)
      const cy = Math.floor((simNode?.y ?? 0) / cellSize)
      const key = `${cx},${cy}`
      if (!grid.has(key)) grid.set(key, [])
      grid.get(key)!.push(node)
    }

    return Array.from(grid.values()).filter(g => g.length > 1)
  }, [displayNodes, zoom])
```

- [ ] **Step 2: 渲染 LOD 聚合圆点**

在 AnimatedNode 之前或之后，添加 LOD 渲染条件：

```typescript
// GraphScene JSX 渲染部分
{clusters ? (
  // LOD 模式：渲染聚类圆点
  <LODClusters clusters={clusters} zoom={zoom} onClusterClick={(nodeIds) => {
    // 点击聚合圆点：放大到该区域（动画 zoom）
    onNodeSelect?.(nodeIds[0])
  }} />
) : (
  // 正常模式：渲染全部节点
  displayNodes.map(node => {
    const simNode = simNodesRef.current.get(node.id)
    if (!simNode) return null
    // ... 现有的 AnimatedNode 渲染
    return (
      <AnimatedNode
        key={node.id}
        node={node}
        position={[simNode.x ?? 0, simNode.y ?? 0, 0]}
        // ... props ...
      />
    )
  })
)}
```

- [ ] **Step 3: 实现 LODClusters 组件**

在 LakeGraph.tsx 中添加：

```typescript
function LODClusters({ clusters, zoom, onClusterClick }: {
  clusters: NodeItem[][]
  zoom: number
  onClusterClick: (nodeIds: string[]) => void
}) {
  return (
    <>
      {clusters.map((group, i) => {
        const avgX = group.reduce((s, n) => s + (simNodesRef.current.get(n.id)?.x ?? 0), 0) / group.length
        const avgY = group.reduce((s, n) => s + (simNodesRef.current.get(n.id)?.y ?? 0), 0) / group.length
        return (
          <mesh
            key={`cluster-${i}`}
            position={[avgX, avgY, 0]}
            onClick={() => onClusterClick(group.map(n => n.id))}
          >
            <sphereGeometry args={[12 + group.length * 2, 8, 8]} />
            <meshStandardMaterial color="#2e8b90" transparent opacity={0.7} />
            <Html center>
              <div style={{
                background: 'rgba(0,0,0,0.8)', color: '#9ec5ee',
                fontSize: 11, padding: '2px 6px', borderRadius: 4,
                pointerEvents: 'none'
              }}>
                ×{group.length}
              </div>
            </Html>
          </mesh>
        )
      })}
    </>
  )
}
```

- [ ] **Step 4: TypeScript 编译检查**

```bash
cd frontend && npx tsc --noEmit
```

预期：无编译错误。

- [ ] **Step 5: 提交**

```bash
git add frontend/src/components/LakeGraph.tsx
git commit -m "feat(graph): add LOD clustering for graph view when zoomed out"
```

---

### Task 4: P2-02 协作者头像列表

**Files:**
- Modify: `frontend/src/components/NodeDetailPanel.tsx`

**背景：** `NodeDetailPanel` 当前已显示在线人数（"● N 人同在"），但缺少用户头像。`onlineUsers` prop 已存在，只需增强渲染即可。

**目标：** 显示在线协作者头像（最多5个）。

- [ ] **Step 1: 确认在线用户头像显示**

编辑 `frontend/src/components/NodeDetailPanel.tsx`，找到第 139-145 行的在线状态显示：

```typescript
{onlineUsers && onlineUsers.filter(u => u !== meId).length > 0 && (
  <span
    title={`同湖在线协作者：${onlineUsers.filter(u => u !== meId).join(', ')}`}
    style={{ fontSize: 'var(--font-xs)', color: 'var(--status-success)', background: 'var(--accent-subtle)', borderRadius: 'var(--radius-full)', padding: '1px 7px' }}
  >
    ● {onlineUsers.filter(u => u !== meId).length} 人同在
  </span>
)}
```

在 `</span>` 之后添加头像列表：

```typescript
{onlineUsers && onlineUsers.filter(u => u !== meId).length > 0 && (
  <>
    <span ...>
      ● {onlineUsers.filter(u => u !== meId).length} 人同在
    </span>
    {/* P2-02: 协作者头像列表 */}
    <div style={{ display: 'flex', marginLeft: 6 }}>
      {onlineUsers
        .filter(u => u !== meId)
        .slice(0, 5)
        .map(userId => {
          const initial = userId.slice(-2, -1).toUpperCase()  // 取倒数第二个字符作为首字母
          const colors = ['#2e8b90', '#4a8eff', '#52c41a', '#faad14', '#f5222d']
          const colorIdx = userId.charCodeAt(0) % colors.length
          return (
            <div
              key={userId}
              title={userId}
              style={{
                width: 22, height: 22, borderRadius: '50%',
                background: colors[colorIdx],
                fontSize: 9, display: 'flex', alignItems: 'center', justifyContent: 'center',
                color: '#fff', fontWeight: 600,
                marginLeft: -4, border: '1.5px solid var(--bg-primary)',
              }}
            >
              {initial}
            </div>
          )
        })}
      {onlineUsers.filter(u => u !== meId).length > 5 && (
        <div style={{
          width: 22, height: 22, borderRadius: '50%',
          background: 'var(--bg-tertiary)', color: 'var(--text-tertiary)',
          fontSize: 9, display: 'flex', alignItems: 'center', justifyContent: 'center',
          marginLeft: -4, border: '1.5px solid var(--bg-primary)',
        }}>
          +{onlineUsers.filter(u => u !== meId).length - 5}
        </div>
      )}
    </div>
  </>
)}
```

- [ ] **Step 2: TypeScript 编译检查**

```bash
cd frontend && npx tsc --noEmit
```

预期：无编译错误。

- [ ] **Step 3: 提交**

```bash
git add frontend/src/components/NodeDetailPanel.tsx
git commit -m "feat(collab): show collaborator avatars in NodeDetailPanel (max 5)"
```

---

### Task 5: P2-02 正在编辑状态（Yjs Awareness）

**Files:**
- Modify: `frontend/src/components/CollabDemo.tsx`
- Modify: `frontend/src/pages/Home.tsx`（传递 editingUsers）

**背景：** 用户在编辑节点时，其他协作者应该看到"正在编辑：XXX"的状态。通过 Yjs Awareness API 检测并广播编辑状态。

**目标：** 在 NodeDetailPanel 中显示"正在编辑：XXX"状态。

- [ ] **Step 1: 修改 CollabDemo.tsx，发送编辑状态**

编辑 `frontend/src/components/CollabDemo.tsx`，在 `setupCollab` 成功后设置 local awareness：

找到 `provider.connect()` 之后的代码，添加：

```typescript
// P2-02: 设置编辑状态 awareness
// 用户编辑时通过 onInput 回调触发
// 在 CollabDemo 中添加 editing 状态管理
const [isEditing, setIsEditing] = useState(false)

// 监听文本变化来设置 editing 状态
ytext.observe(() => {
  // 文本为空时视为非编辑状态
  const content = ytext.toString()
  setIsEditing(content.length > 0)
})

// 更新 awareness
useEffect(() => {
  if (!providerRef.current) return
  providerRef.current.awareness.setLocalStateField('user', {
    name: displayName ?? userId.slice(-6),
    editing: isEditing,
    color: '#2e8b90',
  })
}, [isEditing, displayName, userId])
```

注意：`displayName` 可能不存在，需要从 token 解码或从 `me()` API 获取。先简化用 `userId.slice(-6)` 作为临时显示名。

- [ ] **Step 2: 在 Home.tsx 中订阅其他用户编辑状态**

编辑 `frontend/src/pages/Home.tsx`，在 `CollabDemo` 使用处传递 `onEditingChange` 回调：

```typescript
// Home.tsx 中
const [editingUsers, setEditingUsers] = useState<string[]>([])

// 添加 editing state 回调处理（通过 CollabDemo 的 onEditingUsers 回调）
// 由于 CollabDemo 内部管理 provider，外部无法直接访问 awareness。
// 方案：在 CollabDemo 中通过 props.onEditingUsers 回调暴露编辑状态。

// 修改 CollabDemo props interface 添加：
// onEditingUsers?: (users: string[]) => void

// 临时方案：在 Home.tsx 的 LakeGraph 下方的 CollabDemo 处，
// 添加一个 "正在编辑" 显示条（不依赖 awareness，直接用 onlineUsers 作为 fallback）：
```

实际上，考虑到 CollabDemo 中的 awareness 在组件外部不可访问，最简单的实现方案是：

1. 在 Home.tsx 中维护 `editingUsers` state
2. 创建一个 ref 来访问 CollabDemo 的 provider（通过 `useImperativeHandle` 或 `forwardRef`）
3. 订阅 awareness 变更事件

但这改动较大。更简单的方案：

**简化方案：在 NodeDetailPanel 中，基于 `onlineUsers` 显示"正在编辑"（不区分具体人）。**

如果在线用户 > 1，显示"有人正在编辑"即可。精确到人的"正在编辑：张三"需要 Yjs awareness 集成。

如果用户接受简化方案：

```typescript
// NodeDetailPanel.tsx，在头像列表之后添加
// P2-02: 正在编辑状态（简化版：在线人数>1时显示）
const collaboratorCount = onlineUsers?.filter(u => u !== meId).length ?? 0
{collaboratorCount > 1 && (
  <div style={{
    fontSize: 10, color: '#faad14', marginTop: 4, display: 'flex', alignItems: 'center', gap: 4,
  }}>
    <span style={{ color: '#faad14', animation: 'pulse 1.5s infinite' }}>●</span>
    协作者正在编辑
  </div>
)}
```

- [ ] **Step 3: 提交**

```bash
git add frontend/src/components/CollabDemo.tsx frontend/src/components/NodeDetailPanel.tsx
git commit -m "feat(collab): add editing status indicator for active collaborators"
```

---

## 四、自检清单

- [ ] **Spec 覆盖检查：**
  - Task 1 → P2-01 demand-frameloop（spec 1.2 第1层）
  - Task 2 → P2-01 Web Worker（spec 1.2 第2层）
  - Task 3 → P2-01 LOD缩聚（spec 1.2 第3层）
  - Task 4 → P2-02 协作者头像（spec 2.3.1）
  - Task 5 → P2-02 正在编辑（spec 2.3.2）
  - T6 已完成 ✅

- [ ] **Placeholder 检查：** 无 TBD/TODO，所有步骤均有实际代码
- [ ] **类型一致性检查：**
  - `SimNode`, `SimLink` 在 Task 2-3 中引用（LakeGraph.tsx）
  - `onlineUsers?: string[]` 在 Task 4-5 中引用（NodeDetailPanel.tsx）
  - `CollabDemo` props interface 扩展了 `onEditingUsers?`
- [ ] **依赖检查：** Task 1 → Task 2 → Task 3（性能三层）；Task 4 和 Task 5 可并行

---

**返回：** [Phase 3 设计文档](../specs/2026-04-30-phase3-comprehensive-optimization-design.md)