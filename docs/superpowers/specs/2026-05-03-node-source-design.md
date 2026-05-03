# 图谱节点来源区分设计

> 日期：2026-05-03
> 状态：设计完成

## 1. 背景与目标

图谱中的节点有两类来源：
- **用户输入**：用户直接创建的节点
- **AI 生成**：通过 AI 造云（CloudTask）生成的节点

当前图谱可视化无法区分这两类节点，用户无法快速识别 AI 产出。需要通过视觉手段区分，提升图谱的可读性和信息量。

## 2. 数据模型变更

### 2.1 后端扩展

`NodeItem` 新增字段：

```typescript
interface NodeItem {
  // ... 现有字段
  source: 'user' | 'ai'    // 来源：用户输入 或 AI 生成
  cloud_task_id?: string    // AI 生成时，记录来源的 CloudTask ID
}
```

### 2.2 实现方式

通过 `cloud_task_id` 字段判断节点来源：
- 如果 `cloud_task_id` 不为空 → AI 生成（`source: 'ai'`）
- 如果 `cloud_task_id` 为空 → 用户输入（`source: 'user'`）

优点：
- 无需额外的 API 查询
- 后端在创建节点时自动标记，准确性高
- 向后兼容，历史节点默认为用户输入

## 3. 视觉设计

### 3.1 节点样式

| 元素 | 用户输入节点 | AI 生成节点 |
|------|-------------|------------|
| 球体 | 正常渲染（跟随 state 颜色） | 正常渲染（跟随 state 颜色） |
| 边框 | 无 | 金色虚线边框 |
| 图标 | 无 | 右上角 SVG 机器人图标 |
| Tooltip | 显示 "👤 用户输入" | 显示 "🤖 AI 生成" |
| LOD 模式（缩放 ≤ 10%） | 无 | 只显示虚线边框 |

### 3.2 具体参数

**金色虚线边框：**
```typescript
color: '#f59e0b'        // 橙色/金色
dashArray: [4, 2]       // 4px 实线 + 2px 间隔
opacity: 0.6
lineWidth: 1.5
```

**SVG 机器人图标：**
- 尺寸：12px × 12px
- 位置：节点右上角偏移 [6, 6, 0]
- 颜色：`#f59e0b`（与边框一致）
- 仅在非 LOD 模式下显示

**SVG 图标内容（简约线条风格）：**
```svg
<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
  <rect x="3" y="8" width="18" height="12" rx="2"/>
  <circle cx="9" cy="14" r="2"/>
  <circle cx="15" cy="14" r="2"/>
  <line x1="12" y1="2" x2="12" y2="6"/>
  <circle cx="12" cy="2" r="1" fill="currentColor"/>
</svg>
```

### 3.3 Tooltip 内容

悬停时显示节点详情，增加来源标签：

```jsx
<div>
  <div style={{ fontWeight: 600, marginBottom: 3, color: '#9ec5ee' }}>
    {STATE_LABEL[node.state]} {node.source === 'ai' ? '🤖 AI 生成' : '👤 用户输入'}
  </div>
  <div>{node.content.slice(0, 80)}...</div>
  <div style={{ marginTop: 4, opacity: 0.55, fontSize: 10 }}>
    {new Date(node.created_at).toLocaleDateString('zh-CN')}
  </div>
</div>
```

## 4. 组件修改

### 4.1 AnimatedNode（普通模式）

修改点：
1. 接收 `source` prop（从 `node.source` 获取）
2. AI 节点：添加虚线边框（Torus）+ SVG 图标
3. Tooltip：根据 source 显示来源标签

```typescript
interface AnimNodeProps {
  // ... 现有 props
  source?: 'user' | 'ai'  // 新增
}
```

### 4.2 LODNode（LOD 模式）

修改点：
- AI 节点：添加虚线边框（Circle + LineLoop）
- 不显示图标（节点太小）

### 4.3 GraphScene

- 将 `node.source` 传递给 `AnimatedNode`

### 4.4 LakeGraphProps

```typescript
// LakeGraphProps 新增字段（如果需要前端缓存）
// 目前不需要，因为 source 字段会直接出现在 NodeItem 中
```

## 5. 后端实现

### 5.1 数据库层

```sql
-- 新增字段（如果尚不存在）
ALTER TABLE nodes ADD COLUMN cloud_task_id TEXT REFERENCES cloud_tasks(id);
```

### 5.2 API 层

- `/api/v1/lakes/{lake_id}/nodes` 返回时包含 `source` 和 `cloud_task_id` 字段
- 创建节点时：
  - 用户直接创建：`cloud_task_id` 为 NULL，`source` = 'user'
  - AI 造云创建：`cloud_task_id` 填充任务 ID，`source` = 'ai'

## 6. 性能考虑

- 无需额外的 API 请求，source 字段直接包含在 nodes 响应中
- SVG 图标使用 InstancedMesh 优化（如有多个人工节点）
- LOD 模式不渲染图标，减少 draw call

## 7. 向后兼容

- 历史节点（无 cloud_task_id）默认为用户输入（source: 'user'）
- 前端代码需处理 source 字段缺失的情况

## 8. 实现计划

1. **后端**：新增 cloud_task_id 字段 + source 计算逻辑
2. **前端**：LakeGraph.tsx 修改 AnimatedNode + LODNode + Tooltip
3. **测试**：验证用户输入节点和 AI 生成节点的视觉区分

## 9. 验收标准

- [ ] AI 生成的节点显示金色虚线边框 + SVG 机器人图标
- [ ] 用户输入的节点保持原有样式
- [ ] 悬停 tooltip 显示 "🤖 AI 生成" 或 "👤 用户输入"
- [ ] LOD 模式下 AI 节点显示虚线边框，不显示图标
- [ ] 历史节点（无 cloud_task_id）正确显示为用户输入
