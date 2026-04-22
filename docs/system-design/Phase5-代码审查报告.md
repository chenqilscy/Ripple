# Phase 5 五轮代码审查报告

> 审查范围：P5 批次 7 个提交（a97bc61 → cb1bd72）
> 审查者（角色）：反方（审查员）
> 审查日期：2026-04-23
> 审查规范：AGENTS.md §"五轮代码审查"

---

## 范围

| 提交 | 内容 |
|------|------|
| a97bc61 | P5-T1：前端 AttachmentBar UI（拖拽 + Authorization Blob URL） |
| eda93e2 | P5-T3：上传 magic bytes 校验（http.DetectContentType + sniff buffer） |
| 2bd8258 | P5-T4：推荐器同空间信号（ListIDsByLakes + WithSpaceSignal） |
| 24e7303 | P5-T2：M4-A Yjs Spike（独立二进制 :7790） |
| b196548 | P5-T5：多模态 placeholder image provider（SVG data URI） |
| (T6) | Playwright e2e 脚手架（playwright.config.ts + smoke.spec.ts） |
| acb72a8 / cb1bd72 | P5-T7 全链路实测报告 + .gitignore |

---

## 第 1 轮：逻辑正确性

| 模块 | 结论 | 证据 |
|------|------|------|
| AttachmentBar `useBlobURL` | ✅ | 卸载时 `URL.revokeObjectURL` 已调用；fetch 失败回落空字符串 |
| Upload sniff 缓冲 | ✅ | 先把 `sniffBuf[:nSniff]` 写入 dst 与 hasher，再 `io.Copy` 余下流，未漏字节 |
| Recommender 同空间合并 | ✅ | `sameSpace` 在 collab/hot 之外加权 ×3；排除自己提交节点（`exclude` 集合） |
| Yjs Hub 广播 | ✅ | 房间锁 (`mu sync.Mutex`) 包裹 `rooms[lake]` 操作；写入超时 3 s 防慢消费者 |
| Placeholder image | ✅ | SVG 模板转义 prompt（`html.EscapeString`），Base64 编码 data URI |
| perma_post 字段 | ✅ | `source_node_ids` / `title_hint` 与服务端 contract 对齐 |

**结论**：本轮无逻辑缺陷。

---

## 第 2 轮：边界条件与异常处理

| 风险点 | 处置 |
|--------|------|
| AttachmentBar 5 MB 上限 | ✅ `if (file.size > 5 * 1024 * 1024)` 客户端拦截，并在服务端 multipart MaxBytesReader 二次保护 |
| MIME 白名单 | ✅ 客户端：png/jpeg/gif/webp；服务端：sniffed 与 declared 双重校验 |
| Recommender 空 lakeIDs | ✅ `len(lakeIDs)==0` 早 return；ListIDsByLakes 内部 `ANY($1)` 兼容空数组 |
| Yjs 空房间清理 | ⚠️ **观察项**：peer 断开时 `delete(rooms[lake], p)`，但当 `len==0` 未删除空 map 条目，长期运行可能积累空键 → 列入 P6 改进 |
| Image provider candidates>N | ✅ 循环按 `count` 上限生成；count<=0 处理为 1 |

**结论**：1 个观察项（Yjs 空 map），非阻塞。

---

## 第 3 轮：架构一致性与代码规范

| 检查项 | 结论 |
|--------|------|
| AttachmentBar 放在 `frontend/src/components/` | ✅ 与现有目录约定一致 |
| Yjs Bridge 独立 `cmd/yjs-bridge/`，不污染主 server | ✅ 符合"独立可弃 spike"原则 |
| Recommender 通过 `WithSpaceSignal(perma, member)` 注入，不破坏构造函数签名 | ✅ 选项模式一致 |
| ImageProvider 实现 `llm.Provider` 接口（`Supports(IMAGE)` only） | ✅ 与 fake provider 一致风格 |
| 配置项 `RIPPLE_LLM_IMAGE_STUB` envconfig prefix 正确 | ✅ |
| 中文 commit message + `<type>:` 前缀 | ✅ 全部符合 AGENTS.md §Git |

**结论**：无违规。

---

## 第 4 轮：安全性与数据隔离

| 风险 | 处置 |
|------|------|
| 上传 MIME 伪造（恶意可执行伪装 png） | ✅ http.DetectContentType 嗅探前 512 字节，与 declared 不匹配立即拒绝 |
| Blob URL 跨用户泄漏 | ✅ AttachmentBar 走 Authorization 头拉取，不暴露公开 URL；blob 在组件卸载时回收 |
| Yjs Bridge 鉴权 | ⚠️ **遗留**：当前 `:7790` 无 JWT 校验，仅信任 lakeID 路径；适合 spike，但**生产前必须挂 JWT/Origin 检查**——已在报告 §6 列入 P6 |
| Recommender 跨用户拉别人 perma 节点 ID | ✅ 通过 `memberRepo.ListLakesByUser(userID)` 限定可见湖；不会越权 |
| Image provider 远程加载 | ✅ 当前为 SVG 内联，未发起任何外部 HTTP，无 SSRF 面 |

**结论**：1 项 spike 阶段已知遗留（Yjs JWT），其余通过。

---

## 第 5 轮：性能与资源管理

| 风险 | 处置 |
|------|------|
| Upload sniff 多次写入 | ✅ 单次 `dst.Write(sniffBuf)` + 单次 `io.Copy`，无重复落盘 |
| Recommender ListIDsByLakes 拉太多行 | ✅ 受 `limit` 参数（caller 默认 200）+ `ORDER BY created_at DESC` 索引扫描 |
| Yjs broadcast goroutine 泄漏 | ✅ 每条消息 `ctx, cancel := context.WithTimeout(3s)` 且 defer cancel；写失败的 peer 主动从房间剔除 |
| Image SVG 过大 | ✅ 单张 < 1 KB；Base64 后约 1.4 KB，对 LLM provider 输出无压力 |
| Blob URL 内存泄漏 | ✅ useEffect cleanup 调 revokeObjectURL |

**结论**：本批次所有提交在性能/资源维度均无显著问题。

---

## 总评

| 维度 | 得分 |
|------|------|
| 逻辑正确性 | 5/5 |
| 边界异常 | 4/5（Yjs 空 map） |
| 架构规范 | 5/5 |
| 安全 | 4/5（Yjs JWT 遗留 spike） |
| 性能资源 | 5/5 |

**结论**：P5 批次**通过五轮审查**，2 个非阻塞观察项已转入 P6 backlog。

## P6 待办（从本轮审查导出）

1. Yjs Bridge 增 JWT 校验 + Origin 白名单
2. Yjs Hub 房间空时清理 map 条目
3. perma_post 500 根因排查（参见全链路实测报告 §4.2）
4. WS 1 000 conc 30 s 持续测 + 资源监控
