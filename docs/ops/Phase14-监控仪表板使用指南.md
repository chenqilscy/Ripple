# Ripple 监控仪表板使用指南

> 适用：Phase 14 灰度上线 7 天观察期、Phase 15 运营巡检与值班处置
> 依赖：`docker-compose.monitoring.yml` + `monitoring/` 目录
> 当前状态：staging 已接入 Prometheus + Grafana；日志入口仍以 `docker logs` 为准，尚未落地 Loki / Alertmanager

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

## 值班入口（5 分钟首诊）

收到告警或体验方反馈后，先不要直接重启。先用下面 5 个入口判断是「服务挂了」还是「功能退化」：

| 检查项 | 命令 / 地址 | 期望结果 |
|------|-------------|---------|
| backend 健康 | `curl -fsS http://fn.cky:18000/healthz` | 返回 `{"status":"ok"}` |
| yjs-bridge 健康 | `curl -fsS http://fn.cky:17790/healthz` | 返回 `ok` |
| backend metrics | `curl -fsS http://fn.cky:18000/metrics | head` | 返回 Prometheus 文本 |
| Prometheus 健康 | `curl -fsS http://fn.cky:19090/-/healthy` | 返回 `Prometheus is Healthy.` |
| 容器状态 | `docker compose -f docker-compose.staging.yml -f docker-compose.monitoring.yml ps` | backend / yjs / prometheus / grafana 均为 `Up` |

建议首诊顺序：`healthz -> Grafana 面板 -> docker logs -> 再决定是否重启`。

---

## 仅停止监控

```bash
docker compose -f docker-compose.staging.yml -f docker-compose.monitoring.yml stop prometheus grafana
```

---

## 日志入口（当前生产做法）

设计文档里的目标态是 OTel + Loki + Alertmanager，但当前 staging 值班仍以 Docker 容器日志和 Prometheus 面板为准。

```bash
# backend 最近 10 分钟日志
docker logs --since 10m ripple-staging-backend | tail -n 200

# yjs-bridge 最近 10 分钟日志
docker logs --since 10m ripple-staging-yjs-bridge | tail -n 200

# Prometheus / Grafana 自身日志
docker logs --since 10m ripple-monitoring-prometheus | tail -n 100
docker logs --since 10m ripple-monitoring-grafana | tail -n 100

# PostgreSQL / Redis / Neo4j 健康排查
docker logs --since 10m ripple-staging-postgres | tail -n 120
docker logs --since 10m ripple-staging-redis | tail -n 120
docker logs --since 10m ripple-staging-neo4j | tail -n 120
```

优先 grep 的关键词：`error|panic|timeout|context deadline|quota|llm|redis|websocket|origin`。

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

## 日常巡检清单

| 频率 | 巡检项 | 通过标准 | 失败后的第一动作 |
|------|--------|----------|------------------|
| 每日开工前 | `/healthz` + `/metrics` | backend / yjs 可访问，metrics 正常输出 | 查容器状态与近 10 分钟日志 |
| 每日 1 次 | Grafana `Ripple Overview` 8 个面板 | 无明显断线、无异常尖峰 | 对照告警阈值做手动复核 |
| 每次上线后 | `phase13-smoke.ps1` | 功能链路全绿 | 回滚或继续看日志定位 |
| 每周 1 次 | `grafana` / `prometheus` volume 与磁盘 | 无磁盘爆满、Prometheus 仍保留 15 天数据 | 清理旧 volume 或扩容 |

如果是发布窗口，额外确认：

1. `docker compose -f docker-compose.staging.yml -f docker-compose.monitoring.yml ps` 没有 `Restarting` 容器。
2. Grafana 面板时间窗切到 `Last 30 minutes`，确认发布后没有新的 p95 突刺。
3. `scripts/smoke/phase13-smoke.ps1 -Base http://fn.cky:18000` 至少跑一轮。

---

## 灰度期间建议观察项

7 天观察期内，每天检查：

1. **HTTP 请求速率**：是否有意外流量峰值（> 平时 3x）
2. **HTTP p95 延迟**：> 500ms 时看 DB 连接池是否满
3. **DB 连接池**：`acquired_conns / total_conns > 0.8` 时考虑扩 max_conns（默认 pgxpool.Config.MaxConns=10）
4. **LLM 调用**：调用量 > 预算时检查 `llm_calls` 表（SELECT count(*) FROM llm_calls WHERE created_at > now()-interval '24h'）

---

## 告警规则（Phase 15 引入，当前手动巡检）

| 信号 | 建议阈值 | 严重度 | 第一动作 | 后续动作 |
|------|---------|--------|----------|----------|
| backend `/healthz` 不通 | 任何时刻 | P0 | 查 `ripple-staging-backend` 日志 | 必要时重启 backend 并补跑 smoke |
| HTTP p95 延迟 | > 1000ms 持续 5min | P1 | 对照 DB 连接池趋势 | 查慢查询、抓 pprof |
| DB acquired/total | > 0.9 持续 3min | P1 | 查 backend 日志是否有 acquire timeout | 先重启 backend，再考虑调大池子 |
| LLM 调用速率 | > 10 req/s 持续 5min | P1 | 检查 provider 分布是否异常集中 | 查 `llm_calls` / 组织维度用量 |
| AI 任务积压 | `pending > 50` 持续 10min | P1 | 查 `ai_jobs` 状态与 backend 日志 | 必要时重启 backend 触发 RecoverProcessing |
| WebSocket 403 / 502 | 5min 内持续出现 | P1 | 先确认是当前日志，不是浏览器历史噪声 | 查 yjs-bridge origin 配置与 backend 可达性 |
| Prometheus / Grafana 不可用 | 任何时刻 | P2 | 查 monitoring 容器日志 | 单独重启 prometheus / grafana |

---

## pprof 诊断

```bash
# 需先在 staging 环境配置 RIPPLE_PPROF_ADDR=:6060，然后重启 backend
docker compose -f docker-compose.staging.yml up -d backend

# 若已开启：
curl http://fn.cky:6060/debug/pprof/
go tool pprof -top http://fn.cky:6060/debug/pprof/profile?seconds=30
```

---

## 常见故障 SOP

### 1. backend 5xx 飙升或 `/healthz` 失败

```bash
docker compose -f docker-compose.staging.yml ps backend
docker logs --since 10m ripple-staging-backend | tail -n 200
curl -fsS http://fn.cky:18000/healthz
```

判断：

1. 若容器已退出或反复重启，先执行：
  `docker compose -f docker-compose.staging.yml up -d backend`
2. 若容器存活但接口超时，转到下一个 SOP 看 DB / Redis / Neo4j 依赖。
3. 恢复后必须补跑：
  `powershell -ExecutionPolicy Bypass -File scripts/smoke/phase13-smoke.ps1 -Base http://fn.cky:18000`

### 2. DB 连接池打满或接口明显变慢

```bash
docker logs --since 10m ripple-staging-backend | grep -Ei 'timeout|acquire|sql|context deadline'
docker exec -i ripple-staging-postgres psql -U ripple -d ripple -c "SELECT pid, state, wait_event_type, NOW()-query_start AS age, LEFT(query, 120) AS query FROM pg_stat_activity WHERE datname='ripple' ORDER BY query_start ASC LIMIT 10;"
```

行动：

1. 先确认是不是短时流量尖峰，不要第一时间重启 Postgres。
2. 如果 backend 长时间占满连接，优先重启 backend 释放泄漏连接。
3. 若 `pg_stat_activity` 显示单条慢 SQL 持续堆积，再进入 SQL 定位与索引修复。

### 3. AI 任务积压、失败率升高或组织 AI 账单异常

```sql
SELECT COUNT(*) FROM ai_jobs WHERE status = 'pending';

SELECT id, node_id, started_at FROM ai_jobs
WHERE status = 'processing' AND started_at < NOW() - INTERVAL '5 minutes';

SELECT provider, COUNT(*) AS calls, ROUND(SUM(estimated_cost_cny)::numeric, 2) AS cost
FROM llm_calls
WHERE created_at > NOW() - INTERVAL '24 hours'
GROUP BY provider
ORDER BY calls DESC;
```

```bash
docker logs --since 15m ripple-staging-backend | grep -Ei 'ai job|llm|provider|quota|retry'
```

行动：

1. 若 `processing` 任务卡住，重启 backend 触发 `RecoverProcessing`。
2. 若单一 provider 突然飙高，先核对该 provider 是否有降级或重试风暴。
3. 若是单组织异常高用量，再调用 `GET /api/v1/organizations/{id}/llm_usage?days=7` 做人工复核。

### 4. WebSocket 403 / 502 或协作链路噪声

```bash
curl -fsS http://fn.cky:17790/healthz
docker logs --since 10m ripple-staging-yjs-bridge | tail -n 200
docker logs --since 10m ripple-staging-backend | grep -Ei 'websocket|origin|upgrade|403|502'
```

行动：

1. 先确认日志时间戳，避免把浏览器控制台里的历史错误当成当前事故。
2. 403 优先检查 `CORS_ORIGINS` 与 `YJS_BRIDGE_ALLOWED_ORIGINS` 是否包含当前 staging host。
3. 502 优先检查 backend 是否健康，再看 yjs-bridge 是否能访问 `http://backend:8000`。
4. 修复后补跑一次 smoke，确认 ws probe 已恢复。

### 5. Grafana / Prometheus 打不开

```bash
docker compose -f docker-compose.staging.yml -f docker-compose.monitoring.yml ps prometheus grafana
docker logs --since 10m ripple-monitoring-prometheus | tail -n 100
docker logs --since 10m ripple-monitoring-grafana | tail -n 100
docker compose -f docker-compose.staging.yml -f docker-compose.monitoring.yml up -d prometheus grafana
```

行动：

1. 仅重启 `prometheus grafana`，不要影响 backend / frontend。
2. 若 Grafana 恢复但历史图空白，检查 `grafana_data` / `prometheus_data` volume 是否被误删。
3. 若只是 dashboard 丢失，核对 `monitoring/grafana/provisioning/` 是否仍正确挂载。

---

## 注意事项

- `monitoring/` 目录及 compose 文件已提交 git，**不含任何密码**（密码走 `GRAFANA_PASSWORD` 环境变量）。
- Grafana 数据（图表历史）存于 `grafana_data` volume，重启不丢失；彻底清理：`docker volume rm ripple_grafana_data`。
- Prometheus 数据保留 15 天（`--storage.tsdb.retention.time=15d`）。
- 若 staging 网络名不是 `ripple_default`，编辑 `docker-compose.monitoring.yml` 的 `networks.ripple_default.name` 字段。
- 本仓库当前准入不依赖 GitHub Actions；值班验收以本地命令和 staging smoke 为准。

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

