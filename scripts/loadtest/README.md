# Ripple 性能压测剧本

## 目录

- `../../backend-go/cmd/loadtest/baseline/baseline.go` — Go 原生 HTTP GET 压测（基础端点 QPS/延迟）
- `../../backend-go/cmd/loadtest/perma_post/perma_post.go` — 凝结接口（POST /api/v1/perma_nodes）压测，建议配合 `RIPPLE_LLM_FAKE=true`
- `../../backend-go/cmd/loadtest/ws_connect/ws_connect.go` — WebSocket 仅建连+保持的并发连接压测
- `k6-baseline.js` — k6 基线压测（混合健康/列表/metrics）
- `vegeta-targets.txt` — vegeta 目标列表（HTTP 端点 + 头）
- `pprof-snapshot.ps1` — 抓取 pprof heap / goroutine 快照

## 运行

### baseline.go（推荐：零依赖）

```pwsh
# 后端启动后
cd backend-go
go run ./cmd/loadtest/baseline -url http://localhost:8000/healthz -dur 30s -conc 50
go run ./cmd/loadtest/baseline -url http://localhost:8000/api/v1/lakes -token <jwt> -dur 30s -conc 30
```

### perma_post.go（凝结压测，需 fake LLM 避免计费）

```pwsh
# 启动后端时设 RIPPLE_LLM_FAKE=true RIPPLE_LLM_FAKE_SLEEP_MS=50
go run ./cmd/loadtest/perma_post -base http://localhost:8000 -token <jwt> `
    -lake <lake_id> -nodes "<id1>,<id2>" -dur 30s -conc 10
```

### ws_connect.go（WS 连接容量）

```pwsh
go run ./cmd/loadtest/ws_connect -url ws://localhost:8000/api/v1/lakes/<lakeID>/ws `
    -token <jwt> -conc 1000 -hold 30s
```

### k6（推荐）

前提：安装 [k6](https://k6.io/docs/get-started/installation/)。

```pwsh
# 后端启动后（默认 :8000）
k6 run -e BASE=http://localhost:8000 -e TOKEN=<jwt> scripts/loadtest/k6-baseline.js
```

阈值：
- p95 < 200ms
- p99 < 500ms
- 错误率 < 1%

### vegeta

```pwsh
# 安装：scoop install vegeta
vegeta attack -duration=30s -rate=100 -targets=scripts/loadtest/vegeta-targets.txt | vegeta report
```

### pprof 快照

```pwsh
# 后端需启用：RIPPLE_PPROF_ADDR=:6060
.\scripts\loadtest\pprof-snapshot.ps1 -OutputDir .\pprof-out
```

输出：
- `heap-<timestamp>.pb.gz`
- `goroutine-<timestamp>.txt`

后续用 `go tool pprof heap-*.pb.gz` 交互分析。

## 监控埋点对照

`/metrics` Prometheus 格式输出。重点关注指标：

| 指标 | 说明 |
|------|------|
| `ripple_http_requests_total{path,method,status}` | HTTP 总数（已排除 /metrics 与 /healthz） |
| `ripple_http_request_duration_ms_bucket` | 直方图分布 |
| `ripple_llm_calls_total{provider}` | 每 provider 调用数 |
| `ripple_llm_call_duration_ms_bucket{provider}` | LLM 延迟分布 |
| `ripple_ws_connections` | 当前活跃 WS 连接数 |
| `ripple_ws_messages_in_total` / `out_total` | WS 消息计数 |

## 基线参考（2026-04-23 · Windows 本机 + fn.cky 中间件）

测试环境：
- 客户端 + 后端 同机（Windows 11，Go 1.23.4）
- PG / Neo4j / Redis 均在 fn.cky 远程主机
- 预热 5s，统计窗口 5s，HTTP keep-alive 启用

| 场景 | 并发 | 请求数 | QPS | p50 | p95 | p99 | 错误率 |
|------|-----|--------|-----|-----|-----|-----|--------|
| GET /healthz（无依赖） | 20 | 87596 | 17 517 | 1.06 ms | 2.11 ms | 2.67 ms | 0.022% |
| GET /metrics（自研聚合） | 20 | 83212 | 16 640 | 1.08 ms | 2.21 ms | 2.84 ms | 0.023% |
| GET /api/v1/lakes（PG 查询 + Neo4j 关联） | 20 | 11169 | 2 233 | 7.41 ms | 10.77 ms | 14.01 ms | 0.179% |

**说明**：
- 错误率主要来自压测窗口结束时的 ctx 取消，并非服务端 5xx。
- /api/v1/lakes 的 7~14ms 延迟主要由 fn.cky 远程网络 RTT 主导。
- p99 内的尖峰多由首次连接握手与日志同步刷盘引起；同机更长窗口测试可降至 5ms 内。

**结论**：当前 M3 阶段无依赖端点已具备万 QPS 量级能力；带数据库读取的列表端点 2k+ QPS 满足 100 并发用户场景。
