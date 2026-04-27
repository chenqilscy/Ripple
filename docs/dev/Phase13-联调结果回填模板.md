# Phase 13 联调结果回填模板

> 用途：在第一次真实 Docker 联调执行后，把结果同步回任务清单、准入清单与问题列表。

---

## 1. 执行信息

| 字段 | 内容 |
|------|------|
| 执行日期 |  |
| 执行人 |  |
| 机器 |  |
| 操作系统 |  |
| Docker 版本 |  |
| Git commit |  |

## 2. 执行命令

```powershell
./scripts/bootstrap-staging.ps1
./scripts/smoke/phase13-smoke.ps1 -Base http://127.0.0.1:18000
./scripts/drill-staging.ps1 -Scenario redis -DurationSeconds 15
./scripts/drill-staging.ps1 -Scenario neo4j -DurationSeconds 15
./scripts/drill-staging.ps1 -Scenario yjs-bridge -DurationSeconds 15
./scripts/teardown-staging.ps1
```

## 3. 结果汇总

| 项目 | 结果 | 备注 |
|------|------|------|
| bootstrap | ☐ 通过 / ☐ 失败 |  |
| smoke | ☐ 通过 / ☐ 失败 |  |
| redis 故障演练 | ☐ 通过 / ☐ 失败 |  |
| neo4j 故障演练 | ☐ 通过 / ☐ 失败 |  |
| yjs-bridge 故障演练 | ☐ 通过 / ☐ 失败 |  |
| teardown | ☐ 通过 / ☐ 失败 |  |

## 4. 证据

- healthz 截图 / 输出：
- smoke 输出：
- 故障演练输出：
- 若失败，相关日志片段：

## 5. 回填动作

1. 更新 `docs/system-design/整体任务清单.md`
2. 更新 `docs/launch/Phase13-准入清单.md`
3. 若失败，新增修复任务或技术债条目