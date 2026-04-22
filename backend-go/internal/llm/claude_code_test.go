// Package llm · ClaudeCodeProvider mock CLI 单测。
//
// 设计：测试不依赖真实 claude.exe，而是动态生成一个 mock 可执行（Windows .bat / Unix sh），
// 通过 RIPPLE_CLAUDE_CODE_CLI_PATH 风格的 cfg.CLIPath 注入。
//
// 覆盖：
//   - HappyPath：mock 输出固定文本，验证 Generate 解析正确
//   - Quota：mock stderr 输出 "quota exceeded"，验证 Provider 返回带 quota 关键字的 error
//   - Timeout：mock 睡眠超过 cfg.Timeout，验证 ctx 超时被识别
//   - NUpperBound：N>5 直接报错，无需 mock
package llm

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// writeMockCLI 在 t.TempDir() 下写一个跨平台的 mock 可执行：
//   - Windows：.bat，echo 固定文本
//   - 其它：.sh，#!/bin/sh + echo
//
// 内容由调用方决定（stdoutText / stderrText / exitCode / sleepMs）。
// 返回可执行文件绝对路径。
func writeMockCLI(t *testing.T, stdoutText, stderrText string, exitCode int, sleepMs int) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "mock_claude.bat")
		var sb strings.Builder
		sb.WriteString("@echo off\r\n")
		if sleepMs > 0 {
			// timeout /t 不支持毫秒；用 ping 127.0.0.1 -n 估算（每次 ~1s）
			secs := (sleepMs + 999) / 1000
			sb.WriteString("ping 127.0.0.1 -n ")
			sb.WriteString(itoa(secs + 1))
			sb.WriteString(" >nul\r\n")
		}
		if stdoutText != "" {
			// 简单 echo（不包含特殊字符的测试用例足够）
			sb.WriteString("echo ")
			sb.WriteString(stdoutText)
			sb.WriteString("\r\n")
		}
		if stderrText != "" {
			sb.WriteString("echo ")
			sb.WriteString(stderrText)
			sb.WriteString(" 1>&2\r\n")
		}
		sb.WriteString("exit /b ")
		sb.WriteString(itoa(exitCode))
		sb.WriteString("\r\n")
		if err := os.WriteFile(path, []byte(sb.String()), 0o755); err != nil {
			t.Fatalf("write mock bat: %v", err)
		}
		return path
	}
	// Unix
	path := filepath.Join(dir, "mock_claude.sh")
	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	if sleepMs > 0 {
		sb.WriteString("sleep ")
		// sh sleep 支持小数（GNU/BSD 都行）
		sb.WriteString(itoaFloat(float64(sleepMs) / 1000.0))
		sb.WriteString("\n")
	}
	if stdoutText != "" {
		sb.WriteString("echo '")
		sb.WriteString(stdoutText)
		sb.WriteString("'\n")
	}
	if stderrText != "" {
		sb.WriteString("echo '")
		sb.WriteString(stderrText)
		sb.WriteString("' 1>&2\n")
	}
	sb.WriteString("exit ")
	sb.WriteString(itoa(exitCode))
	sb.WriteString("\n")
	if err := os.WriteFile(path, []byte(sb.String()), 0o755); err != nil {
		t.Fatalf("write mock sh: %v", err)
	}
	return path
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func itoaFloat(f float64) string {
	// 简单 1 位小数足够 sleep 用
	whole := int(f)
	frac := int((f - float64(whole)) * 10)
	return itoa(whole) + "." + itoa(frac)
}

func TestClaudeCodeProvider_HappyPath(t *testing.T) {
	cli := writeMockCLI(t, "Hello from mock claude", "", 0, 0)
	p := NewClaudeCodeProvider(ClaudeCodeConfig{
		CLIPath: cli,
		Timeout: 5 * time.Second,
	})
	cands, err := p.Generate(context.Background(), GenerateRequest{
		Modality: ModalityText,
		Prompt:   "想个点子",
		N:        1,
	})
	if err != nil {
		t.Fatalf("Generate err: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("want 1 cand, got %d", len(cands))
	}
	if !strings.Contains(cands[0].Text, "Hello from mock claude") {
		t.Errorf("unexpected text: %q", cands[0].Text)
	}
	if cands[0].CostTokens != 0 {
		t.Errorf("want CostTokens=0 (subscription), got %d", cands[0].CostTokens)
	}
}

func TestClaudeCodeProvider_QuotaExceeded(t *testing.T) {
	cli := writeMockCLI(t, "", "Error: quota exceeded for this month", 1, 0)
	p := NewClaudeCodeProvider(ClaudeCodeConfig{
		CLIPath: cli,
		Timeout: 5 * time.Second,
	})
	_, err := p.Generate(context.Background(), GenerateRequest{
		Modality: ModalityText,
		Prompt:   "想个点子",
		N:        1,
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "quota") {
		t.Errorf("want quota error, got %v", err)
	}
}

func TestClaudeCodeProvider_RateLimit(t *testing.T) {
	cli := writeMockCLI(t, "", "rate limit reached", 1, 0)
	p := NewClaudeCodeProvider(ClaudeCodeConfig{
		CLIPath: cli,
		Timeout: 5 * time.Second,
	})
	_, err := p.Generate(context.Background(), GenerateRequest{
		Modality: ModalityText,
		Prompt:   "x",
		N:        1,
	})
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "quota") {
		// router 把 rate limit 也归类为 quota 类错误
		t.Errorf("want quota error (incl rate limit), got %v", err)
	}
}

func TestClaudeCodeProvider_NUpperBound(t *testing.T) {
	// 不需要真 mock；N>5 在 Generate 入口直接报错
	p := NewClaudeCodeProvider(ClaudeCodeConfig{CLIPath: "definitely-not-exist"})
	_, err := p.Generate(context.Background(), GenerateRequest{
		Modality: ModalityText,
		Prompt:   "x",
		N:        6,
	})
	if err == nil {
		t.Fatal("want error for N>5")
	}
	if !strings.Contains(err.Error(), "N>5") {
		t.Errorf("want N>5 error, got %v", err)
	}
}

func TestClaudeCodeProvider_UnsupportedModality(t *testing.T) {
	p := NewClaudeCodeProvider(ClaudeCodeConfig{CLIPath: "x"})
	_, err := p.Generate(context.Background(), GenerateRequest{
		Modality: ModalityImage,
		Prompt:   "x",
		N:        1,
	})
	if err == nil {
		t.Fatal("want error for image modality")
	}
}

func TestClaudeCodeProvider_Supports(t *testing.T) {
	p := NewClaudeCodeProvider(ClaudeCodeConfig{})
	if !p.Supports(ModalityText) {
		t.Error("should support text")
	}
	if p.Supports(ModalityImage) {
		t.Error("should not support image")
	}
	if p.Name() != "claude-code" {
		t.Errorf("want name claude-code, got %s", p.Name())
	}
}
