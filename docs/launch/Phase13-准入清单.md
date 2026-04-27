# Phase 13 灰度准入清单

> 状态：联调进行中（2026-04-27 已完成首轮远端启动与 smoke）
> 适用版本：`v0.13.0` 灰度前  
> 目标：把 Phase 13 的联调、性能、稳定性、运维、回滚条件收敛成单页准入门槛。

> 最新回填：远端主机 `fn.cky` 已完成标准 staging bootstrap 与 smoke 一键验收；`backend`、`yjs-bridge`、frontend 均已通过真实 Docker 启动验证，`scripts/smoke/phase13-smoke.ps1 -Base http://fn.cky:18000` 全绿。TD-04 也已在远端 Linux 客户端完成诊断 + clean rerun：`docs/dev/TD-04-WS-loadtest-report-20260427-142214.md` 负责根因收口，`docs/dev/TD-04-WS-loadtest-report-20260427-142125.md` 记录到 `1000 / 1000` 成功、`p95 175.797ms`、`p99 192.333ms`，WS 准入门槛现已满足。2026-04-27 当天的 Redis / Neo4j / yjs-bridge 故障演练也已完成并回填到 `docs/dev/Phase13-故障演练验收记录-20260427.md`；同日补跑的全文检索 `10k` 基线 `p95 66.4085ms`、`p99 117.9482ms`，详见 `docs/dev/Phase13-全文检索10k基线-20260427.md`。批量导入 `1000` 行时延验收也已通过，详见 `docs/dev/Phase13-batch-import-1000-baseline-20260427-213021.md`。staging 非破坏回收演练、远端实际回收、回收后重新拉起与 smoke 均已通过，见 `docs/dev/Phase13-staging-teardown-20260427.md` 与 `docs/dev/Phase13-staging-rebootstrap-smoke-20260427.md`。当前剩余事项为回滚验收。

---

## 1. 功能准入

| 项目 | 门槛 | 状态 | 备注 |
|------|------|------|------|
| 注册 / 登录 | 冒烟脚本全绿 | ✅ | 2026-04-27 远端 `fn.cky` smoke 通过 |
| 建湖 / 节点 CRUD | 冒烟脚本全绿 | ✅ | 建湖、建节点已在远端 smoke 验证 |
| 全文检索 | `GET /api/v1/search` 返回结果正确 | ✅ | 远端 smoke 通过；`10k` 基线也已补测通过，见 `docs/dev/Phase13-全文检索10k基线-20260427.md` |
| 批量导入 | 1000 行导入成功率 100% | ✅ | 远端 smoke 最小批量导入链路通过；1000 行环境验收未完成 |
| API Key | `raw_key` 返回正确 | ✅ | 远端 smoke 已验证 |
| Org 邀请 | `/members/by_email` 200 / 404 路径正确 | ✅ | 远端 smoke 已验证按邮箱邀请成功 |
| 审计日志查询 | 指定资源查询成功返回 `logs` | ✅ | 远端 smoke 已验证接口契约 |

## 2. 性能准入

| 项目 | 门槛 | 状态 | 备注 |
|------|------|------|------|
| WS 跨机压测 | 1000 并发持续 30s，成功率 ≥ 99%，p95 < 200ms | ✅ | 诊断报告 `docs/dev/TD-04-WS-loadtest-report-20260427-142214.md` 已确认退化根因是建湖后未等待 lake 投影视图可读；clean rerun `docs/dev/TD-04-WS-loadtest-report-20260427-142125.md` 已收口到 `1000 / 1000` 成功、`p95 175.797ms`、`p99 192.333ms`，满足 Phase 13 准入门槛 |
| 全文检索 | 10k 节点下 p95 < 300ms | ✅ | 2026-04-27 环境验收结果：`50` 次样本 `p95 66.4085ms`、`p99 117.9482ms`；见 `docs/dev/Phase13-全文检索10k基线-20260427.md` |
| 批量导入 | 1000 行导入 5s 内回响应 | ✅ | `2026-04-27` 远端 `fn.cky` 验收：`created=1000`，`elapsed=0.1506s`，见 `docs/dev/Phase13-batch-import-1000-baseline-20260427-213021.md` |

## 3. 稳定性准入

| 项目 | 门槛 | 状态 | 备注 |
|------|------|------|------|
| Redis 中断恢复 | broker 自动恢复，数据不丢 | ✅ | 2026-04-27 演练通过，容器恢复到 `healthy`，演练后 smoke 全绿；见 `docs/dev/Phase13-故障演练验收记录-20260427.md` |
| Neo4j 中断恢复 | 检索与图写入恢复正常 | ✅ | 2026-04-27 演练通过，容器恢复到 `healthy`，演练后 smoke 全绿；见 `docs/dev/Phase13-故障演练验收记录-20260427.md` |
| yjs-bridge 重启 | 协作链路恢复，后端健康不受影响 | ✅ | 2026-04-27 演练通过；修复启动镜像与环境变量错配后，`/healthz` 返回 `200 ok`，演练后 smoke 全绿；见 `docs/dev/Phase13-故障演练验收记录-20260427.md` |

## 4. 运维准入

| 项目 | 门槛 | 状态 | 备注 |
|------|------|------|------|
| staging 启动 | `bootstrap-staging.ps1` 在干净机器可执行 | ✅ | 已在 `fn.cky` 上完成标准 bootstrap 一键验收，`docker compose down && up -d --build` 后 `/healthz` 与 `phase13-smoke.ps1` 全绿；`migrate` 空库探测与 frontend Docker build/访问均已验证 |
| staging 回收 | `teardown-staging.ps1` 可清理容器与卷 | ✅ | 已完成非破坏 dry-run 与远端实际回收；实际回收记录见 `docs/dev/Phase13-staging-teardown-20260427.md` |
| 指标 | `/metrics` 可抓取 | ✅ | 已用于 TD-04 补采样，Prometheus 文本格式可直接 `curl` 抓取 |
| 手册 | 非作者按文档独立完成一次拉起与回收 | ✅ | 已复现 dry-run、实际回收、回收后重新拉起与 smoke；端口冲突处理已回填到手册 |

## 5. 回滚准入

| 项目 | 门槛 | 状态 | 备注 |
|------|------|------|------|
| 应用回滚 | 能回退到上一稳定镜像 / 提交 | ☐ | 本轮按执行策略暂缓，待独立回滚窗口；见 `docs/dev/Phase13-回滚验收待办-20260427.md` |
| 数据库回滚 | 至少完成一次 down 演练 | ☐ | 本轮按执行策略暂缓，待独立回滚窗口；见 `docs/dev/Phase13-回滚验收待办-20260427.md` |
| 演练记录 | 演练时间、步骤、结果可追溯 | ☐ | 本轮暂未形成回滚演练记录，待完成后补档 |

## 6. 决策签字

| 角色 | 结论 | 日期 | 备注 |
|------|------|------|------|
| PM | ☐ 通过 / ☐ 驳回 |  |  |
| QA | ☐ 通过 / ☐ 驳回 |  |  |
| 决策者 | ☐ 通过 / ☐ 驳回 |  |  |