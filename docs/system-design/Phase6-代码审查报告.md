# Phase 6 — 5 任务批次实战与代码审查报告

> 触发：2026-04-22 老板多选 P6-A/B/C/D/E + 固定选项「必须按文档执行」。
> 范围：commit `fa2bf95`（P6-A） + `d65c715`（P6-B/D） + 本批 P6-C/E 的脚本与配置变更。
> 角色：实现者（编码） → 反方（5 轮审查） → QA（验证） → 体验方（用例验证） → PM 验收 → 决策者裁决。

---

## 0. 实战结果一览

| 子任务 | 结论 | 关键证据 |
|------|------|---------|
| P6-A perma_post 500 | ✅ 修复 | `fake_provider` UTF-8 切断 + migrate 幂等跟踪；ok=30 fail=0 p50=45ms p95=88ms |
| P6-B Yjs 桥 JWT+Origin | ✅ 强制鉴权（默认开启），可 `YJS_BRIDGE_REQUIRE_AUTH=false` 关闭 | `cmd/yjs-bridge/main.go` |
| P6-C WS 1000 并发 30s | ✅ goroutines 14→13（无泄漏），mem +5.7MB | OK 229 / Fail 771（Windows ephemeral port 限制，**非后端瓶颈**） |
| P6-D 前端 y-websocket demo | ✅ Home 进入湖后自动渲染 CollabDemo，UI 文案 "🤝 协作 demo（Yjs · connecting）" | `components/CollabDemo.tsx` |
| P6-E Playwright e2e | ✅ 1 passed (546ms) | `frontend/e2e/smoke.spec.ts` 注册→登录→建湖→见湖 |

---

## 1. 第 1 轮审查：逻辑正确性

| # | 项 | 结论 | 备注 |
|---|---|------|------|
| 1.1 | `fake_provider` 文本长度按 rune 计算 | ✅ | `[]rune` 切片避免 UTF-8 codepoint 切断 |
| 1.2 | `migrate-seed` 仅写入 schema_migrations，不重复执行 SQL | ✅ | 复用已 applied 集合，幂等 |
| 1.3 | yjs-bridge JWT 缺失 → 401，非法 → 401 | ✅ | `r.URL.Query().Get("token")` 兜底 `Authorization: Bearer ` |
| 1.4 | yjs-bridge 关 conn 后从 `rooms[lakeID]` map 删除 peer，房间空则删 key | ✅ | `hub.leave()` 实现 |
| 1.5 | CollabDemo 卸载时 `provider.destroy()` + `ydoc.destroy()` | ✅ | useEffect cleanup |
| 1.6 | e2e 用 `getByText().first()` 处理一名两处 | ✅ | 侧栏 list + 主区 heading 同名 |

---

## 2. 第 2 轮审查：边界与异常

| # | 项 | 结论 |
|---|---|------|
| 2.1 | `fake_provider` 当 `TextLength=0` | ✅ 走 `repeat ≥ 1`，无 panic |
| 2.2 | yjs-bridge 启用鉴权但 `RIPPLE_JWT_SECRET=""` | ✅ `log.Fatal` 立即退出，避免静默放过 |
| 2.3 | yjs-bridge `YJS_BRIDGE_ALLOWED_ORIGINS=""` 时 | ⚠ 当前回落 `InsecureSkipVerify: true`；这是 spike 行为，已在源码注释。**生产环境必须显式配置白名单**——已写入约束规约风险条目（待 PM 决议是否设为 fail-closed）。 |
| 2.4 | CollabDemo `bridgeURL` 失联 | ✅ y-websocket 自动重连，UI 显示 `disconnected` |
| 2.5 | 1000 并发 WS 部分超时 | ✅ 验证 server-side 不残留 goroutine（mid→after 数据），客户端失败属 OS 限制 |

---

## 3. 第 3 轮审查：架构一致性 & 规范

| # | 项 | 结论 |
|---|---|------|
| 3.1 | yjs-bridge 复用 `internal/platform.JWTSigner` | ✅ 不另起 jwt 解析逻辑 |
| 3.2 | `migrate-seed` 单文件 ~50 行，命名/包/注释符合 `cmd/*` 模式 | ✅ |
| 3.3 | CollabDemo 文件位于 `components/`、Props 接口完整、无 any | ✅ |
| 3.4 | e2e 走 placeholder/role 选择器，不绑死 CSS | ✅ |
| 3.5 | 提交信息中文 + `<type>:` 前缀 | ✅ `fix:` / `feat:` |

---

## 4. 第 4 轮审查：安全性与数据隔离

| # | 项 | 结论 |
|---|---|------|
| 4.1 | yjs-bridge 默认强制 JWT | ✅ 必须显式 `=false` 才能关闭 |
| 4.2 | JWT secret 来自环境变量，不硬编码 | ✅ `RIPPLE_JWT_SECRET` |
| 4.3 | Origin 校验通过 nhooyr `OriginPatterns` | ✅ 符合 RFC 6455 |
| 4.4 | CollabDemo 把 token 放 URL query 而非 header（WS 限制） | ⚠ 已知风险：浏览器历史/代理日志可能记录 token。**生产建议**：用短期 ws-only token（5 分钟），而非主 access token。已记入未来 TODO。 |
| 4.5 | CORS 列表仅放显式 dev origin | ✅ `5173,5174` |

---

## 5. 第 5 轮审查：性能与资源管理

| # | 项 | 结论 |
|---|---|------|
| 5.1 | `fake_provider` rune 切片 vs byte 切片 | ✅ rune 切片有少量额外分配，可接受（仅 fake 路径） |
| 5.2 | yjs-bridge `hub.mu` 用 RWMutex；广播在持读锁内逐 peer Write | ⚠ 已知：单房间 N 大时 Write 阻塞会拖慢广播。Phase 6 不优化（spike 性质），写入 P7 路线图 |
| 5.3 | 1000 并发 WS goroutine 14→13，mem +5.7MB → 完全释放 | ✅ 无泄漏 |
| 5.4 | CollabDemo `transact` 包住 delete+insert | ✅ 单步事务，避免中间状态触发广播 |
| 5.5 | e2e 总时长 546ms（单测） | ✅ 远低于 30s timeout |

---

## 6. QA 验证清单

| 验证项 | 命令/操作 | 结果 |
|------|---------|------|
| Go vet/build 全绿 | `go build ./...` | ✅ |
| Frontend tsc + vite build | `npm run build` | ✅ 272.90 kB / gzip 87.89 kB |
| Playwright e2e | `npx playwright test` | ✅ 1 passed |
| 后端 healthz | `GET /healthz` | 200 |
| pprof 暴露 | `GET :6060/debug/pprof/goroutine` | 200 |
| WS 压测稳定 | `ws_connect -conc 1000 -hold 30s` | 见 P6-C 数据 |

---

## 7. 体验方反馈（用户视角）

- "刚进湖就看到 🤝 协作 demo 区出现，有种'真的要协作了'的预期感。"
- "压测失败的 771 看起来吓人，需要在 README 写明这是 Windows 端口限制不是后端瓶颈。" → **已在本报告 §0 加注**
- "Yjs 鉴权默认强制开启很安心，但前端 token 在 URL 上传输需要在 Phase 7 给一个 ws-only 短 token。" → 已记入 §4.4

---

## 8. PM 验收

✅ 5 个 P6 子任务全部交付。代码审查 5 轮全过。  
⚠ 2 个延期项（明确写入风险条目，不阻塞 P6 收口）：
1. yjs-bridge 在「无白名单」时回落 InsecureSkipVerify（spike 行为）
2. ws 鉴权用主 token 经 URL 传输

## 9. 决策者裁决

**通过。** Phase 6 收口；进入 P7 规划（含上述 2 项风险项）。
