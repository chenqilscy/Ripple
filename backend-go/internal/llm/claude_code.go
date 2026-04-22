// Package llm · Claude Code CLI Provider（订阅制；不计 token）。
//
// 调用本机已安装的 Claude Code CLI（`claude` 可执行）。订阅按月固定费用，
// 因此 CostTokens 字段返回 0；真实成本通过调用次数 + 上层配额限制控制。
//
// ## 设计要点
//
//  1. **不计 token**：CostTokens=0
//  2. **stdin/stdout**：使用 `claude -p` 模式，prompt 从 stdin 传入避免命令行注入
//  3. **timeout 必选**：默认 60s，context cancel 杀进程
//  4. **N 上限 5**：避免订阅配额被快速耗尽
//  5. **启动侦测**：[claude_code_detect.go](claude_code_detect.go) ProbeClaudeCodeCLI
//
// 启用：在配置中提供 RIPPLE_CLAUDE_CODE_CLI_PATH（或留空走 PATH）即可注册。
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
		errOut := strings.TrimSpace(stderr.String())
		// 尝试识别配额相关错误，便于上层 router 决策 fallback
		lower := strings.ToLower(errOut)
		if strings.Contains(lower, "quota") || strings.Contains(lower, "rate limit") ||
			strings.Contains(lower, "usage limit") || strings.Contains(lower, "exhausted") {
			return "", fmt.Errorf("claude-code: quota exceeded: %s", errOut)
		}
		return "", fmt.Errorf("claude-code: exec failed: %w (stderr: %s)", err, errOut)
	}
	return stdout.String(), nil
}
