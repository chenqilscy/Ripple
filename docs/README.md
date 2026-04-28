# 「青萍 (Ripple)」文档地图

> 风起于青萍之末，浪成于微澜之间。
> 本文档是全部资料的单一入口，按 **品牌 / 用户 / 系统 / 联调发布** 四层组织。

---

## 一、阅读路径推荐

| 你的角色 | 推荐顺序 |
| :--- | :--- |
| **新人 / 第一次了解** | 品牌与产品概念 → 用户故事 → 本文档 → 快速上手 |
| **产品 / 设计** | 品牌与产品概念 → 造浪池交互视图 → 节点与连线状态机 → 用户故事 |
| **后端开发** | 技术架构总览 → 数据模型与权限设计 → API网关设计 → 多人实时协作 → 测试策略 → 快速上手 |
| **前端 / 图形** | 造浪池交互视图 → 节点与连线状态机 → 流体动效引擎 → 快速上手 |
| **联调 / 运维** | 快速上手 → Phase13 联调与回滚手册 → Phase13 准入清单 |
| **架构评审 / 老板** | 技术架构总览 → 用户故事 → 整体任务清单 |

---

## 二、文档分层索引

### Layer 0 · 品牌与叙事 (`docs/pinpai/`)
| 文档 | 一句话用途 |
| :--- | :--- |
| [01-品牌与产品概念](pinpai/01-品牌与产品概念.md) | **品牌圣经**：起源 / 隐喻 / 三层架构 / 交互词典 / 配色 |
| [02-对外汇报话术](pinpai/02-对外汇报话术.md) | 文艺 / 逻辑 / 商业三种汇报口径 + "青萍与马具"反思 |

### Layer 1 · 用户故事 (`docs/user-story/`)
| 文档 | 覆盖场景 |
| :--- | :--- |
| [story.md](user-story/story.md) | 10 个故事：自由撰稿人 / 跨部门 PM / 研究生 / 编剧 / 首次 onboarding / 移动端云霓采集 / 误删恢复 / 公开分享 / AI 降级 / 冰山再用 |

### Layer 1.5 · 交付与商业 (`docs/`)
| 文档 | 一句话用途 |
| :--- | :--- |
| [MVP-范围与里程碑](MVP-范围与里程碑.md) | M1-M4 阶段门槛 / IN 与 OUT 划分 / 实现依据 |
| [商业化模型](商业化模型.md) | Freemium 四档 / AI 成本测算 / 盈亏平衡 |
| [快速上手](快速上手.md) | 当前 Go 主线 + Phase 13 staging 启动入口 |

### Layer 1.8 · 联调与发布 (`docs/launch/`, `docs/ops/`, `docs/dev/`)
| 文档 | 一句话用途 |
| :--- | :--- |
| [Phase13-准入清单](launch/Phase13-准入清单.md) | 灰度前单页准入门槛 |
| [Phase14-准入清单](launch/Phase14-准入清单.md) | Phase 14 去 CI 化准入门（本地 + staging 脚本） |
| [Phase14.7-三签字流程](launch/Phase14.7-三签字流程.md) | PM / QA / 决策者三签字制 |
| [Phase13-联调与回滚手册](ops/Phase13-联调与回滚手册.md) | staging 拉起 / 回收 / 故障演练 |
| [Phase13-rollback-acceptance-20260427](dev/Phase13-rollback-acceptance-20260427.md) | Phase 13 应用回滚、数据库 down/up 与 CI 验收记录 |
| [Phase13-canonical-path-repair-20260428](dev/Phase13-canonical-path-repair-20260428.md) | 远端 `/home/admin/Ripple` 标准源码路径权限修复记录 |
| [v0.13.0-release-notes](dev/v0.13.0-release-notes.md) | Phase 13 灰度准入版本发布说明 |
| [Phase14-A-技术方案评审-20260427](dev/Phase14-A-技术方案评审-20260427.md) | Org 配额数据模型第一切片评审与验证记录 |
| [Phase14-6-运营化回归与准入设计-20260428](dev/Phase14-6-运营化回归与准入设计-20260428.md) | Phase 14.6 运营化回归、WebSocket smoke 与准入 Gate 设计 |
| [Phase13-故障演练验收记录模板](dev/Phase13-故障演练验收记录模板.md) | 演练结果记录模板 |

### Layer 2 · 系统设计 (`docs/system-design/`)

#### 2.1 总览与诊断
| 文档 | 主题 | 状态 |
| :--- | :--- | :--- |
| [00-设计缺失分析与路线图](system-design/00-设计缺失分析与路线图.md) | 本期诊断：哪些关键设计还缺、按什么优先级补 | ✅ |
| [技术架构总览](system-design/技术架构总览.md) | 总体分层 + 多湖连通哲学 | ✅ |
| [Phase14-规划草案](system-design/Phase14-规划草案.md) | Phase 14 产品化与运营化候选方向、推荐主线与 Sprint 拆分 | 📝 |

#### 2.2 模块级设计
| 文档 | 主题 | 状态 |
| :--- | :--- | :--- |
| [数据模型与权限设计](system-design/数据模型与权限设计.md) | User / Lake / Node / Edge Schema + RBAC | ✅ |
| [云霓-灵感采集模块设计](system-design/云霓-灵感采集模块设计.md) | 移动端 / 语音 / 涂鸦 / 凝露 / 离线同步 | ✅ |
| [冰山-资产沉淀模块设计](system-design/冰山-资产沉淀模块设计.md) | 决堤口 / 灵感三角洲 / 导出 / 资产复用 | ✅ |
| [文件存储与导入流水线](system-design/文件存储与导入流水线.md) | 对象存储 + 文档炸开管道 | ✅ |
| [AI服务编排-织网工作流](system-design/AI服务编排-织网工作流.md) | 多 Agent 红蓝军、对抗性迭代 | ✅ |
| [可观测性与监控](system-design/可观测性与监控.md) | OpenTelemetry / SLO / Token 看板 | ✅ |
| [安全与合规](system-design/安全与合规.md) | 内容审核 / GDPR / 节点级 ACL / 密钥管理 | ✅ |
| [测试策略](system-design/测试策略.md) | 单元 / 集成 / 视觉回归 / 协作并发 | ✅ |
| [设计系统与Tokens](system-design/设计系统与Tokens.md) | 颜色 / 字体 / 间距 / 动效 Token | ✅ |
| [国际化与无障碍](system-design/国际化与无障碍.md) | i18n / a11y / 文化适配 | ✅ |
| [可靠性与容灾](system-design/可靠性与容灾.md) | SLA / 混沌工程 / 备份 / DR | ✅ |

#### 2.3 子系统级设计
| 文档 | 主题 | 状态 |
| :--- | :--- | :--- |
| [造浪池-交互视图设计](system-design/造浪池-交互视图设计.md) | 核心创作画布交互规范 | ✅ |
| [节点与连线状态机规范](system-design/节点与连线状态机规范.md) | 前端状态机 + 动效参数 | ✅ |
| [流体动效引擎-WebGL](system-design/流体动效引擎-WebGL.md) | Shader 实现细节 | ✅ |
| [API网关设计](system-design/API网关设计.md) | 路由 / 认证 / 限流 / 熔断 | ✅ |
| [多人实时协作设计](system-design/多人实时协作设计.md) | WebSocket + LWW + 暗流分岔 | ✅ |
| [跨湖连通设计](system-design/跨湖连通设计.md) | 运河 / 合流 两种模式 | ✅ |
| [AI-Prompt工程规范](system-design/AI-Prompt工程规范.md) | 单次调用 Prompt 模板 | ✅ |

---

## 三、术语速查（水文词典精简）

| 系统术语 | 水文隐喻 | 行业术语 |
| :--- | :--- | :--- |
| Lake | 工作区 | Workspace / Project |
| Node | 灵感节点（浮萍） | Card / Block |
| Edge | 关联连线（暗流） | Link / Edge |
| 投石 | 创建 | Create |
| 引流 | 关联 | Link |
| 分蘖 | 复制衍生 | Duplicate / Branch |
| 蒸发 | 删除（软删） | Soft Delete |
| 固形 | 保存 | Save / Commit |
| 决堤口 | 发布 | Publish |
| 探源 | 检索 | Search |
| 沉淀 | 归档为资产 | Archive |
| 凝露 | 灵感初步定型 | Promote draft |
| 舟子 / 副舟子 / 渡客 / 观潮 | OWNER / NAVIGATOR / PASSENGER / OBSERVER | RBAC roles |
| 雾气节点 (MIST) | 待凝露的草稿 | Draft / Inbox item |
| 迷雾区 (VAPOR) | 蒸发回收站（30 天 TTL） | Trash / Mist Zone |
| 漂流瓶 | 公开访客留言 | Visitor message |
| 朦胧版 | 拓扑可见、内容打码的分享模式 | Skeleton-only share |
| 蓄能潭 | 个人长期资产仓 | Personal asset reservoir |
| 游魂节点 (GHOST) | 跨湖引用源被删后的占位（30 天快照） | Tombstone reference |
| 灵感三角洲 | 冰山资产的拓扑视图 | Asset topology view |

---

## 四、版本与维护

- 本地图随设计文档同步更新；新增设计文档**必须**在此登记。
- 文档命名规范：采用“中文语义化文件名”，避免 D/G/M 前缀；阶段文档可在后缀标注阶段（例如 `设计白皮书-M3.md`）。
- 所有设计文档须在文末标注：版本号 / 日期 / 适用对象 / 文档状态。
- **v2.3（2026-04-27）**：补充 Phase 13 回滚验收记录与 Phase 14 规划草案入口。
- **v2.2（2026-04-27）**：新增 Phase 13 联调 / 准入 / 运维文档入口，阅读路径切换到 Go 主线现状。

