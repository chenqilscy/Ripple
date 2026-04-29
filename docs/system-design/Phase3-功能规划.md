# Phase 3 功能规划：协作与实时推送完善

**日期：** 2026-04-29  
**状态：** 规划草稿  
**依赖：** Phase 2 UI 修复全部完成（17/17 ✅）

---

## 一、背景

Phase 2 完成了所有 UI 问题修复，并实现了 `Edge.strength` 全链路接入（创建 API → Neo4j → 前端 hover tooltip）。Phase 3 聚焦以下方向：

1. **协作感知深化**：多人同时编辑时的实时反馈体验
2. **AI Weaver 图感知进化**：M3 白皮书 §10 中提到的邻居上下文注入
3. **M3 凝结功能（Crystallize）**：PERMA 节点与反馈偏好画像
4. **WebSocket 稳定性提升**：当前 WS 30秒无消息超时重连机制优化

---

## 二、规划功能清单

### P0 · 必须先做

| # | 功能 | 说明 | 复杂度 |
|----|------|------|--------|
| 3-P0-01 | WS 心跳保活 | 客户端每 20s 发 ping，服务端 pong，超时 60s 断线重连（当前 30s 空闲断）| 低 |
| 3-P0-02 | Edge `strength` AI Weaver 写入 | `cloud.go` AIWeaver 在批量创建边时通过 LLM 相似度计算 Strength 并写入 | 中 |

### P1 · 重要功能

| # | 功能 | 说明 | 复杂度 |
|----|------|------|--------|
| 3-P1-01 | 协作光标跟随 | 他人光标在 3D 图谱上实时显示姓名+颜色（current: 仅位置推送，无名称渲染）| 中 |
| 3-P1-02 | PERMA 节点凝结 | 选定 N 节点 → 后台 LLM 生成摘要 → 创建永久节点 | 高 |
| 3-P1-03 | 节点编辑实时同步 | 基于 Yjs CRDT，多人同时编辑节点内容，实时合并 | 高 |
| 3-P1-04 | AI Weaver 邻居上下文注入 | buildPrompt 追加目标节点一跳邻居（Neo4j 查询，最多5个，4000字上限）| 中 |

### P2 · 扩展功能

| # | 功能 | 说明 | 复杂度 |
|----|------|------|--------|
| 3-P2-01 | 用户反馈偏好画像 | feedback_events 表 + 每周离线汇总 → 个性化 system prompt | 高 |
| 3-P2-02 | 多空间（MultiSpace） | Space CRUD + space_members + lakes.space_id | 高 |
| 3-P2-03 | 消息通知已读批量操作 | "全部标为已读" 按钮 | 低 |
| 3-P2-04 | P2-03 hover tooltip Phase4 | 边 hover 支持手工边的 label 展示（非仅 AI strength）| 低 |

---

## 三、优先执行路径（推荐）

```
3-P0-01（WS心跳）→ 3-P0-02（Strength写入）→ 3-P1-04（邻居上下文）→ 3-P1-01（光标跟随）→ 3-P1-02（PERMA节点）
```

- `3-P0-01` 是基础稳定性，改动极小（客户端 setInterval + 服务端 pong），可单独交付
- `3-P0-02` + `3-P1-04` 共同让 AI Weaver 产生有 strength 值的边，使 P2-03 hover 有实际数据
- `3-P1-01` 协作光标需协调 WS Hub 广播逻辑，单测可覆盖
- `3-P1-02` PERMA 节点复杂度最高，需新建 `perma_nodes` 表 + migration + 前端凝结 UI

---

## 四、接口设计要点（草稿）

### 3-P0-01：WS 心跳

客户端发：`{"type": "ping"}`  
服务端回：`{"type": "pong"}`  
客户端若 60s 内未收到任何消息 → 断线重连  

### 3-P0-02：AIWeaver strength

`cloud.go` 中 `processWeaveTask` 调用 LLM 生成节点时，同步批量创建边时：
- 使用向量相似度（或 LLM 打分）计算 strength
- 通过 `EdgeService.Create(ctx, systemActor, CreateEdgeInput{..., Strength: score})`

### 3-P1-02：凝结 API

```
POST /api/v1/lakes/{id}/crystallize
Body: { source_node_ids: string[], template: "summary"|"mindmap"|"narrative"|"qa", title_hint: string }
Response: { task_id: string }  # 异步，通过 WS 事件 PERMA_NODE_CREATED 推送
```

---

## 五、依赖与风险

| 风险 | 缓解 |
|------|------|
| Yjs CRDT 与现有 WS Hub 集成复杂度 | P3 内评估是否采用 yjs-bridge（cmd/yjs-bridge 已有原型） |
| PERMA 节点生成 LLM token 消耗 | 设置每用户每日凝结上限（配置化） |
| Multi-Space 迁移影响现有数据 | lakes.space_id 可为 NULL，存量数据不受影响 |
| AI Weaver strength 准确性 | 先接向量相似度，M3 后期升级 LLM 打分 |

---

## 六、里程碑

| 阶段 | 目标 | 预期 |
|------|------|------|
| Phase 3-A | 3-P0-01 + 3-P0-02 + 3-P1-04 | WS稳定 + AI边有strength |
| Phase 3-B | 3-P1-01 + 3-P1-03 | 协作感知完善 |
| Phase 3-C | 3-P1-02 + 3-P2-01 | PERMA凝结 + 反馈画像 |
| Phase 3-D | 3-P2-02 | 多空间（M3完整实现） |
