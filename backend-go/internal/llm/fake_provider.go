// Package llm · M4-T5 Fake provider for load testing without real API cost.
//
// 行为：
//   - Name() == "fake"
//   - Supports(TEXT) == true，其余 false
//   - Generate 返回固定 N 条 candidate，每条文本含可配置长度
//   - 可选延迟 sleepMS 模拟真实 LLM 响应时间（默认 0）
//
// 启用方式：cfg.LLMFake = true（envvar RIPPLE_LLM_FAKE=true）。
package llm

import (
	"context"
	"strings"
	"time"
)

// FakeProvider 不调用任何外部 API。
type FakeProvider struct {
	SleepMS    int
	TextLength int
}

// NewFakeProvider 装配；textLen<=0 时 = 200。
func NewFakeProvider(sleepMS, textLen int) *FakeProvider {
	if textLen <= 0 {
		textLen = 200
	}
	return &FakeProvider{SleepMS: sleepMS, TextLength: textLen}
}

// Name 返回 "fake"。
func (p *FakeProvider) Name() string { return "fake" }

// Supports 仅 TEXT。
func (p *FakeProvider) Supports(m Modality) bool { return m == ModalityText }

// Generate 返回 req.N 条固定文本（默认 1）。
func (p *FakeProvider) Generate(ctx context.Context, req GenerateRequest) ([]Candidate, error) {
	if p.SleepMS > 0 {
		select {
		case <-time.After(time.Duration(p.SleepMS) * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	n := req.N
	if n <= 0 {
		n = 1
	}
	// 注意：必须按 rune 截断，否则会切断 UTF-8 多字节字符导致下游 PG 报 0xe9 0x9d 错。
	body := strings.Repeat("青萍涟漪压测占位文本。", p.TextLength/12+1)
	if r := []rune(body); len(r) > p.TextLength {
		body = string(r[:p.TextLength])
	}
	out := make([]Candidate, n)
	for i := range out {
		out[i] = Candidate{
			Modality:   ModalityText,
			Text:       body,
			MIME:       "text/plain",
			CostTokens: int64(p.TextLength / 4),
		}
	}
	return out, nil
}
