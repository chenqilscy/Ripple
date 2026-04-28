# Phase 14 规划草案

> 文档层级：`docs/system-design/*.md`，承接 `docs/system-design/整体任务清单.md`。
> 状态：草案已裁决主线；Sprint 14.1 数据模型与 Sprint 14.2 配额拦截点已实现。

## 1. 阶段定位

Phase 13 已完成可信交付闭环：远端 staging、性能基线、故障演练、实际回收、回滚验收与 CI 绿线。Phase 14 不应继续做零散补丁，而应从“可发布”转向“可运营 / 可增长”。

建议定位：**商业化前的产品化与运营化阶段**。

## 2. 候选方向

| 方向 | 价值 | 风险 | 建议 |
|------|------|------|------|
| P14-A：Org 配额与计费基础 | 承接多租户，形成商业化入口 | 涉及计费模型与权限边界 | 推荐优先 |
| P14-B：PWA 离线写队列 | 提升真实用户弱网体验 | 冲突合并与数据一致性复杂 | 第二优先 |
| P14-C：GraphQL / 聚合 API | 降低前端多请求复杂度 | 当前 REST 尚未成为瓶颈 | 暂缓 |
| P14-D：管理员运营台 | 支持灰度名单、审计、配额、用户状态 | 需要清晰 RBAC | 可与 P14-A 合并 |
| P14-E：公开分享体验完善 | 外链分享页、访问统计、撤销与限流可视化 | 需谨慎处理隐私 | 可作为小切片 |

## 3. 推荐主线：P14-A Org 配额与运营台

### 目标

把现有组织能力从“协作模型”推进到“运营模型”：

- 组织配额：成员数、湖数量、节点数量、附件容量、API Key 数量；
- 运营台：查看组织、用户、配额使用、审计日志；
- 灰度名单：控制 Phase 13 后的内测准入；
- 限额拒绝：超限时返回明确错误，不静默失败。

### 验收标准

1. 组织 owner/admin 可看到配额使用情况；
2. 超出配额时，建湖 / 邀请成员 / 上传附件 / 创建 API Key 返回明确 `429` 或 `403` 风格错误；
3. 管理员可调整组织配额；
4. 所有配额变更写入 audit log；
5. CI + 单测 + 集成测试覆盖主要配额路径。

## 4. Sprint 拆分

### Sprint 14.1：配额数据模型

- ✅ 新增 `org_quotas` / `org_usage_snapshots` 表；
- ✅ 定义默认免费配额；
- ✅ OrgService 增加 `GetQuota` / `UpdateQuota` / `CheckQuota`；
- ✅ 单测覆盖权限、边界值、审计写入与溢出保护；
- ✅ HTTP 第一切片：`GET/PATCH /api/v1/organizations/{id}/quota`。

评审记录：`docs/dev/Phase14-A-技术方案评审-20260427.md`。

### Sprint 14.2：配额拦截点

- ✅ 湖归属组织绑定：`SetLakeOrg` 校验 `max_lakes`；
- ✅ 邀请 / 添加成员：`AddMember` 校验 `max_members`；
- ✅ 上传附件：节点归属组织下校验 `max_attachments` 与 `max_storage_mb`；
- ✅ 创建 API Key：提供 `org_id` 时校验 `max_api_keys`；
- ✅ 节点创建与批量导入：校验 `max_nodes`；
- ✅ 前端 Org 运营面板新增 `Quota` Tab，可读 / 可调配额。

交付记录：`docs/dev/Phase14-2-配额拦截点-20260428.md`。

### Sprint 14.3：运营台 API 与 UI

- ✅ 最小切片已完成：组织 quota API 已返回 usage 聚合（members / lakes / nodes / attachments / api_keys / storage_mb）；
- ✅ `GET /api/v1/organizations/{id}/overview` 已返回组织信息、quota/usage 与最近 quota audit；
- ✅ 前端 Org `Quota` Tab 已展示已用量 / 限额进度条，并补充最近 quota audit 摘要；
- ✅ 已完成 staging 回归：`quota` / `overview` API 实流通过，前端 `Quota` Tab 已在 `fn.cky:14173` 实机展示 `2 / 3` 等 usage 值与 `Recent quota audits` 审计摘要；
- ✅ 已补齐列表总览 API：`GET /api/v1/organizations/overview` 返回当前用户组织列表的 `organization + quota/usage + recent_quota_audits` 聚合视图；
- ✅ 前端 Org 列表页已切到 overview-list 数据源，列表态直接展示 `Members/Lakes/Nodes` usage 摘要与 `Latest audit` 时间；
- ✅ 已完成本地 + staging 实流回归：`http://localhost:5234` 与 `http://fn.cky:14173` 均可在组织列表直接看到 usage 摘要与最近 quota audit；
- ✅ 审计日志联动已补齐：前端 `Quota` Tab 可直接展开 `AuditLogViewer`，自动查询 `resource_type=org_quota` 的完整审计记录；
- ✅ 灰度名单入口已补齐：后端新增 `/api/v1/admin/graylist` CRUD 与注册灰度校验，前端 `Settings` 页已提供邮箱增删入口；本地 `go test -run "Graylist|Auth" ./internal/service/... ./internal/api/http/...` 与 `npm run build` 通过；
- ✅ 管理员总览已补齐：`GET /api/v1/admin/overview` 聚合平台组织数、用户数、灰度名单数与组织 quota usage / latest audit；前端 `Settings` 管理员总览已完成 `recent_quota_audits` / `organizations` 缺省容错并在 staging 实机验证无崩溃；
- ✅ 灰度名单 UI staging smoke 已完成新增 / 移除闭环，移除后列表恢复为空；

### Sprint 14.4：发布与体验闭环

- ✅ 已启动：发布与体验闭环执行清单见 `docs/dev/Phase14-4-发布体验闭环-20260428.md`；
- 体验方试用：按普通用户 / 组织管理员 / 平台管理员三类脚本执行；
- 配额文档：补默认免费配额、触发点、错误语义与运营调整方式；
- 运营 SOP：灰度名单、管理员总览、配额调整、审计追踪与回滚；
- Phase 14 准入清单：后端 race 测试、前端构建、staging smoke、敏感信息与性能观察项。

### Sprint 14.5：发布后优化与规则收紧

- ✅ Admin Overview 聚合优化：已将 quota / usage / latest audit 读取改为可选批量接口，PG / Neo4j 仓库提供批量计数，避免平台总览按组织 N+1 查询；
- ✅ 平台管理员 RBAC：设计草案见 `docs/dev/Phase14-5-RBAC-设计草案-20260428.md`；14.5-A/B 已完成 `platform_admins` 数据表、仓库、集中鉴权 helper 与 OWNER 管理 API，继续兼容 `RIPPLE_ADMIN_EMAILS`；
- 前端 lint 规则收紧：当前已恢复 0 error / 0 warning gate，后续按组件分批恢复 Fast Refresh 与 Hooks 依赖规则；
- ✅ 灰度名单审计增强：灰度名单新增 / 更新 / 删除写入 `audit_logs`，`resource_type=graylist`，便于运营追溯；
- 真实体验补录：补普通用户超限拦截、组织管理员配额调整、平台管理员灰度名单三条完整操作录像 / 记录。

### Sprint 14.6：运营化回归与准入 Gate

- ✅ 已启动设计：见 `docs/dev/Phase14-6-运营化回归与准入设计-20260428.md`；
- ✅ WebSocket 回归：LakeWS 与 yjs-bridge 已统一将完整 CORS URL 转为 `nhooyr` 所需 host pattern，staging `/api/v1/lakes/{id}/ws` 与 `/yjs/<room>` smoke 均返回 `101 Switching Protocols`；
- ✅ Settings E2E：新增 Playwright 覆盖 Settings 子 Tab（管理员总览 / 平台管理员 / API Key / 灰度名单 / 审计日志）；
- 后续：将 staging smoke 数据清理脚本与 Phase 14 准入清单固化为可重复执行门禁。

## 5. 暂缓项

- GraphQL：当前 REST + 聚合 handler 仍足够；待前端页面出现明显 N+1 请求问题再启动。
- 离线写：价值高，但需要冲突解决设计；建议在 P14-A 稳定后作为 P14-B 单独立项。

## 6. 决策建议

建议 Phase 14 立项为：**Org 配额与运营台**。

原因：

1. 直接承接 Phase 12 多租户与 Phase 13 灰度准入；
2. 与商业化模型强相关；
3. 风险可控，可用现有 Go/React/PG 栈完成；
4. 能自然带出灰度名单、审计、限流与运营 SOP。
