# Ripple 监控仪表板使用指南

> 适用：Phase 14 灰度上线 7 天观察期及后续运营  
> 依赖：`docker-compose.monitoring.yml` + `monitoring/` 目录

---

## 架构

```
Prometheus (19090) ──scrape──> ripple-staging-backend:8000/metrics
                   ──scrape──> ripple-staging-yjs-bridge:7790/metrics (可选)
Grafana    (13000) ──query──>  Prometheus
```

Grafana 内置 dashboard **Ripple Overview** 自动 provision，访问即可看到。

---

## 快速启动

```bash
# staging 服务器上
cd /home/admin/Ripple

# 首次拉取镜像（需联网）
docker compose -f docker-compose.staging.yml -f docker-compose.monitoring.yml pull prometheus grafana

# 启动监控（不影响 staging 服务）
docker compose -f docker-compose.staging.yml -f docker-compose.monitoring.yml up -d prometheus grafana
```

访问：
- **Prometheus**：http://fn.cky:19090
- **Grafana**：http://fn.cky:13000（admin / 设 `GRAFANA_PASSWORD` 或默认 `Admin888`）

---

## 仅停止监控

```bash
docker compose -f docker-compose.staging.yml -f docker-compose.monitoring.yml stop prometheus grafana
```

---

## Ripple Overview 面板说明

| 面板 | 指标 | 用途 |
|------|------|------|
| HTTP 请求速率 | `ripple_http_requests_total` | 总 QPS，观察高峰时段 |
| HTTP p95 延迟 | `ripple_http_request_duration_ms_bucket` | 慢接口告警（建议阈值 500ms）|
| DB 连接池 | `ripple_db_pool_acquired_conns` / `_total_conns` | 数据库压力，exceeded 时扩池 |
| LLM 调用总数 | `ripple_llm_calls_total` | AI 功能使用量 |
| HTTP 请求速率趋势 | 同上时序版 | 趋势分析 |
| HTTP 延迟 p50/p95/p99 | histogram | 性能退化早期预警 |
| DB 连接池趋势 | 时序 | 连接泄漏检测 |
| LLM 调用速率（by provider）| `by (provider)` | 各 provider 分流情况 |

---

## 灰度期间建议观察项

7 天观察期内，每天检查：

1. **HTTP 请求速率**：是否有意外流量峰值（> 平时 3x）
2. **HTTP p95 延迟**：> 500ms 时看 DB 连接池是否满
3. **DB 连接池**：`acquired_conns / total_conns > 0.8` 时考虑扩 max_conns（默认 pgxpool.Config.MaxConns=10）
4. **LLM 调用**：调用量 > 预算时检查 `llm_calls` 表（SELECT count(*) FROM llm_calls WHERE created_at > now()-interval '24h'）

---

## 告警规则（Phase 15 引入，当前手动巡检）

| 条件 | 建议阈值 | 行动 |
|------|---------|------|
| p95 延迟 | > 1000ms 持续 5min | 检查 DB 慢查询 + pprof |
| DB acquired/total | > 0.9 | 临时扩 maxConns，重启 backend |
| LLM 调用速率 | > 10 req/s | 检查是否有爬虫或滥用 |
| backend /healthz 不通 | 任何时刻 | 立即重启容器 + 查日志 |

---

## pprof 诊断

```bash
# 开启 pprof（设置 RIPPLE_PPROF_ADDR=:6060）
docker exec ripple-staging-backend sh -c "kill -USR1 1"  # 不支持热开关；需 .env 预设

# 若已开启：
curl http://fn.cky:6060/debug/pprof/ 
go tool pprof -top http://fn.cky:6060/debug/pprof/profile?seconds=30
```

---

## 注意事项

- `monitoring/` 目录及 compose 文件已提交 git，**不含任何密码**（密码走 `GRAFANA_PASSWORD` 环境变量）。
- Grafana 数据（图表历史）存于 `grafana_data` volume，重启不丢失；彻底清理：`docker volume rm ripple_grafana_data`。
- Prometheus 数据保留 15 天（`--storage.tsdb.retention.time=15d`）。
- 若 staging 网络名不是 `ripple-net`，编辑 `docker-compose.monitoring.yml` 的 `networks.ripple-net.name` 字段。

---

## Phase 15 新增功能监控补充

### AI Job Worker 监控

Phase 15.1 引入了 `AiJobWorker`（3 goroutine，`FOR UPDATE SKIP LOCKED` 轮询），可通过以下 SQL 监控任务积压：

```sql
-- 查询待处理任务数（积压）
SELECT COUNT(*) FROM ai_jobs WHERE status = 'pending';

-- 查询处于处理中超过 5 分钟的任务（可能卡死）
SELECT id, node_id, started_at FROM ai_jobs
WHERE status = 'processing' AND started_at < NOW() - INTERVAL '5 minutes';

-- 最近 24 小时任务失败率
SELECT
  COUNT(*) FILTER (WHERE status='done') AS done,
  COUNT(*) FILTER (WHERE status='failed') AS failed
FROM ai_jobs WHERE created_at > NOW() - INTERVAL '24 hours';
```

服务启动时会自动 `RecoverProcessing`（将 processing→pending），确保重启后无僵尸任务。

### LLM 用量 API 监控

新增端点 `GET /api/v1/organizations/{id}/llm_usage?days=N`，可用于：
- 定期汇总组织调用成本
- 告警阈值：单组织 7 天调用量 > 10000 次时人工复查

### 订阅状态监控

```sql
-- 查询活跃订阅分布
SELECT plan_id, COUNT(*) FROM org_subscriptions WHERE status='active' GROUP BY plan_id;

-- 查询 7 天内到期的订阅
SELECT org_id, plan_id, expires_at FROM org_subscriptions
WHERE status='active' AND expires_at < NOW() + INTERVAL '7 days';
```

### 告警规则补充（Phase 15）

| 条件 | 建议阈值 | 行动 |
|------|---------|------|
| AI 任务积压 | `pending > 50 持续 10min` | 检查 worker 是否存活（日志 `ai job worker started`）|
| AI 任务失败率 | `失败/总数 > 10%` | 检查 `ai_jobs.error` 字段 + LLM provider 状态 |
| 处理中超时 | `processing 且 started_at > 5min 前` | 手动重启 worker 触发 RecoverProcessing |

