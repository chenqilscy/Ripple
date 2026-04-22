# Ripple 性能压测剧本

## 目录

- `k6-baseline.js` — k6 基线压测（混合健康/列表/metrics）
- `vegeta-targets.txt` — vegeta 目标列表（HTTP 端点 + 头）
- `pprof-snapshot.ps1` — 抓取 pprof heap / goroutine 快照

## 运行

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

## 基线参考（待实测填入）

| 场景 | QPS | p50 | p95 | p99 | 错误率 |
|------|-----|-----|-----|-----|--------|
| GET /healthz | 1000 | TBD | TBD | TBD | 0% |
| GET /api/v1/lakes | 500 | TBD | TBD | TBD | <0.1% |
| POST /api/v1/perma_nodes | 50 | TBD | TBD | TBD | <0.5% |
