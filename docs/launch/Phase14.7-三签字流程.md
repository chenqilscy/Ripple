# Phase 14.7 · 三签字准入流程（PM / QA / 决策者）

> 状态：📝 流程稿（2026-04-28）
> 适用范围：Phase 14 整体准入收口（含 14.1-14.6 全部子项）
> 配套文档：[Phase14-准入清单.md](../launch/Phase14-准入清单.md)

---

## 1. 流程概览

Phase 14 准入采用**三签字制**，对应 AGENTS.md §「十角色团队流程」中 PM / QA / 决策者三角色。每签字必须基于**可验证的输出物**，禁止空头承诺。

```
开发完成 → 实现者自检 → 反方 5 轮代码审查 → QA 验证 → 体验方试用
   ↓
PM 签字（功能闭环）→ QA 签字（质量门）→ 决策者签字（最终裁决）
   ↓
合并到 main / 进入灰度
```

## 2. PM 签字（功能闭环）

**职责**：确认 Phase 14 所有子项的功能需求都已实现且符合产品目标。

**输入物**：
1. Phase14-准入清单.md §1「功能准入」全部 ✅
2. Settings 子 Tab E2E 跑通（`npm.cmd run e2e -- e2e/settings-tabs.spec.ts`）3 用例全 PASS
3. 体验方试用反馈记录（手动操作 5 个 Tab + 灰度 CRUD + OWNER 撤销）

**签字模板**：
```
角色：PM
日期：YYYY-MM-DD
结论：✅ 通过 / 🟡 待补 / ❌ 拒绝
依据：
  - acceptance 输出（附 phase14-6-acceptance.ps1 全 PASS 输出截图）
  - E2E 输出（npm.cmd run e2e）
  - 体验方反馈记录（docs/dev/Phase14-体验反馈-YYYYMMDD.md）
备注：（如有未闭环项，列出阻塞原因）
```

## 3. QA 签字（质量门）

**职责**：验证质量门全部满足。

**输入物**：
1. 后端 `go test -race -count=1 ./...` 0 失败 0 race（**已验证**：2026-04-28 全 10 包 PASS，最长 7.4s）
2. 前端 `npm.cmd run lint`、`npm.cmd test`、`npm.cmd run build` 全绿
3. Settings E2E 3 用例 PASS
4. Staging healthz + /yjs probe 通过（`phase14-6-acceptance.ps1` 不带 -SkipStaging）
5. Staging cleanup dry-run 输出可控（仅 smoke 前缀候选）

**签字模板**：
```
角色：QA
日期：YYYY-MM-DD
结论：✅ 通过 / 🟡 待补 / ❌ 拒绝
依据：
  - race test 输出（附 race.log tail 30）
  - E2E 输出
  - staging probe 输出
  - cleanup dry-run 候选列表
备注：（如有性能 / 安全 / 数据隔离遗留问题，列出）
```

## 4. 决策者签字（最终裁决）

**职责**：在团队意见分歧 / 重大变更 / 灰度准入时做最终裁决。

**输入物**：
1. PM 签字记录
2. QA 签字记录
3. 反方代码审查 5 轮全部完成（commit message 中 `Co-authored-by` 或 `audit_logs` 表查询）
4. 自学习日志已追加本 Phase 关键经验（`docs/system-design/自学习日志.md`）

**签字模板**：
```
角色：决策者
日期：YYYY-MM-DD
结论：✅ 准入 / 🔄 返工 / ❌ 拒绝
依据：
  - PM 签字（链接）
  - QA 签字（链接）
  - 5 轮审查记录（commit 列表 + 审查输出）
  - 自学习日志条目
后续行动：
  - （如准入）进入灰度，监控 X 天后转 GA
  - （如返工）列出必须修复的问题清单 + ETA
  - （如拒绝）说明原因 + 是否需要重新立项
```

## 5. 当前 Phase 14 签字状态（占位）

| 角色 | 结论 | 日期 | 备注 |
|------|------|------|------|
| PM | 待定 | — | 待 phase14-6-acceptance.ps1 完整跑（含 staging）+ 体验方反馈 |
| QA | 🟡 部分通过 | 2026-04-28 | 后端 race test 全包 PASS、前端三项全绿、Settings E2E 3/3 PASS；待 staging cleanup -Apply 真实演练（阻塞 Neo4j 真实密码） |
| 决策者 | 待定 | — | 待 PM/QA 全 ✅ |

## 6. 阻塞项

1. **Neo4j 真实密码**：cleanup -Apply 真实演练阻塞，需老板提供 staging Neo4j 密码
2. **体验方反馈**：尚未做完整手动操作（Settings 5 Tab + 灰度 CRUD + OWNER 撤销）

## 7. 下一步行动

- [ ] 老板提供 Neo4j 真实密码 → 跑 cleanup -Apply → QA 补签 staging cleanup 项
- [ ] 体验方手动试用 → PM 签字
- [ ] PM + QA 全 ✅ → 决策者签字 → 进入灰度
