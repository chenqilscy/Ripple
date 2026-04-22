# M4-A 实时协作冲突解决 · 选型对比 + Spike

> 状态：Spike 阶段 · 2026-04-23
> 决策权：决策者（项目经理）；本文为正方提案 + 反方审查初稿

## 背景

M3 已实现节点级 WS 广播（增删改即时推送），但缺少：
- 字符级合并：两人同时编辑同一节点 content，后写覆盖
- 离线编辑合并：移动端断网后写入，重连合并
- 因果保序：不同客户端事件乱序到达，不能保证最终一致

CRDT / OT 是工业界两条主流路线。

## 三方案对比矩阵

| 维度 | Yjs (CRDT) | Automerge (CRDT) | 自研 OT |
|------|-----------|------------------|---------|
| **算法** | YATA (CRDT) | RGA (CRDT) | OT |
| **核心语言** | TypeScript | Rust + JS/Wasm | TypeScript |
| **后端集成** | y-websocket（独立服务）/ yrs (Rust) | automerge-repo / Rust crate | 后端需要 OT 服务器 |
| **Go 后端友好度** | ⚠️ 需桥接（独立 WS 服务 或 yrs FFI） | ⚠️ Rust crate via FFI | ✅ 可纯 Go 实现 |
| **离线合并** | ✅ 原生 | ✅ 原生 | ⚠️ 需自行设计 |
| **包体积（前端）** | ~50KB gzipped | ~80KB gzipped | < 10KB |
| **生态成熟度** | ★★★★★（VSCode/Notion 等用） | ★★★★（Lexical/Ink&Switch） | ★ |
| **维护成本** | 低（社区活跃） | 低（Ink&Switch 主导） | 高（需团队长期投入） |
| **二进制体积（doc）** | 紧凑（CRDT 增量） | 较大（每次 commit 完整 diff） | 紧凑 |
| **学习曲线** | 中 | 中 | 高 |
| **License** | MIT | MIT | 自有 |

### 关键约束（来自系统约束规约）

- 后端强制 Go 1.23+ → JS 库需独立服务
- 不引入实验性依赖（< 1.0 / GitHub stars < 1k）→ 自研 OT 风险高
- 中间件半年锁版本 → 选型必须谨慎

## 推荐方案：Yjs + 独立 y-websocket 桥接服务

### 决策依据

1. **生态最成熟**：Notion / VSCode Live Share / Figma（OT 但思路一致）背书
2. **Go 后端不耦合**：y-websocket 单独部署在 :7790（或 sidecar），主后端只通过事件订阅同步状态到 Neo4j
3. **前端零摩擦**：Yjs + y-prosemirror 直接接 React 文本组件，无需自研协议
4. **离线友好**：Yjs.encodeStateAsUpdate / applyUpdate 原生支持离线 diff
5. **未来可演进**：若后期 Go 生态有 yrs 稳定 FFI，再迁回 Go 内嵌

### 架构草图

```
Browser ─[Yjs binding]─ y-websocket Server (:7790, Node.js)
                             │
                             └─ Persistence Adapter
                                    │
                                    ├─ PG: nodes_doc(id, ydoc bytea, version int)
                                    └─ Webhook → Go API → Neo4j 同步
```

### 反方审查（初轮）

**反方** 提出的关键风险：

1. **多服务部署复杂度**：Node.js + Go 双进程，运维需额外监控
   - **正方** 答：M3 已经有 PG/Neo4j/Redis 三套依赖，新增一个 Node 服务边际成本可控；且 y-websocket 可降级为内嵌 sidecar
2. **认证一致性**：y-websocket 需要复用主后端的 JWT；社区方案是中间件验证 token query param
   - **正方** 答：y-websocket 有 `verifyClient` 钩子，可调用主后端 /api/v1/auth/verify 端点（预留接口）
3. **状态双写风险**：ydoc 在 Node 服务持久化 + Go 后端 Neo4j 索引，可能不一致
   - **正方** 答：以 ydoc 为单一事实源；Neo4j 仅做关系/搜索索引，从 ydoc 周期性派生（弱一致）
4. **冷启动**：用户打开节点时如果 ydoc 在 Node 服务尚未加载，需要从 PG 取出再恢复
   - **正方** 答：标准模式，y-websocket-server 自带 leveldb/postgres adapter

### 待评审决策点

- **决策者**：是否接受新增 Node.js 进程作为长期组件？
- **反方**：是否要先做 PoC 量化"Yjs 二进制 doc 在 PG 的存储成本 + 同步延迟"？
- **接口设计师**：nodes 表 schema 如何加 ydoc 列？是否新建 nodes_doc 表？

## Spike 计划（2 周）

### Week 1：技术验证

- [ ] D1-D2: 起 y-websocket-server demo，前端接 1 个节点 content，本地双窗口验证字符级合并
- [ ] D3: 加 PG persistence adapter，验证服务重启 ydoc 不丢
- [ ] D4: JWT 鉴权钩子接通主后端
- [ ] D5: 主后端订阅 webhook，把 ydoc 文本字段同步到 Neo4j

### Week 2：性能 + 成本

- [ ] D6-D7: 用 baseline.go 改造为 WS 客户端模拟 100 并发同时编辑 → 测延迟 / 内存
- [ ] D8: ydoc 体积分析（同一节点编辑 1000 次后的 doc 大小）
- [ ] D9: 反方 5 轮审查
- [ ] D10: PM 评审 + 决策者裁决

### 风险预案

- 若 y-websocket 在 100 并发时 P95 > 200ms：评估降级为 polling-based OT
- 若 ydoc 单节点 1000 次编辑后 > 1MB：引入 GC（Yjs.snapshot + 截断）
- 若 PG 写压力大：尝试 LevelDB sidecar

## 替代路径

若 Spike 失败：
- **Plan B**：Automerge + automerge-repo + Cloudflare Workers Durable Objects（重写部署）
- **Plan C**：弃用 CRDT，做"软冲突"：双写 → 标红 → 手动选合并

## 当前 TODO

- [x] 选型对比表
- [x] Spike 计划
- [ ] 决策者裁决「是否启动 Spike」
- [ ] 接口设计师设计 nodes_doc schema
- [ ] 反方对 PoC 范围二次评审
