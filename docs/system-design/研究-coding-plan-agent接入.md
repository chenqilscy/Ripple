# Coding-Plan Agent 接入研究

> 出处：老板原话「研究如何将 coding plan 这类 agent（它们一般不按 token 计算）作为本系统的 Agent 执行，这样能节省成本」
> 状态：研究阶段（无生产代码）
> 决策：见末尾"决策建议"

---

## 一、问题定义

本系统当前 LLM 调用走 token 计费（智谱/OpenAI/DeepSeek 等），单次"造云"产 5 候选 ≈ 800-1500 tokens。
高频创意场景下成本随用户活跃度线性增长。

而**订阅制 coding agent**（Cursor / Claude Code / Copilot CLI / Aider 等）按月固定费用，
理论上单订阅可承载 **远超** 等额 token 配额的调用量。

**问题**：能否把这类 agent 当作 Provider 接入本系统的 Router？

---

## 二、候选 Agent 调研

| Agent | 计费模式 | 可编程接口 | Headless 模式 | 接入难度 |
|------|---------|-----------|---------------|---------|
| **Claude Code** (CLI) | $20/月 Pro / API 按 token | `claude -p "<prompt>"` 一次性输出 | ✅ 原生支持 | 低 |
| **Cursor CLI** | $20/月 Pro | `cursor-agent` (beta) | ⚠️ 需 IDE 上下文 | 中 |
| **GitHub Copilot CLI** | $10/月 Individual | `gh copilot suggest` | ✅ | 低（但限编程场景） |
| **Aider** | 自托管 + 自带 LLM key | `aider --message "..." --yes` | ✅ | 低（但仍调你自己 key） |
| **Continue.dev** | 自托管 + 自带 LLM key | TS SDK | ✅ | 中（仍按 token） |
| **Codex CLI** (OpenAI) | API 按 token | CLI 工具 | ✅ | 低（仍按 token） |

**关键发现**：
1. **Aider / Continue.dev 不省钱** — 它们是 LLM 客户端，本身不带额度，调用还是走你的 OpenAI/Claude key。
2. **真正订阅省钱的只有 Cursor / Claude Code / Copilot CLI** — 它们有"包月不限量"的特性（虽然有公平使用限制）。
3. **Copilot CLI 限编程场景**：`gh copilot suggest` 主要服务命令行/git/shell 场景，对"创意发散"这类自然语言任务不一定能产出高质量结果。
4. **Cursor CLI 仍 beta**：`cursor-agent` 命令依赖 IDE 工作区上下文，headless 调用稳定性不足。
5. **Claude Code (`claude` CLI) 是最佳候选**：
   - 已 GA、文档完整
   - `claude -p "prompt" --output-format json` 一次性输出，无需交互
   - Pro/Max 套餐含大量 Sonnet 4.5 调用配额
   - 输出质量与 Anthropic API 同源

---

## 三、可行性分析

### 方案 A：Claude Code CLI as Provider

**架构**：
```
Router → ClaudeCodeProvider (本地 fork+exec)
       → exec.CommandContext("claude", "-p", prompt, "--output-format=json")
       → 解析 stdout JSON → []Candidate
```

**优势**：
- 边际成本 ≈ 0（订阅已付）
- 模型质量高（Claude Sonnet 4.5）
- 失败时 Router 自动 fallback 到 token 计费 provider，零业务影响

**风险与限制**：
| 风险 | 说明 | 缓解 |
|------|------|------|
| **依赖本地二进制** | 服务器需预装 `claude` CLI 并完成 OAuth 登录 | Docker 镜像层固化 / 只在自托管部署可用 |
| **公平使用限制** | Anthropic 对 Pro 套餐有 5h 滚动窗口的 message 上限 | 监控 429，触发后退避到 token provider |
| **进程冷启动** | 每次 fork+exec ~200-500ms 开销 | 可接受（造云本身异步） |
| **并发受限** | CLI 设计为单用户使用，并发可能触发账号风控 | 串行队列 + 限流（如 1 QPS） |
| **不能商用** | Anthropic ToS 禁止订阅账号代多用户提供服务 | **仅限本地/私有部署/单人场景** |

### 方案 B：Aider / Continue.dev 自托管转译层

**思路**：用 Aider 当中介把 prompt 翻译给本地 Ollama (qwen2.5/llama3.3)。
**结论**：等于自建本地推理。如果有 GPU 可行，否则还不如继续用云 API。

### 方案 C：混合策略（推荐）

将订阅制 agent 作为 **优先 provider**，token-based 作为 **保底**：

```go
providers := []Provider{
    NewClaudeCodeProvider(...),       // 优先：订阅免费额度
    NewOpenAICompatClient(deepseek),  // 兜底：按量便宜
    NewZhipuClient(...),              // 二次兜底
}
router := NewDefaultRouter(providers, Policy{EnableFallback: true}, recorder)
```

复用现有 R2 已交付的 Router fallback 机制，**零新增运行时复杂度**。

---

## 四、接口草案

```go
// internal/llm/claude_code_provider.go (草案，非生产代码)

type ClaudeCodeProvider struct {
    binPath  string        // "claude" 或绝对路径
    timeout  time.Duration // 默认 60s（CLI 比 HTTP 慢）
    sem      chan struct{} // 限流信号量（如 cap=1 串行）
}

func (p *ClaudeCodeProvider) Name() string         { return "claude-code" }
func (p *ClaudeCodeProvider) Supports(m Modality) bool { return m == ModalityText }

func (p *ClaudeCodeProvider) Generate(ctx context.Context, req GenerateRequest) ([]Candidate, error) {
    // 1. 限流
    select {
    case p.sem <- struct{}{}:
        defer func() { <-p.sem }()
    case <-ctx.Done():
        return nil, ctx.Err()
    }

    // 2. 拼装 prompt（复用 zhipu/openai_compat 的 system prompt 模板）
    sys := buildDivergencePrompt(req.N)
    fullPrompt := sys + "\n\n" + req.Prompt

    // 3. fork+exec
    cctx, cancel := context.WithTimeout(ctx, p.timeout)
    defer cancel()
    cmd := exec.CommandContext(cctx, p.binPath, "-p", fullPrompt, "--output-format=json")
    out, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("claude-code: %w", err)
    }

    // 4. 解析 JSON（{result: "...", usage: {...}}）
    var resp struct {
        Result string `json:"result"`
    }
    if err := json.Unmarshal(out, &resp); err != nil {
        return nil, err
    }

    // 5. 复用 parseLines（zhipu.go 已有）
    lines := parseLines(resp.Result, req.N)
    cands := make([]Candidate, 0, len(lines))
    for _, l := range lines {
        cands = append(cands, Candidate{
            Modality: ModalityText, Text: l, MIME: "text/plain",
            CostTokens: 0, // 订阅制不计 token
        })
    }
    return cands, nil
}
```

**实现复杂度**：< 100 行，复用 R2 抽出的 `parseLines`。

---

## 五、风险红线

| 红线 | 来源 | 处理 |
|------|------|------|
| **不得用订阅账号提供商业服务** | Anthropic ToS § Acceptable Use | 仅文档标注"仅限自托管/个人使用"，不在云端版本默认开启 |
| **不得绕过限流** | Anthropic 风控 | 内置 1 QPS 信号量 + 429 退避 |
| **CLI 二进制信任** | 供应链安全 | 锁定版本 + checksum 校验（部署文档补） |
| **prompt 注入风险** | 用户输入拼到 CLI 参数 | 用 stdin 传输不走 argv，避免 shell 解析（cmd.Stdin = strings.NewReader(prompt)） |

---

## 六、决策建议

### 推荐路径：**P3 实施**（M2 之后再做）

**理由**：
1. M1 阶段优先把核心创意闭环跑稳（节点 / 造云 / 凝结 / 蒸发 / 实时）
2. R2 已让 Router 支持多 provider + fallback，**接入 Claude Code 是纯加法**，不阻塞主线
3. 商用合规问题需先确定部署形态（自托管 vs SaaS）

### 短期 quick win（可做）：
- 在 `docs/system-design/` 留下本研究文档（已完成 ✅）
- 在 `internal/llm/` 留一个 `claude_code_provider.go.draft`（草案，go build tag 排除）— 但本轮**不做**，避免引入未验证代码

### 长期（M3+）：
- 评估接入 LiteLLM / OpenRouter 等聚合层，用统一接口管理订阅 + 按量混合策略
- 追踪 Cursor CLI / Copilot CLI 的 headless 能力成熟度

---

## 七、后续 TODO（不本次实施）

- [ ] 老板决定部署形态后，再评估是否接入 Claude Code Provider
- [ ] 若上线，需补：限流监控 / 429 退避 / CLI 健康检查（启动期 `claude --version`）
- [ ] 自学习日志记录决策过程（已在本文档体现，无需重复）
