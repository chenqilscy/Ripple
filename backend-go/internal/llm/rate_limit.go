// Package llm · Provider 速率限制装饰器（TD-002 偿还）。
//
// 包装一个 Provider，在 Generate / GenerateStream 前申请令牌。
// 用 golang.org/x/time/rate 的 token bucket，参数：每秒 N 次 + burst。
//
// 设计：装饰器透明转发 Name() / Supports()；Generate 阻塞等待令牌（受 ctx 控制）。
package llm

import (
	"context"
	"fmt"

	"golang.org/x/time/rate"
)

// RateLimitedProvider 速率限制装饰器。
type RateLimitedProvider struct {
	inner   Provider
	limiter *rate.Limiter
}

// NewRateLimitedProvider 包装 inner provider。
//   - rps：每秒最多多少次 Generate 调用
//   - burst：令牌桶容量（短时高峰）
//   - rps<=0 表示不限制（直接返回 inner）
func NewRateLimitedProvider(inner Provider, rps float64, burst int) Provider {
	if rps <= 0 {
		return inner
	}
	if burst <= 0 {
		burst = 1
	}
	return &RateLimitedProvider{
		inner:   inner,
		limiter: rate.NewLimiter(rate.Limit(rps), burst),
	}
}

// Name 实现 Provider。
func (r *RateLimitedProvider) Name() string { return r.inner.Name() }

// Supports 实现 Provider。
func (r *RateLimitedProvider) Supports(m Modality) bool { return r.inner.Supports(m) }

// Generate 实现 Provider，先 Wait 令牌再转发。
func (r *RateLimitedProvider) Generate(ctx context.Context, req GenerateRequest) ([]Candidate, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("%s: rate limit wait: %w", r.inner.Name(), err)
	}
	return r.inner.Generate(ctx, req)
}

// GenerateStream 透传：若 inner 实现 StreamProvider 则也限速。
func (r *RateLimitedProvider) GenerateStream(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error) {
	sp, ok := r.inner.(StreamProvider)
	if !ok {
		return nil, fmt.Errorf("%s: does not support streaming", r.inner.Name())
	}
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("%s: rate limit wait: %w", r.inner.Name(), err)
	}
	return sp.GenerateStream(ctx, req)
}
