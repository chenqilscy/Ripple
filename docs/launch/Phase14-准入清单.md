# Phase 14 准入清单（去 CI 化版）

> 状态：🟡 进行中（Phase 14.4 / 14.5 / 14.6 收口阶段）  
> 适用版本：`v0.14.x` 灰度前  
> 目标：把 Phase 14 的功能闭环、性能门、安全门、运维门、数据隔离门收敛成单页准入清单。  
> **重要规则（自 2026-04-28 起）**：本仓库已停用 GitHub Actions（详见 `AGENTS.md` §「CI / 准入策略」）。所有质量门以**本地命令** + **staging 手动 / 脚本化 smoke** 为准，**禁止**以 “GitHub Actions 绿线” 作为门槛。

---

## 1. 功能准入

| 项目 | 门槛 | 验证命令 / 入口 | 状态 |
|------|------|-----------------|------|
| 平台管理员 RBAC | OWNER 可授予 / 撤销 ADMIN，无管理员账号需 bootstrap 自助接管 | `go test -race -count=1 ./internal/api/http/...`（`TestPlatformAdminRouter*`）+ Settings 子 Tab E2E | ✅ |
| API Key 不继承平台权限 | API Key 调用 `/admin/*` 返回 403 | `go test -race -count=1 ./internal/api/http/...`（`TestApiKeyAdminForbidden`） | ✅ |
| 灰度名单 CRUD | 平台管理员可新增 / 删除灰度邮箱；命中名单允许注册 | `npm.cmd run e2e -- e2e/settings-tabs.spec.ts`（含「灰度名单可以新增并删除一条记录」用例） | ✅ |
| 审计日志 | `platform_admin.grant` / `platform_admin.revoke` 写入审计 | `go test -race -count=1 ./internal/service/...` + Settings 审计日志 Tab 加载 | ✅ |
| WebSocket Origin 校验 | `OriginPatterns` 仅放行 staging host pattern | `staging-cleanup-smoke.ps1` 同源；`scripts/smoke/phase13-smoke.ps1` ws probe | ✅ |
| Settings 子 Tab 切换 | 5 个 Tab 加载无白屏 | `npm.cmd run e2e -- e2e/settings-tabs.spec.ts` | ✅ |

## 2. 性能准入

| 项目 | 门槛 | 验证命令 | 状态 |
|------|------|----------|------|
| 后端单测稳定性 | `-race -count=1` 全包通过且无数据竞争 | `cd backend-go ; $env:GOTOOLCHAIN="local" ; go test -race -count=1 ./...` | 🟡 抽样通过（`./internal/api/http` 3.7s PASS）；完整跑作为发布前 gate |
| 前端构建 | `npm.cmd run build` 无 TypeScript / Vite 错误 | `cd frontend ; npm.cmd run build` | ✅ |
| WebSocket 连接 | `/yjs` 缺 lake 参数返回 400 | `phase14-6-acceptance.ps1`（`staging: /yjs returns 400` 步骤） | ✅ |

## 3. 安全准入

| 项目 | 门槛 | 验证 | 状态 |
|------|------|------|------|
| OWASP A01（鉴权失效）| 未登录 / 普通用户访问 `/admin/*` 返回 401 / 403 | 单测 `TestPlatformAdminRouter_Forbidden_*` | ✅ |
| OWASP A02（敏感数据）| 远端凭据（SSH / Neo4j / API Key）不写入仓库 | `git grep -E 'cky\.my\.2|placeholder_key'` 应无命中（已确认） | ✅ |
| 平台权限不可越权 | API Key 不能继承平台管理员权限 | `TestApiKeyAdminForbidden`、Settings UI 验证 | ✅ |
| 审计完整 | 所有平台权限变更写审计 | `audit_logs` 表查询 + 单测 | ✅ |

## 4. 运维准入

| 项目 | 门槛 | 验证命令 | 状态 |
|------|------|----------|------|
| 一键准入脚本 | `phase14-6-acceptance.ps1` 可单步 / 全量执行 | `powershell -File scripts/smoke/phase14-6-acceptance.ps1 [-SkipBackend] [-SkipFrontend] [-SkipE2E] [-SkipStaging] [-IncludeStagingCleanup] [-StagingCleanupApply]` | ✅ |
| Staging 数据清理 | smoke 数据可一键 dry-run / 真实清理 | `powershell -File scripts/smoke/staging-cleanup-smoke.ps1 [-Apply]`，dry-run 已验证候选列表（PG: 3 条 phase13+ 用户） | ✅ dry-run；🟡 真实清理待 Neo4j 真实密码 |
| Staging healthz | `http://fn.cky:18000/healthz` 返回 `ok` | `phase14-6-acceptance.ps1`（`staging: backend healthz` 步骤） | ✅ |
| 文档 | Phase 14 设计文档 / 运营化设计已发布 | `docs/dev/Phase14-*.md`、`docs/dev/Phase14-6-运营化回归与准入设计-20260428.md` | ✅ |

## 5. 数据隔离准入

| 项目 | 门槛 | 验证 | 状态 |
|------|------|------|------|
| Lake 维度隔离 | 用户只能看到自己 owner / 被邀的 lake | `TestLakeService_*`、Settings UI 验证 | ✅ |
| 平台管理员可见性 | OWNER / ADMIN 才能列出全平台用户 / 组织 | `TestAdminOverview_Forbidden_*` | ✅ |
| API Key 范围 | API Key 不能跨用户访问数据 | `TestApiKey_Scope_*` | ✅ |

## 6. CI / 准入策略变更（自 2026-04-28 起）

| 旧策略 | 新策略 |
|--------|--------|
| GitHub Actions `ci.yml` 绿线 | ❌ 禁用（`.github/workflows/ci.yml` 已删除，禁止恢复） |
| `gh run` 查询作为门禁 | ❌ 禁用 |
| “CI 跑通即可合并” | ✅ 改为：本地 `phase14-6-acceptance.ps1` 全 PASS + staging smoke 全绿 + 决策者签字 |

详见 `AGENTS.md` §「CI / 准入策略（自 2026-04-28 起）」与 `docs/system-design/自学习日志.md`（2026-04-28 条目）。

## 7. 决策签字

| 角色 | 结论 | 日期 | 备注 |
|------|------|------|------|
| PM | 待定 | — | 待 Neo4j 真实清理演练 + 完整 race test 通过后签字 |
| QA | 待定 | — | smoke + E2E 全绿后签字 |
| 决策者 | 待定 | — | 准入门全部闭环后签字 |

---

## 附：一键准入命令速查

```powershell
# 完整本地准入（不含 staging）
$env:GOTOOLCHAIN="local"
powershell -File scripts/smoke/phase14-6-acceptance.ps1 -SkipStaging

# 加 staging healthz / yjs probe
$env:RIPPLE_STAGING_BASE="http://fn.cky:18000"
$env:RIPPLE_STAGING_FRONTEND_BASE="http://fn.cky:14173"
powershell -File scripts/smoke/phase14-6-acceptance.ps1

# 加 staging 数据清理（dry-run）
$env:RIPPLE_STAGING_SSH_HOST="fn.cky"
$env:RIPPLE_STAGING_SSH_USER="admin"
$env:RIPPLE_STAGING_NEO4J_PASSWORD="<真实 neo4j 密码>"
powershell -File scripts/smoke/phase14-6-acceptance.ps1 -IncludeStagingCleanup

# 加 staging 真实清理（破坏性，需谨慎）
powershell -File scripts/smoke/phase14-6-acceptance.ps1 -IncludeStagingCleanup -StagingCleanupApply
```
