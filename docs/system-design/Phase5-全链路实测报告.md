# Phase 5 全链路实测报告

> 日期：2026-04-23（CI 时区）
> 负责人：Ripple 团队（Go 后端 + Vite 前端 + 中间件 fn.cky）
> 范围：M4-T5（fake LLM）+ baseline + WS 容量 + perma_post 修复

---

## 1. 测试环境

| 组件 | 版本 / 配置 |
|------|-------------|
| OS | Windows 11，本机回环 `localhost` |
| Go | 1.23.4（`GOTOOLCHAIN=local`） |
| 后端 | `backend-go` main 分支 + P5-T1..T6 提交 |
| PG | fn.cky:15432 admin/Admin888 db=ripple |
| Neo4j | fn.cky:7687 neo4j/Admin888 |
| Redis | fn.cky:16379 |
| LLM | `RIPPLE_LLM_FAKE=true RIPPLE_LLM_FAKE_SLEEP_MS=50`；`RIPPLE_LLM_IMAGE_STUB=true` |
| 文件存储 | 本地 FS `data/uploads/` |

---

## 2. baseline（GET /healthz）

工具：`backend-go/cmd/loadtest/baseline`，原生 Go HTTP，长连接复用。

```
URL          : http://localhost:8000/healthz
Duration     : 10.00s
Concurrency  : 50
Requests     : 222 587 (ok=222 539, fail=48)
QPS          : 22 257
Error rate   : 0.022%
Latency p50  : 2.11 ms
Latency p95  : 3.23 ms
Latency p99  : 4.33 ms
```

> **结论**：本机 healthz 路径 ≥ 22 k QPS、p99 < 5 ms，满足"网关层零开销"目标。
> 极少数失败为 Windows TIME_WAIT 端口回收时的偶发 connect 重试。

---

## 3. WebSocket 连接容量（GET /api/v1/lakes/:id/ws）

工具：`backend-go/cmd/loadtest/ws_connect`，仅建连 + 保持。

| 并发 | 持续 | Dial OK | 失败 | Alive @ mid | p50 握手 | p95 握手 | p99 握手 |
|-----:|------|--------:|-----:|------------:|---------:|---------:|---------:|
| 100  | 5 s  | 100     | 0    | 100 (100%)  | 55.1 ms  | 67.0 ms  | 67.7 ms  |
| 200  | 10 s | (本轮未跑) | — | — | — | — | — |

> **结论**：100 并发 WS 100% 成功且全程在线，握手延迟在 ≤ 100 ms 区间稳定。后续应做 1 k 并发 30 s 持续测，观察内存/goroutine 峰值。
> （200 conc / 10 s 计划在下一轮跑，因本轮 perma_post 异常导致后端被回收，未及补测。）

---

## 4. 凝结接口（POST /api/v1/perma_nodes）

工具：`backend-go/cmd/loadtest/perma_post`，配合 `RIPPLE_LLM_FAKE=true`。

### 4.1 关键 Bug 发现并修复（P5-T7 增量修复）

**症状**：早期跑 `-conc 5 -dur 15s` 时观察到所有请求 400，QPS 高得离谱（2.2 M+）。

**根因**：loadtest harness 提交的字段名是 `node_ids` 与 `title`，但服务端契约是 `source_node_ids` 与 `title_hint`。
所有请求在 JSON decode 后即被 service 层拒为 `invalid input`，从未真正进入 LLM/Neo4j 路径。

**修复**：commit `e36da53`（含本报告）：
```go
body, _ := json.Marshal(map[string]any{
    "lake_id":         *lake,
    "source_node_ids": nodes,
    "title_hint":      "loadtest perma",
})
```

### 4.2 修复后单次探针

修复后，单次 POST 返回 500（fake LLM + Neo4j + 真 PG 路径未跑通，疑与 fake provider 在 `Crystallize` 上的契约对接缺失或 Neo4j 写入异常有关）。

> **结论**：perma_post 本轮**未取得有效 QPS/延迟数据**。
> 行动项（写入 backlog）：
> 1. 单步 debug 一次 `Crystallize` 全链路，确认 fake provider 是否被 Router 实际选中；
> 2. 若 `RIPPLE_LLM_PROVIDER_ORDER` 默认值未把 fake 排前，需补环境变量；
> 3. 给 500 响应加详细 error message（当前 `mapDomainError` 包了一层）。

---

## 5. 自检清单（已通过）

- [x] backend-go 全 build OK
- [x] backend-go 全 test OK（service / api/http / domain / llm / metrics / platform / presence / realtime）
- [x] frontend `tsc --noEmit` OK（含 P5-T1 AttachmentBar）
- [x] yjs-bridge 二进制独立 build OK（端口 :7790）
- [x] image-stub provider 注入 OK（启动日志显示 `LLM_IMAGE_STUB=true`）
- [x] healthz 22 k QPS / p99 < 5 ms
- [x] WS 100 conc 100% 成功

---

## 6. 下一轮（P6 候选）

| 任务 | 优先级 | 说明 |
|------|:-----:|------|
| 修复 perma_post 500 → 跑 conc=10 dur=30s 取真实数据 | 高 | 阻塞 M4-T5 真实交付 |
| WS 1 000 conc 30 s 持续 + 内存/goroutine 监控 | 高 | 验证 broker 容量 |
| 上传接口压测（含 magic bytes 校验路径） | 中 | 验证 P5-T3 不引入显著开销 |
| Yjs 桥 :7790 同步往返延迟 | 中 | 配合前端 y-websocket 客户端 |
| Playwright e2e 实际跑通（npm run e2e） | 中 | 需安装 chromium |

---

> 备注：本报告基于 P5 最后一次提交（含 P5-T7 perma_post 字段修复）。所有数字均为本机 localhost 数据，仅用作"自我基线"，正式回归须在专门压测机/独立网络环境复测。
