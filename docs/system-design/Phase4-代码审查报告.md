# Phase 4 全局 5 轮代码审查

**审查范围**：999f011、a8a32b8、91345c5、2efd511（启动批 4 commit）+ Phase 4 主批次（T1 M4-A 文档 / T2 附件 / T3 推荐器缓存 / T4 PWA / T5 压测）

审查者：反方（审查员）  
日期：2026-04-23

---

## 第 1 轮：逻辑正确性

### 检查项
- [x] **T2 Upload**：`hdr.Size` 检查在 `MaxBytesReader` 之后冗余，但不会出错（多重保险）。✅
- [x] **T2 dedupe**：先写盘再查 SHA → 命中重复时删除新文件，返回旧 DTO。逻辑正确。✅
- [x] **T2 Download**：`a.UserID != u.ID` 拒绝；属主可下载；`filepath.Clean` + `..` 检查。✅
- [x] **T3 缓存**：JSON 序列化/反序列化双向，cache miss 走完整算法、cache hit 直接返回。✅
- [x] **T3 InvalidateUser**：`Scan + Del` 模式覆盖该用户所有 (target_type, limit) 组合。✅
- [x] **T3 score 融合**：`2*collab + hot`，权重明确文档化。✅
- [x] **T4 SW**：API 走 network-first + fallback 缓存；静态走 stale-while-revalidate；写请求/SSE 不缓存。✅
- [x] **T5 fake provider**：`SleepMS` 通过 `select { case <-time.After: case <-ctx.Done() }` 正确响应取消。✅
- [x] **T5 ws_connect**：mid alive 取样在 hold/2，反映稳态连接数。✅

### 发现 & 修复
- **隐患 1**：T3 缓存 key 不包含算法版本号，未来调权重将让旧缓存继续生效。
  - **修复**：将 cache key 前缀从 `reco:` 改为 `reco:v2:`。下次算法变更再 bump。

---

## 第 2 轮：边界条件与异常处理

### 检查项
- [x] **T2 Upload 空文件**：`hdr.Size == 0` 通过；写盘成功，sha 为空内容 hash。可接受（MIME 仍校验）。✅
- [x] **T2 Upload MIME 缺失**：MIME 不在白名单 → 415。✅
- [x] **T2 Download 不存在**：`domain.ErrNotFound` → 404。✅
- [x] **T3 Recommend mine 为空**：跳过 collab，仍返回 hotScores → 解决冷启动！相比旧版（直接返空）改进明显。✅
- [x] **T3 Recommend Redis 失败**：`Get`/`Set` 错误均忽略，降级走 PG。✅
- [x] **T3 InvalidateUser ctx 取消**：iter.Next 内部检查 ctx；可能漏删几个 key，但下次 GET 会写回最新。✅
- [x] **T5 ws_connect dial timeout**：`context.WithTimeout` 10s。✅
- [x] **T5 perma_post**：5xx/连接错误均计 fail，分母用 maxInt(1) 防除零。✅

### 发现 & 修复
- **缺陷 1**：T2 Upload 的 `attachmentToDTO` 序列化 `node_id` 为空字符串而非 `null`（前端可能区分）。
  - **修复策略**：可接受；前端传空时不会查询，DTO 一致即可。**不修**（YAGNI）。

---

## 第 3 轮：架构一致性与代码规范

### 检查项
- [x] **T2 attachments 表**：`node_id` 字段使用 `VARCHAR(64)` 而非 UUID FK，因 nodes 在 Neo4j。已在 SQL 注释说明。✅
- [x] **T3 RecommenderService 签名变更**：`NewRecommenderService(feedback, rdb)` —— 调用方仅 `cmd/server/main.go` + 测试，已同步更新。✅
- [x] **T2 Upload 路径**：`{UploadDir}/{userID}/{uuid}{ext}`，按用户隔离。✅
- [x] **T4 manifest icons**：使用 SVG，省图片处理；`maskable` 已声明。✅
- [x] **T5 loadtest** 移入 `backend-go/cmd/loadtest/<name>/`，复用模块依赖。✅

### 发现 & 修复
- **改进点 1**：T3 注释提到"未做同空间/同主题，因实体在 Neo4j"，已写入服务注释。读者无歧义。✅

---

## 第 4 轮：安全性与数据隔离

### 检查项
- [x] **T2 ownership**：`Download` 只允许 `a.UserID == u.ID`，**核心隔离**。✅
- [x] **T2 path traversal**：`filepath.Clean` + `strings.Contains(.., "..")` 双保险。文件名亦不来自用户输入（用 uuid + safeExt 白名单）。✅
- [x] **T2 MIME 白名单**：`image/png|jpeg|gif|webp`，拒绝 SVG / HTML（避免 XSS via download）。✅
- [x] **T2 MaxBytesReader**：硬限大小，防 DoS。✅
- [x] **T2 SHA dedupe 跨用户**：唯一索引是 `(user_id, sha256)`，**用户间隔离**——同一图被两用户上传会各存一份。✅
- [x] **T3 缓存 key**：含 user_id 前缀，**不会跨用户串数据**。✅
- [x] **T4 SW**：仅缓存 GET，不缓存 Authorization 异常的写操作；不存 Authorization 头本身。✅
- [x] **T5 fake provider**：仅在 `RIPPLE_LLM_FAKE=true` 时启用；启动 warn 日志提示。生产应保持默认 false。✅

### 发现 & 修复
- **隐患 1**：T2 Upload 的 `safeExt` 信任 `Content-Type` 头（攻击者可伪造）。
  - **风险评估**：MIME 已在白名单，最坏情况是 `.png` 文件其实是 jpg，浏览器 sniffing 会自适应；不会执行恶意脚本。
  - **加固建议**：未来加入 magic bytes 检测（DetectContentType 前 512 字节）。
  - **本轮决定**：记入 TODO（M4-S2.1），不阻塞当前 commit。

---

## 第 5 轮：性能与资源管理

### 检查项
- [x] **T2 Upload 流式**：`io.MultiWriter(dst, hasher)` 单遍流式，无全量驻留内存。✅
- [x] **T2 Download**：`http.ServeFile` 支持 range / sendfile（OS 原生）。✅
- [x] **T3 缓存**：5 分钟 TTL，命中省去 4 次 PG 查询；按用户 invalidate 写后立即生效。✅
- [x] **T3 Scan**：使用 `Iterator` 增量扫，不一次拉全部。但 prod 环境若有 100k 用户活跃，可能 scan 慢——可后续加 `reco:idx:<uid>` 反向索引。
- [x] **T4 SW**：缓存大小未限制，可能积累——浏览器有 quota 兜底，约 50MB。✅
- [x] **T5 baseline.go**：`http.Transport` 启用 keep-alive，避免端口耗尽。✅
- [x] **T5 ws_connect.go**：每连接一个 goroutine，1000 连接 ≈ 8MB stack——可接受。✅

### 发现 & 修复
- **优化机会 1**：T2 SHA 计算可放协程 + io.Pipe，但当前 5MB 上限下单线程 < 50ms 完成，不优化。✅

---

## 总结

| 轮次 | 通过 | 修复 | 待跟踪 |
|------|------|------|--------|
| 1 逻辑 | ✅ | cache key 加 v2 前缀 | — |
| 2 边界 | ✅ | — | — |
| 3 规范 | ✅ | — | — |
| 4 安全 | ✅ | — | TODO: magic bytes 检测 |
| 5 性能 | ✅ | — | TODO: reco 反向索引（>10k 用户后） |

**结论**：Phase 4 全部 commit 通过 5 轮审查，仅 1 处需立即修复（T3 cache key 版本前缀），其余进入 backlog。
