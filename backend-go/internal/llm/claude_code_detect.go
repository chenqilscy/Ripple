// Package llm · Claude Code CLI 侦测（不依赖 build tag）。
//
// 用于启动时检查 CLI 是否可用，写入 zerolog 便于运维诊断。
// 不实际调用 LLM，只做 `which claude` + `claude --version` 验证。
package llm

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ClaudeCodeProbeResult 侦测结果。
type ClaudeCodeProbeResult struct {
	Available bool   // 是否找到可执行文件
	Path      string // 绝对路径（LookPath 解析后）
	Version   string // --version 输出（trim 后）
	Err       error  // 侦测过程中的错误（若 Available=false）
}

// ProbeClaudeCodeCLI 在启动时探测 Claude Code CLI。
//
//   - cliPath 传空时在 PATH 下查找 "claude"
//   - 整个过程上限 3 秒（--version 必须秒回）
func ProbeClaudeCodeCLI(ctx context.Context, cliPath string) ClaudeCodeProbeResult {
	if cliPath == "" {
		cliPath = "claude"
	}
	resolved, err := exec.LookPath(cliPath)
	if err != nil {
		return ClaudeCodeProbeResult{Available: false, Err: fmt.Errorf("lookPath: %w", err)}
	}

	subCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(subCtx, resolved, "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return ClaudeCodeProbeResult{
			Available: false, Path: resolved,
			Err: fmt.Errorf("claude --version: %w (%s)", err, strings.TrimSpace(out.String())),
		}
	}
	return ClaudeCodeProbeResult{
		Available: true, Path: resolved,
		Version: strings.TrimSpace(out.String()),
	}
}
