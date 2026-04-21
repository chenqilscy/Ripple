package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Policy 决定 Router 行为。
type Policy struct {
	EnableFallback bool // false（默认）→ 第一个支持 Modality 的 provider 失败就返
	MaxRetries     int  // 0 = 不重试（仅生效于 Fallback 内部相同 provider 的瞬时错误）
}

// DefaultRouter 选择策略：按 providers 注册顺序找第一个 Supports(Modality)。
// EnableFallback=true 时失败后顺位下一个。
type DefaultRouter struct {
	providers []Provider
	policy    Policy
	recorder  CallRecorder
}

// NewDefaultRouter 注册 providers（按优先级顺序），policy 与 CallRecorder。
func NewDefaultRouter(providers []Provider, policy Policy, recorder CallRecorder) *DefaultRouter {
	if recorder == nil {
		recorder = NoopRecorder{}
	}
	return &DefaultRouter{providers: providers, policy: policy, recorder: recorder}
}

// Generate 实现 Router。
func (r *DefaultRouter) Generate(ctx context.Context, req GenerateRequest) ([]Candidate, error) {
	if !req.Modality.IsValid() {
		return nil, fmt.Errorf("router: invalid modality %s", req.Modality)
	}
	if len(r.providers) == 0 {
		return nil, errors.New("router: no provider registered")
	}

	hash := promptHash(req.Prompt)
	var lastErr error
	tried := 0
	for _, p := range r.providers {
		if !p.Supports(req.Modality) {
			continue
		}
		tried++
		started := time.Now()
		out, err := p.Generate(ctx, req)
		latency := int(time.Since(started).Milliseconds())
		rec := CallRecord{
			Provider:    p.Name(),
			Modality:    req.Modality,
			PromptHash:  hash,
			CandidatesN: len(out),
			LatencyMS:   latency,
		}
		if err == nil {
			rec.Status = "ok"
			r.recorder.Record(rec)
			return out, nil
		}
		rec.Status = "error"
		rec.ErrorMessage = err.Error()
		r.recorder.Record(rec)
		lastErr = err
		if !r.policy.EnableFallback {
			break
		}
	}
	if tried == 0 {
		return nil, fmt.Errorf("router: no provider supports modality %s", req.Modality)
	}
	return nil, lastErr
}

// promptHash 用 sha256 前 16 字节的 hex（32 字符）。
func promptHash(p string) string {
	sum := sha256.Sum256([]byte(p))
	return hex.EncodeToString(sum[:16])
}
