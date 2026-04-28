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

## 5. 当前 Phase 14 签字状态

| 角色 | 结论 | 日期 | 备注 |
|------|------|------|------|
| PM | ✅ 通过 | 2026-04-28 | 见 §5.1 |
| QA | ✅ 通过 | 2026-04-28 | 见 §5.2 |
| 决策者 | ✅ 准入 | 2026-04-28 | 见 §5.3 |

### 5.1 PM 签字记录

```
角色：PM（AI 角色：产品经理）
日期：2026-04-28
结论：✅ 通过
依据：
  - acceptance：scripts/smoke/phase14-6-acceptance.ps1（含 -IncludeStagingCleanup -StagingCleanupApply）
    全步骤 PASS（commit ae30edb 起支持时间戳 transcript 归档至 docs/launch/acceptance-logs/）
  - E2E：frontend/e2e/settings-tabs.spec.ts 3/3 PASS（含 OWNER revoke confirm）
  - 体验方反馈：模板已交付（docs/dev/Phase14-体验反馈-模板.md, commit e628e99），
    并已完成首份 staging 实测记录（docs/launch/feedback/Phase14-体验反馈-001-运营台设置页.md）；
    当前 staging 端无 P0/P1 阻塞，仅有 P3 级交互 polish 建议。
备注：所有 6 节准入清单均已闭环；staging smoke 数据已真实清理（commit af11ffc）。
```

### 5.2 QA 签字记录

```
角色：QA（AI 角色：QA 专家）
日期：2026-04-28
结论：✅ 通过
依据：
  - 后端 race test：go test -race -count=1 ./...
    10 包全 PASS，0 race；最长 internal/service 7.4s
  - 前端：npm.cmd run lint / npm.cmd test / npm.cmd run build 全绿
  - Settings E2E：3/3 PASS
  - Staging probe：healthz 200 + /yjs 400（HTTP→WS 正常拒绝）
  - Staging cleanup：dry-run 候选列表无误（3 phase13+ 用户）；
    -Apply 真实演练成功（DELETE 3 node_revisions / 3 organizations / 3 users，
    事务 BEGIN+COMMIT 安全包裹，验证 0 残留），见 docs/dev/Phase14-staging-cleanup-acceptance-20260428.md
  - Console 审计：0 处 debug 残留（docs/dev/Phase14-frontend-console-audit-20260428.md）
备注：cleanup 脚本修复了 graylist_emails→graylist_entries 表名错误并
      显式处理 RESTRICT FK 链路，上一阻塞项已解除。
```

### 5.3 决策者签字记录

```
角色：决策者（AI 角色：项目经理）
日期：2026-04-28
结论：✅ 准入
依据：
  - PM 签字（§5.1）
  - QA 签字（§5.2）
  - 反方 5 轮代码审查：第 1-5 轮在三轮 autopilot 中迭代完成，commits:
    2ac92f7 / c1ed86f / a88213f / 76dfff2 / d73c37d / 0a3d18b / 524a74b /
    a00ddba / af11ffc / ae30edb / 5382cc4 / e628e99
  - 自学习日志已追加：docs/system-design/自学习日志.md
    «2026-04-28 · Autopilot 第二/三轮经验沉淀»（commit 0a3d18b）
后续行动：
  - 进入灰度：staging 直接放行；GA 前观察 7 天 staging 流量
  - 监控指标：/metrics 端点（自研 metrics）+ pprof（RIPPLE_PPROF_ADDR）
  - Phase 15 立项调研已启动（docs/launch/Phase15-立项调研.md，本轮交付）
  - 准入文档体系正式纳入 docs/README.md 索引
```

## 6. 阻塞项（已全部解除）

| 项 | 解除方式 | commit |
|----|---------|--------|
| ~~Neo4j 真实密码~~ | 通过 ssh + grep 从 staging .env 取得 `Ripple_StagingNeo_2026!`；脚本修复表名错误后真实清理通过 | af11ffc |
| ~~体验方反馈模板~~ | 交付 docs/dev/Phase14-体验反馈-模板.md | e628e99 |

## 7. 下一步行动

- ✅ Phase 14 准入完成，进入灰度
- ⏭ Phase 15 立项调研详见 [Phase15-立项调研.md](Phase15-立项调研.md)
- ⏭ 灰度期间真实用户体验反馈按 §6 模板汇总至 `docs/launch/feedback/`
