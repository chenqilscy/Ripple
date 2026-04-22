# LLM Provider 接入手册

本文档说明 Ripple 后端目前支持的 LLM Provider、配置方式、路由策略与扩展方法。

> **来源约束**：`docs/system-design/系统约束规约.md` §7、`AGENTS.md` 技术决策原则。

---

## 1. 已实现 Provider 一览

| Provider | 协议 | 调用方式 | 模态 | 计费单位 | 集成度 |
|----------|------|----------|------|----------|--------|
| **zhipu** | 智谱原生 (`/api/paas/v4`) | HTTP + JWT | TEXT | token | 已 e2e 验证 |
| **openai** | OpenAI Chat Completion | HTTP + Bearer | TEXT | token | 已实现 |
| **deepseek** | OpenAI 兼容 | HTTP + Bearer | TEXT | token | 已实现 |
| **volc / doubao** | 火山方舟（OpenAI 兼容） | HTTP + Bearer | TEXT | token | 已实现 |
| **minimax** | MiniMax v2（OpenAI 兼容） | HTTP + Bearer | TEXT | token | 已实现 |
| **openai-compat** | 任意 OpenAI 兼容（Ollama / vLLM / SGLang） | HTTP + Bearer | TEXT | token | 已实现 |
| **claude-code** | CLI 子进程 (`claude -p`) | exec stdin/stdout | TEXT | **不计 token**（订阅） | 草案 + 启动侦测 |

实现位置：

- 通用客户端：[internal/llm/openai_compat.go](backend-go/internal/llm/openai_compat.go)
- 智谱专用：[internal/llm/zhipu.go](backend-go/internal/llm/zhipu.go)
- Claude Code：[internal/llm/claude_code.go](backend-go/internal/llm/claude_code.go)（主线，env `RIPPLE_CLAUDE_CODE_ENABLED=true` 启用）
- Claude Code 启动侦测：[internal/llm/claude_code_detect.go](backend-go/internal/llm/claude_code_detect.go)
- 路由 + 注册：[internal/llm/router.go](backend-go/internal/llm/router.go) / [internal/llm/registry.go](backend-go/internal/llm/registry.go)

---

## 2. 环境变量

所有 Provider 均通过 envconfig 加载，前缀 `RIPPLE_`。

```bash
# === 智谱 ===
RIPPLE_ZHIPU_API_KEY=xxx
RIPPLE_ZHIPU_MODEL=glm-4-flash

# === OpenAI 官方 ===
RIPPLE_OPENAI_API_KEY=sk-xxx
RIPPLE_OPENAI_MODEL=gpt-4o-mini
RIPPLE_OPENAI_ENDPOINT=             # 留空用默认；可指 Azure/proxy

# === DeepSeek ===
RIPPLE_DEEPSEEK_API_KEY=sk-xxx
RIPPLE_DEEPSEEK_MODEL=deepseek-chat

# === 火山豆包 ===
RIPPLE_VOLC_API_KEY=xxx
RIPPLE_VOLC_MODEL=doubao-1-5-lite-32k-250115

# === MiniMax ===
RIPPLE_MINIMAX_API_KEY=xxx
RIPPLE_MINIMAX_MODEL=abab6.5s-chat

# === 通用 OpenAI 兼容（如本地 Ollama）===
RIPPLE_OPENAI_COMPAT_KEY=ollama
RIPPLE_OPENAI_COMPAT_MODEL=qwen2.5:14b
RIPPLE_OPENAI_COMPAT_ENDPOINT=http://localhost:11434/v1/chat/completions
RIPPLE_OPENAI_COMPAT_NAME=ollama-local

# === Claude Code（订阅制；当前仅启动侦测）===
RIPPLE_CLAUDE_CODE_CLI_PATH=         # 留空走 PATH 查找 "claude"

# === 路由策略 ===
RIPPLE_LLM_PROVIDER_ORDER=zhipu,deepseek,openai
RIPPLE_LLM_FALLBACK=true
```

> **未配置 API Key 的 provider 会被静默跳过**，不会启动失败。

---

## 3. 路由策略

`router.go` 中的 `DefaultRouter`：

1. 按 `LLM_PROVIDER_ORDER` 顺序遍历 providers
2. 第一个 `Supports(req.Modality)=true` 的被选中
3. 若 `EnableFallback=true` 且首个失败，依次尝试后续 providers
4. 每次成功调用通过 `CallRecorder` 异步落 `llm_calls` 表（不阻塞业务）

**故障域**：`fallback` 仅对网络/HTTP 5xx/超时生效；4xx 视为 fatal 直接返回。

---

## 4. Claude Code Provider 落地说明

### 4.1 现状（主线已接入，2025-Q4）

Claude Code Provider 已主线编译（移除 `claude_code` build tag），通过环境变量 `RIPPLE_CLAUDE_CODE_ENABLED=true` 启用，调用本机已安装的 `claude` CLI 完成生成。配额异常（`quota` / `rate limit` / `usage limit` / `exhausted`）会被 stderr 关键字识别并以独立错误返回，便于上层熔断。

实现：[claude_code.go](backend-go/internal/llm/claude_code.go) · 注册：[registry.go](backend-go/internal/llm/registry.go) `case "claude-code"`。

> 合规提示：Claude Code 个人订阅不得用于商业多租户复用；生产启用前请人工核对 Anthropic ToS，并务必配合 §4.5 的速率限制与配额监控。

### 4.2 启动侦测

主线代码 [claude_code_detect.go](backend-go/internal/llm/claude_code_detect.go) 提供 `ProbeClaudeCodeCLI(ctx, path)`，启动时会：

- `exec.LookPath("claude")` 解析绝对路径
- 跑 `claude --version` 验证可用（3 秒超时）
- 写 zerolog `info` 日志 `claude code cli detected`；若显式配了 `CLAUDE_CODE_CLI_PATH` 但不可用则 `warn`

侦测**不阻塞启动**，失败仅日志。

### 4.3 启用步骤

```bash
# 1. 本机安装 Claude Code CLI 并完成登录订阅
claude --version    # 验证可用

# 2. 配置环境变量（.env）
RIPPLE_CLAUDE_CODE_ENABLED=true
RIPPLE_CLAUDE_CODE_CLI_PATH=         # 可选；空走 PATH
RIPPLE_CLAUDE_CODE_MODEL=            # 可选；空走 CLI 默认
# 可选：把 claude-code 放到顺位末尾做 fallback
RIPPLE_LLM_PROVIDER_ORDER=zhipu,deepseek,openai,claude-code

# 3. 启动；启动日志会出现 `claude code cli detected`
```

### 4.4 集成测试

```powershell
# 本机已装 claude CLI
$env:RIPPLE_CLAUDE_CODE_CLI=1
go test ./internal/llm/... -run TestProbeClaudeCodeCLI_Real -v
```

详见 [claude_code_detect_test.go](backend-go/internal/llm/claude_code_detect_test.go)。

### 4.5 安全红线 + 配额监控

| 红线 | 实现位置 |
|------|----------|
| 不把 prompt 拼进 shell 命令行（只走 stdin） | `invokeOnce` 使用 `cmd.Stdin = strings.NewReader(prompt)` |
| context cancel 必须杀子进程 | `exec.CommandContext` 自动处理 |
| 单次调用上限 60s | `cfg.Timeout` 默认 60s |
| N>5 拒绝（订阅配额保护） | `Generate` 入参校验 |
| 配额耗尽自动识别 | `invokeOnce` 扫描 stderr 含 `quota`/`rate limit`/`usage limit`/`exhausted` |
| 全局速率限制 | `RIPPLE_LLM_RPS` + `RIPPLE_LLM_BURST` 套用 `RateLimitedProvider`（见 §6） |
| 不把完整 prompt 写日志 | 仅记 sha256 前 16B（待落地） |

---

## 5. 流式输出（OpenAI 兼容，TD-001 已偿还）

`OpenAICompatClient` 实现了可选接口 `StreamProvider`：

```go
type StreamProvider interface {
    GenerateStream(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)
}
type StreamChunk struct {
    Delta      string
    Done       bool
    CostTokens int64
    Err        error
}
```

- 协议：标准 SSE（`data: {...json...}` / `data: [DONE]`）
- 行为：自动转发 ctx cancel，关闭 channel；usage 通过 `Done=true` 帧的 `CostTokens` 返回
- 适用：仅 `ModalityText`；其他模态 `GenerateStream` 直接报错
- Router 暂未集成（M3 PERMA Crystallize 上线时再接入）；当前业务可直接通过类型断言使用：

```go
if sp, ok := provider.(llm.StreamProvider); ok { /* stream */ }
```

---

## 6. Provider 速率限制（TD-002 已偿还）

所有 Provider 在 `BuildProviders` 阶段被 `RateLimitedProvider` 装饰器透明包裹，参数来自配置：

| 环境变量 | 默认 | 说明 |
|----------|-----:|------|
| `RIPPLE_LLM_RPS` | 5 | 每个 Provider 每秒最大调用次数；`<=0` 关闭限速 |
| `RIPPLE_LLM_BURST` | 10 | 短时令牌桶容量 |

实现细节：

- 内部用 `golang.org/x/time/rate` 标准 token bucket
- `Generate` / `GenerateStream` 调用前先 `limiter.Wait(ctx)`；ctx cancel 立即返回错误
- `Name()` / `Supports()` 透传给 inner Provider，对 Router 透明
- 单元测试：[rate_limit_test.go](backend-go/internal/llm/rate_limit_test.go) 覆盖无限速绕过、burst 不阻塞、耗尽后阻塞、ctx cancel 退出

---

## 5. 扩展新 Provider

### 5.1 OpenAI 兼容协议

直接通过 `OPENAI_COMPAT_*` 环境变量即可，无需改代码。

### 5.2 非 OpenAI 协议

参考 `zhipu.go` 模式：

1. 新建 `internal/llm/<vendor>.go`，实现 `Provider` 接口（`Name() / Supports() / Generate()`）
2. `internal/llm/registry.go` 中加 case 分支
3. `internal/config/config.go` 中加配置字段
4. `cmd/server/main.go` 中传入 `BuildProviders` 调用
5. 提交需经接口设计师评审（`AGENTS.md` §技术决策原则）

---

## 6. 已知问题与未来工作

- [ ] Claude Code provider 当前缺：sha256 prompt 日志、配额监控、`quota exceeded` 错误识别
- [ ] OpenAI 兼容客户端的 `Generate` 不支持 `stream=true`，长输出会等待完整响应
- [ ] 没有针对 provider 的速率限制（依赖 LLM 厂商自身 429）
- [ ] `CallRecorder` 异步通道满时丢弃记录（512 buffer）

详见 [技术债清单.md](技术债清单.md)。
