//go:build claude_code
// +build claude_code

// Package llm · Claude Code CLI Provider 草案（build tag 隔离）。
//
// 本文件仅在 `go build -tags=claude_code` 时参与编译，主线不参与。
// 研究背景：docs/system-design/研究-coding-plan-agent接入.md
//
// ## 设计要点
//
//  1. **不计 token**：Claude Code 订阅制（Pro/Team/Enterprise），按月固定费用，
//     不按 token 计费。因此 CostTokens 字段返回 0（占位），真实成本分摊由上层统计调用次数实现。
//  2. **通过 stdin/stdout**：使用 `claude -p` 模式（print/non-interactive），
//     prompt 从 stdin 传入，避免 shell 命令行注入；响应从 stdout 读取。
//  3. **timeout 必选**：默认 60s（Claude Code 思考周期较长）。context cancel 时杀进程。
//  4. **红线**：
//     - 仅用于内部 / 单租户场景（共享订阅，Anthropic ToS 可能不允许商业多租户复用）
//     - 绝不把用户输入直接 fork+exec shell（用 exec.CommandContext 参数列表）
//     - 日志脱敏：不记录完整 prompt，只记录 sha256 前 16B
//     - 配额监控：若 Claude Code 返回 "quota exceeded"，router 应 fallback 到 token-based provider
//
// ## 使用（未来）
//
//   export CLAUDE_CODE_CLI_PATH=/usr/local/bin/claude
//   go build -tags=claude_code -o ripple-server ./cmd/server
//
// ## 测试
//
// 本文件**不**包含测试。集成验证需真实安装 Claude Code CLI + 有效订阅。
// 首次启用前必须人工核对 Anthropic ToS。
package llm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ClaudeCodeConfig Claude Code Provider 配置。
type ClaudeCodeConfig struct {
	CLIPath string        // claude 可执行文件绝对路径；空则用 PATH 查找
	Timeout time.Duration // 单次调用上限；默认 60s
	Model   string        // 可选：对应 Anthropic model name（claude-sonnet-4 / claude-opus-4 等）
}

// ClaudeCodeProvider 通过 fork+exec Claude Code CLI 调用。
type ClaudeCodeProvider struct {
	cfg ClaudeCodeConfig
}

// NewClaudeCodeProvider 构造。若 cfg.CLIPath 为空，使用 "claude"（依赖 PATH）。
func NewClaudeCodeProvider(cfg ClaudeCodeConfig) *ClaudeCodeProvider {
	if cfg.CLIPath == "" {
		cfg.CLIPath = "claude"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &ClaudeCodeProvider{cfg: cfg}
}

// Name 实现 Provider。
func (p *ClaudeCodeProvider) Name() string { return "claude-code" }

// Supports 仅支持文本。
func (p *ClaudeCodeProvider) Supports(m Modality) bool { return m == ModalityText }

// Generate 实现 Provider。
//
// 实现注意：
//   - 用 exec.CommandContext 保证 ctx cancel 时子进程被杀
//   - stdin 传入 prompt（避免命令行注入）
//   - stdout 作为响应；stderr 仅用于错误信息
//   - N 参数：Claude Code 一次只回一条；这里循环调用 N 次（代价：订阅共享配额消耗更快）
func (p *ClaudeCodeProvider) Generate(ctx context.Context, req GenerateRequest) ([]Candidate, error) {
	if req.Modality != ModalityText {
		return nil, fmt.Errorf("claude-code: unsupported modality %q", req.Modality)
	}
	if req.N <= 0 {
		req.N = 1
	}
	if req.N > 5 {
		return nil, fmt.Errorf("claude-code: N>5 not allowed (subscription quota concern)")
	}

	sysPrompt := "You are Ripple, a creative ideation assistant. " +
		"Output exactly ONE divergent idea in Chinese, one line, no markdown, no explanation."
	fullPrompt := sysPrompt + "\n\n" + req.Prompt

	out := make([]Candidate, 0, req.N)
	for i := 0; i < req.N; i++ {
		// 每次独立 timeout
		subCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
		text, err := p.invokeOnce(subCtx, fullPrompt)
		cancel()
		if err != nil {
			// 若已成功几个，返回已有结果 + 错误给上层决策
			if len(out) > 0 {
				return out, nil
			}
			return nil, err
		}
		out = append(out, Candidate{
			Modality:   ModalityText,
			Text:       strings.TrimSpace(text),
			MIME:       "text/plain",
			CostTokens: 0, // 订阅制，不按 token 计费
		})
	}
	return out, nil
}

// invokeOnce 调用一次 claude CLI。
func (p *ClaudeCodeProvider) invokeOnce(ctx context.Context, prompt string) (string, error) {
	args := []string{"-p"} // print mode
	if p.cfg.Model != "" {
		args = append(args, "--model", p.cfg.Model)
	}

	cmd := exec.CommandContext(ctx, p.cfg.CLIPath, args...)
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// 区分 context 超时
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("claude-code: timeout after %s", p.cfg.Timeout)
		}
		return "", fmt.Errorf("claude-code: exec failed: %w (stderr: %s)",
			err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
