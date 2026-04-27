# Phase 13 故障演练验收记录模板

> 用途：记录 staging 环境下 Redis / Neo4j / yjs-bridge 故障注入与恢复结果。

---

## 基本信息

| 字段 | 内容 |
|------|------|
| 演练日期 |  |
| 演练人 |  |
| 环境 | staging |
| commit / tag |  |
| 演练脚本 | `scripts/drill-staging.ps1` |

## 场景记录

| 场景 | 注入方式 | 注入时长 | 观察项 | 结果 | 备注 |
|------|----------|----------|--------|------|------|
| Redis 中断 | `-Scenario redis` |  | broker / presence / healthz | ☐ 通过 / ☐ 失败 |  |
| Neo4j 中断 | `-Scenario neo4j` |  | 搜索 / 图写入 / healthz | ☐ 通过 / ☐ 失败 |  |
| yjs-bridge 中断 | `-Scenario yjs-bridge` |  | 协作恢复 / healthz | ☐ 通过 / ☐ 失败 |  |

## 统一观察项

1. 注入期间 `healthz` 是否瞬时失败，恢复后是否回到 `ok`。
2. 注入恢复后 `phase13-smoke.ps1` 是否能再次通过。
3. 后端日志是否存在持续错误而非瞬时错误。
4. 是否需要人工干预才能恢复。

## 结论

| 维度 | 结论 |
|------|------|
| 可恢复性 | ☐ 通过 / ☐ 失败 |
| 数据完整性 | ☐ 通过 / ☐ 失败 |
| 运维可操作性 | ☐ 通过 / ☐ 失败 |

## 后续动作

- [ ] 更新 `docs/launch/Phase13-准入清单.md`
- [ ] 更新 `docs/system-design/整体任务清单.md`
- [ ] 若失败，补一个修复任务与 owner