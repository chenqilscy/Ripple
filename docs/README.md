# 「青萍 (Ripple)」文档地图

> 风起于青萍之末，浪成于微澜之间。
> 本文档是阅读全部设计资料的入口，按 **「品牌 → 用户 → 系统」** 三层组织。

---

## 一、阅读路径推荐

| 你的角色 | 推荐顺序 |
| :--- | :--- |
| **新人 / 第一次了解** | 01-品牌与产品概念 → 用户故事 → 文档地图（本文） |
| **产品 / 设计** | 01-品牌与产品概念 → D1 造浪池 → D2 状态机 → 用户故事 |
| **后端开发** | D0 总览 → G1 数据模型 → D4 API 网关 → D5 实时协作 → G4 文件存储 |
| **前端 / 图形** | D1 Delta View → D2 状态机 → D3 流体动效 (WebGL) |
| **AI 算法** | D7 Prompt 规范 → G5 AI 服务编排 → 02-对外汇报话术（"青萍与马具"反思段） |
| **架构评审 / 老板** | 00 缺失分析 → 01 品牌与产品概念 → 用户故事 |

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

### Layer 2 · 系统设计 (`docs/system-design/`)

#### 2.1 总览与诊断
| 文档 | 主题 | 状态 |
| :--- | :--- | :--- |
| [00-设计缺失分析与路线图](system-design/00-设计缺失分析与路线图.md) | 本期诊断：哪些关键设计还缺、按什么优先级补 | ✅ |
| [D0-技术架构总览](system-design/D0-技术架构总览.md) | 总体分层 + 多湖连通哲学 | ✅ |

#### 2.2 G 系列 · 模块级设计
| 文档 | 主题 | 状态 |
| :--- | :--- | :--- |
| [G1-数据模型与权限设计](system-design/G1-数据模型与权限设计.md) | User / Lake / Node / Edge Schema + RBAC | ✅ |
| [G2-云霓-灵感采集模块设计](system-design/G2-云霓-灵感采集模块设计.md) | 移动端 / 语音 / 涂鸦 / 凝露 / 离线同步 | ✅ |
| [G3-冰山-资产沉淀模块设计](system-design/G3-冰山-资产沉淀模块设计.md) | 决堤口 / 灵感三角洲 / 导出 / 资产复用 | ✅ |
| [G4-文件存储与导入流水线](system-design/G4-文件存储与导入流水线.md) | 对象存储 + 文档炸开管道 | ✅ |
| [G5-AI服务编排-织网工作流](system-design/G5-AI服务编排-织网工作流.md) | 多 Agent 红蓝军、对抗性迭代 | ✅ |
| [G6-可观测性与监控](system-design/G6-可观测性与监控.md) | OpenTelemetry / SLO / Token 看板 | ✅ |
| [G7-安全与合规](system-design/G7-安全与合规.md) | 内容审核 / GDPR / 节点级 ACL / 密钥管理 | ✅ |
| [G8-测试策略](system-design/G8-测试策略.md) | 单元 / 集成 / 视觉回归 / 协作并发 | ✅ |
| [G9-设计系统与Tokens](system-design/G9-设计系统与Tokens.md) | 颜色 / 字体 / 间距 / 动效 Token | ✅ |
| [G10-国际化与无障碍](system-design/G10-国际化与无障碍.md) | i18n / a11y / 文化适配 | ✅ |
| [G11-可靠性与容灾](system-design/G11-可靠性与容灾.md) | SLA / 混沌工程 / 备份 / DR | ✅ |

#### 2.3 D 系列 · 子系统级设计
| 文档 | 主题 | 状态 |
| :--- | :--- | :--- |
| [D1-造浪池-Delta-View](system-design/D1-造浪池-Delta-View.md) | 核心创作画布交互规范 | ✅ |
| [D2-节点与连线状态机规范](system-design/D2-节点与连线状态机规范.md) | 前端状态机 + 动效参数 | ✅ |
| [D3-流体动效引擎-WebGL](system-design/D3-流体动效引擎-WebGL.md) | Shader 实现细节 | ✅ |
| [D4-API网关设计](system-design/D4-API网关设计.md) | 路由 / 认证 / 限流 / 熔断 | ✅ |
| [D5-多人实时协作](system-design/D5-多人实时协作.md) | WebSocket + LWW + 暗流分岔 | ✅ |
| [D6-跨湖连通设计](system-design/D6-跨湖连通设计.md) | 运河 / 合流 两种模式 | ✅ |
| [D7-AI-Prompt工程规范](system-design/D7-AI-Prompt工程规范.md) | 单次调用 Prompt 模板 | ✅ |

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
- 文档命名规范：`<前缀>-<主题>.md`，前缀枚举：`G`（模块级）/ `D`（子系统级）/ 数字（顶层）。
- 所有设计文档须在文末标注：版本号 / 日期 / 适用对象 / 文档状态。
- **v2.0（2026-04-21）**：完成原始草稿消化整合，统一为 D0-D7 + G1-G10 命名体系，删除全部 8 份草稿文档。
