# 青萍 (Ripple) · 水文生态创意系统

> 一个把"灵感捕捉、织网、沉淀、分享"设计成**水的循环**的 AI 创作系统。

**状态：** 设计阶段完成（v2.1 · 26 份文档），进入 M1 编码。

---

## 快速导航

| 你是谁 | 从哪儿开始 |
| :--- | :--- |
| **老板 / 投资人** | [品牌与产品概念](docs/pinpai/01-品牌与产品概念.md) · [对外汇报话术](docs/pinpai/02-对外汇报话术.md) · [商业化模型](docs/商业化模型.md) |
| **产品经理** | [用户故事](docs/user-story/story.md) · [MVP 范围](docs/MVP-范围与里程碑.md) · [文档地图](docs/README.md) |
| **架构师** | [D0 技术架构总览](docs/system-design/D0-技术架构总览.md) · [G1 数据模型](docs/system-design/G1-数据模型与权限设计.md) |
| **后端工程师** | [开发者 Onboarding](docs/dev/开发者-Onboarding指南.md) ← **从这里开始** |
| **前端工程师** | [D1 造浪池 Delta View](docs/system-design/D1-造浪池-Delta-View.md) · [D3 流体动效引擎](docs/system-design/D3-流体动效引擎-WebGL.md) · [G9 设计 Tokens](docs/system-design/G9-设计系统与Tokens.md) |
| **AI 工程师** | [G5 AI 服务编排](docs/system-design/G5-AI服务编排-织网工作流.md) · [D7 Prompt 规范](docs/system-design/D7-AI-Prompt工程规范.md) |
| **SRE / 运维** | [G6 可观测性](docs/system-design/G6-可观测性与监控.md) · [G11 可靠性与容灾](docs/system-design/G11-可靠性与容灾.md) |

---

## 文档全景

```
docs/
├── README.md                            文档地图 + 术语表
├── MVP-范围与里程碑.md                   M1/M2/M3/M4 路线
├── 商业化模型.md                         Freemium 四档 + 成本毛利
├── pinpai/                              品牌层（2 份）
├── system-design/                       技术层（D0-D7 + G1-G11 + 00 路线图）
├── user-story/story.md                  10 个用户故事（最高判据）
└── dev/开发者-Onboarding指南.md          新人入场
```

## 本地跑起来

```powershell
docker compose up -d
curl http://localhost:8000/health
```

详见 [开发者 Onboarding 指南](docs/dev/开发者-Onboarding指南.md)。

---

## 团队协作

本项目采用**十角色 AI 团队流程**，详见 [`AGENTS.md`](AGENTS.md)。所有功能需求从需求创造师开始，经 PM 评估、接口设计、正反方辩论、决策者裁决、实现、五轮代码审查、QA、体验方试用、PM 验收的完整闭环。

提交规范：`<type>: <中文描述>`，类型见 AGENTS.md。

---

## License

TBD（商业化模型确定后再定）。
