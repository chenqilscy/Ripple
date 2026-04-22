package llm_test

import (
	"context"
	"os"
	"testing"

	"github.com/chenqilscy/ripple/backend-go/internal/llm"
)

// TestProbeClaudeCodeCLI_NotFound 默认情况（未安装 CLI）返回 Available=false。
// CI 常态应命中此路径（Windows/Linux runner 无 Claude Code 订阅）。
func TestProbeClaudeCodeCLI_NotFound(t *testing.T) {
	if _, err := os.Stat("/definitely/not/a/path/to/claude"); err == nil {
		t.Skip("unexpectedly found claude at fake path")
	}
	r := llm.ProbeClaudeCodeCLI(context.Background(), "/definitely/not/a/path/to/claude")
	if r.Available {
		t.Fatalf("want Available=false, got %+v", r)
	}
	if r.Err == nil {
		t.Fatal("want non-nil Err")
	}
}

// TestProbeClaudeCodeCLI_Real 本地有 CLI 时验证能探测到。
// 触发：RIPPLE_CLAUDE_CODE_CLI=1（表示本机安装了 claude）
func TestProbeClaudeCodeCLI_Real(t *testing.T) {
	if os.Getenv("RIPPLE_CLAUDE_CODE_CLI") != "1" {
		t.Skip("set RIPPLE_CLAUDE_CODE_CLI=1 to verify real CLI probe")
	}
	r := llm.ProbeClaudeCodeCLI(context.Background(), "")
	if !r.Available {
		t.Fatalf("want Available=true, got %+v", r)
	}
	if r.Path == "" || r.Version == "" {
		t.Fatalf("want non-empty Path/Version, got %+v", r)
	}
	t.Logf("claude at %s: %s", r.Path, r.Version)
}
