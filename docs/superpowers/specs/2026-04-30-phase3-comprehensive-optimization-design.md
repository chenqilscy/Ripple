# Phase 3 综合优化设计

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this design task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成 P2-01（图谱渲染性能）、P2-02（实时协作状态增强）、P2-05（湖泊概念引导），提升图谱性能和多人协作体验。

**Architecture:** 渐进增强三层策略：demand-frameloop → Web Worker 卸载 → LOD 聚合；协作层复用现有 presence + Yjs 基础设施。

**Tech Stack:** React Three Fiber + d3-force + Web Worker / Yjs Awareness + Presence Service

---

## 一、P2-01 图谱渲染性能（500 节点）

### 1.1 目标

支持 500+ 节点流畅渲染（60fps），内存占用 < 500MB。

### 1.2 方案：渐进增强三层策略

#### 第1层：demand-frameloop + 静止停止渲染循环

**当前问题：** `frameloop="always"` 导致即使图谱静止时也以 60fps 持续渲染。

**改动：** 将 LakeGraph.tsx 中 `<Canvas frameloop="demand">` 并在 `onChange` 时 invalidate。

```typescript
// LakeGraph.tsx
<Canvas
  frameloop="demand"
  onCreated={({ invalidate }) => {
    // 每当节点位置变化（拖拽/动画）时 invalidate()
    // 静止时 R3F 不会重渲染，CPU 归零
  }}
>
```

**预期效果：** 空闲时 CPU 使用率归零，用户交互时自动恢复渲染。

#### 第2层：d3-force 布局卸载到 Web Worker

**当前问题：** d3-force 的 `forceSimulation` 在主线程运行，500 节点时可能导致卡顿。

**方案：** 将物理模拟计算移到 Web Worker，Worker 每帧推送节点位置到主线程。

```
主线程                          Worker 线程
────────────────                ────────────────
nodes/edges 数据 ──────────→   接收节点数据
                                运行 forceSimulation
位置更新 ←───────────────    每帧推送 position
R3F 渲染节点
```

**实现：**
1. 创建 `workers/forceWorker.js` — 运行 d3-force simulation
2. LakeGraph.tsx 中启动 Worker，接收位置更新并更新 SimNode
3. 拖拽时临时固定节点（`fx`/`fy`），更新 Worker 继续模拟

**注意：** Web Worker 与 R3F 在不同线程，Worker 中 THREE.js 无法使用。需要：
- Worker 端只计算物理位置（纯数字 x/y）
- 主线程负责将位置同步到 THREE.Mesh

#### 第3层：LOD（Level of Detail）缩聚

**当前问题：** 500 节点即使停止物理计算，渲染 500 个 `<mesh>` 仍然开销大。

**方案：** 当相机 zoom < 阈值时，将相邻节点聚合为大圆点，显示数量标签。

```
zoom > 100:  显示全部节点详情
50 < zoom < 100: 节点缩小，显示基本信息
zoom < 50:  相邻节点合并为聚合圆点（显示 "×N"）
```

**实现：**
1. 在 `GraphScene` 中检测相机缩放比例
2. 当 zoom < 阈值时，计算节点空间聚类（简单的网格聚类）
3. 渲染聚合圆点 `<mesh>` + `<Html>` 显示数量
4. 缩放恢复时反向动画展示原始节点

**验收标准：**
- [ ] 500 节点流畅渲染（60fps on mid-range hardware）
- [ ] 内存占用 < 500MB
- [ ] 缩放/平移流畅
- [ ] demand-frameloop 空闲时 CPU 归零

### 1.3 文件结构

```
frontend/src/
├── components/LakeGraph.tsx          # 修改：添加 Canvas frameloop demand + LOD 逻辑
├── workers/
│   └── forceWorker.js                # 新建：d3-force Web Worker
└── ...

backend-go/internal/                  # 无需改动
```

---

## 二、P2-02 实时协作状态增强

### 2.1 目标

在 `NodeDetailPanel` 中显示协作者头像和"正在编辑"状态，增强多人协作可见性。

### 2.2 现状

- `presence.Service` 已支持获取湖在线用户列表（`GET /api/v1/lakes/{id}/presence`）
- `NodeDetailPanel` 已显示在线人数
- `CollabDemo.tsx` 已集成 Yjs WebSocket 协作

### 2.3 待实现功能

#### 2.3.1 协作者头像（NodeDetailPanel）

**改动：** 在 `NodeDetailPanel` 中显示在线用户的头像列表。

```typescript
// NodeDetailPanel.tsx
// 已有: const [onlineCount, setOnlineCount] = useState(0)
// 新增: const [onlineUsers, setOnlineUsers] = useState<string[]>([])

// 获取在线用户列表（复用 presence 逻辑）
useEffect(() => {
  api.listPresence(lakeId).then(r => setOnlineUsers(r.users))
}, [lakeId])

// 渲染：在线用户头像列表（取前5个）
{onlineUsers.length > 0 && (
  <div style={{ display: 'flex', gap: 4 }}>
    {onlineUsers.slice(0, 5).map(userId => (
      <div key={userId} style={{
        width: 24, height: 24, borderRadius: '50%',
        background: '#2e8b90', fontSize: 10, display: 'flex',
        alignItems: 'center', justifyContent: 'center'
      }}>
        {userId.slice(0, 1).toUpperCase()}
      </div>
    ))}
    {onlineUsers.length > 5 && <span> +{onlineUsers.length - 5}</span>}
  </div>
)}
```

#### 2.3.2 "正在编辑" 状态

**方案：** 利用 Yjs Awareness API 实时检测用户编辑状态。

```typescript
// 在 CollabDemo.tsx 中，Yjs provider 的 awareness 包含用户状态：
provider.awareness.setLocalStateField('user', {
  name: displayName,
  editing: true,
})

// NodeDetailPanel 或 LakeGraph 中订阅 awareness 变更：
provider.awareness.on('change', () => {
  const states = provider.awareness.getStates()
  const editors = Array.from(states.entries())
    .filter(([id, state]) => state.user?.editing && id !== clientID)
    .map(([id, state]) => state.user.name)
  setEditingUsers(editors)
})
```

**验收标准：**
- [ ] NodeDetailPanel 显示在线协作者头像（最多5个）
- [ ] 协作者头像显示在线状态变化（1秒内更新）
- [ ] "正在编辑：张三" 状态在 NodeDetailPanel 中显示
- [ ] 支持 10+ 协作者（性能不退化）

### 2.4 文件结构

```
frontend/src/
├── components/NodeDetailPanel.tsx   # 修改：添加头像列表 + 正在编辑状态
├── components/CollabDemo.tsx        # 修改：增强 awareness 状态推送
└── ...
```

---

## 三、P2-05 湖泊概念引导

### 3.1 目标

在湖泊列表顶部添加说明文字，解释"湖泊"与"项目/工作区"的区别。

### 3.2 方案

在 `Home.tsx` 的湖泊列表区域（SpaceSwitcher 或 LakeSelector）添加一行说明文字。

```typescript
// Home.tsx 或 SpaceSwitcher.tsx
// 在湖泊列表顶部添加
{lakeCount > 0 && (
  <div style={{
    fontSize: 11, color: '#666', padding: '8px 12px',
    borderBottom: '1px solid rgba(46,139,144,0.2)',
    lineHeight: 1.6
  }}>
    湖泊是知识的容器，每个湖泊独立管理节点和关联
  </div>
)}
```

**验收标准：**
- [ ] 湖泊列表顶部显示说明文字
- [ ] 文案符合品牌调性（青碧水纹风格）

### 3.3 文件结构

```
frontend/src/
├── pages/Home.tsx                    # 修改：添加湖泊说明文字
└── ...
```

---

## 四、任务分解

| 任务 | 描述 | 范围 | 预估工时 |
|------|------|------|---------|
| T1 | P2-01: demand-frameloop + 静止停止渲染 | 前端 | 2h |
| T2 | P2-01: Web Worker 卸载 d3-force | 前端 | 8h |
| T3 | P2-01: LOD 缩聚 | 前端 | 12h |
| T4 | P2-02: 协作者头像列表 | 前端 | 4h |
| T5 | P2-02: 正在编辑状态（Yjs Awareness） | 前端 | 6h |
| T6 | P2-05: 湖泊列表说明文字 | 前端 | 1h |
| **合计** | | | **33h** |

---

## 五、依赖关系

```
T1 (demand-frameloop)      → T2 (Web Worker) → T3 (LOD)
T4 (协作者头像)             → T5 (正在编辑)
T6 (湖泊说明)              → 无依赖
```

T1 无依赖，可最先实施；T4 和 T5 可并行；T6 独立。

---

## 六、测试策略

1. **P2-01 性能测试：** 用 mock 数据生成 500 节点，验证 fps（用 `performance.now()` 测量帧间隔）
2. **P2-02 协作测试：** 打开多个浏览器标签页，验证头像和编辑状态实时更新
3. **P2-05 文案测试：** 检查说明文字显示位置和样式

---

**返回：** [问题清单](../report/2026-04-29-ui分析/02-问题清单.md)