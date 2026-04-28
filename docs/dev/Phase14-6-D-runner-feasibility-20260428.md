# Phase 14.6-D · 自托管 CI Runner 可行性调研

> 状态：📝 调研稿（2026-04-28）
> 背景：本仓库自 2026-04-28 起停用 GitHub Actions（详见 `AGENTS.md` §「CI / 准入策略」）。本文档评估**自托管 runner**的可行性，作为可选的自动化补充——**前提**：自托管不能再变成另一个收费黑洞，必须可控。

---

## 1. 调研动机

| 问题 | 现状 |
|------|------|
| GitHub Actions 收费 | ❌ 已禁用 |
| 本地 `phase14-6-acceptance.ps1` | ✅ 可一键运行，但需要人执行 |
| 多人协作时谁跑准入门 | 🟡 当前依赖 dev 本地，缺少 PR / 合并触发 |
| 历史 CI 绿线门槛丢失 | 🟡 改为本地命令 + staging 脚本，但缺自动化触发 |

**核心问题**：完全去 CI 后，PR / 合并准入只能依赖人工 + 本地脚本，存在以下风险：
- 提交者跳过 acceptance 直接 push（无强制门）
- 多人并发开发时，本地环境差异掩盖问题
- 紧急回滚时缺少自动化构建产物

## 2. 候选方案对比

### 方案 A：Gitea Actions（自托管）

**优点**：
- 与 GitHub Actions YAML 100% 兼容，迁移成本最低
- 自托管在内网，无外部费用
- 可与 Gitea 仓库镜像同步，PR 触发原生支持

**缺点**：
- 需额外维护一台 Gitea 实例（Docker 单容器即可）
- 需镜像同步（GitHub → Gitea，单向 webhook 或定时 pull）
- runner 仍需一台 Linux 主机（可复用 `fn.cky`）

**资源开销**：
- Gitea: ~200MB 内存 / 1GB 磁盘
- act_runner: ~100MB 内存 / 按 job 临时磁盘
- 估算总成本：可复用 staging 主机，**无新增云成本**

**风险**：
- 镜像同步延迟可能误判 PR 状态
- Gitea Actions 生态相对小众，部分 marketplace action 不可用

### 方案 B：本地 PowerShell + Pre-commit hook

**优点**：
- 0 额外基础设施
- 提交者必跑（git hook 强制）
- 与现有 `phase14-6-acceptance.ps1` 直接复用

**缺点**：
- 跑全包 race test 太慢（>1 分钟），开发者体验差
- 易被 `--no-verify` 跳过（违反 AGENTS.md 安全规则但可绕开）
- 无法做集中报告 / 历史趋势

**适用场景**：作为最后一道本地把关，不替代 CI。

### 方案 C：自托管 runner（GitHub Actions runner，但跑在自己机器上）

**优点**：
- 仍可用现有 GitHub UI 看 PR 状态
- 不计费（runner 工时不算 GitHub Actions 配额）

**缺点**：
- 仍需在 `.github/workflows/*.yml` 写 workflow——**违反** AGENTS.md 「禁止新增 workflow」规则
- 暴露内网 runner 给 GitHub，安全敏感
- 与本规则冲突，**不推荐**

### 方案 D：Drone CI / Woodpecker CI（自托管）

**优点**：
- 轻量级，单二进制
- 与 Gitea / Gogs 集成好

**缺点**：
- 与 GitHub Actions YAML 不兼容，需重写 pipeline
- 维护成本与 Gitea Actions 相当但语法学习曲线更陡

## 3. 推荐方案

**当前阶段（Phase 14）**：**方案 B + 手动执行**
- 不引入新基础设施
- 依赖 `phase14-6-acceptance.ps1` 作为合并前必跑脚本
- 在 Phase14 准入清单中明确「PM/QA/决策者三签字」流程，签字时需附 acceptance 输出截图

**Phase 15 演进**：评估**方案 A（Gitea Actions）**
- 触发条件：团队规模 > 3 人 / PR 频率 > 5/天 / 故障频次 > 1/月
- 实施估算：
  - 在 `fn.cky` 部署 Gitea + act_runner（Docker compose）
  - GitHub → Gitea 单向镜像同步（每 5 分钟 pull）
  - 把 `phase14-6-acceptance.ps1` 改写为 Linux bash 版本，作为 Gitea Actions 入口
  - 评审周期：1 周

## 4. 决策门槛

| 触发指标 | 决策 |
|---------|------|
| 团队规模 ≤ 2 人 | 维持方案 B |
| 团队规模 3-5 人 | 启动方案 A 试点 |
| PR / 合并频次 > 10/天 | 必须方案 A |
| 出现 1 次因「提交跳过 acceptance」的线上故障 | 立即启动方案 A |

## 5. 当前 TODO

- [ ] 在 `phase14-6-acceptance.ps1` 增加 `-Strict` 模式：失败时返回明确的非 0 退出码 + 输出文件供签字附加（**已部分实现**：`exit 1` + summary table）
- [ ] 在 `docs/launch/Phase14-准入清单.md` 增加「acceptance 输出存档」要求
- [ ] Phase 15 立项时评估方案 A 试点

## 6. 决策签字

| 角色 | 结论 | 日期 |
|------|------|------|
| 架构师（正方） | 推荐方案 B（当前阶段） | 2026-04-28 |
| 审查员（反方） | 同意，但需在 Phase 15 强制评估方案 A | 2026-04-28 |
| 决策者 | 待 PM 签字后 final | — |
