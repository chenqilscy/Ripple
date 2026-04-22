package llm

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// GenerateStream 实现 StreamProvider。
//
// 选择策略：
//  1. 按 providers 注册顺序，找第一个 Supports(Modality) 且实现 StreamProvider 的；
//  2. 若没有任何 provider 实现流式但有支持 modality 的，降级为调用 Generate（n=1），
//     把 Text 一次性当 Delta 推出去后 Done，让上层 UI 仍可走流式通道；
//  3. 都没有则返回错误。
//
// EnableFallback=true 时，首选 provider 启动失败（Initial err）会顺位下一个；
// 但流式 chunk 中途的 Err 不再切换 provider（避免半路换源造成上下文撕裂）。
func (r *DefaultRouter) GenerateStream(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error) {
	if !req.Modality.IsValid() {
		return nil, fmt.Errorf("router: invalid modality %s", req.Modality)
	}
	if len(r.providers) == 0 {
		return nil, errors.New("router: no provider registered")
	}

	hash := promptHash(req.Prompt)
	var lastErr error
	streamTried := 0
	for _, p := range r.providers {
		if !p.Supports(req.Modality) {
			continue
		}
		sp, ok := p.(StreamProvider)
		if !ok {
			continue
		}
		streamTried++
		started := time.Now()
		ch, err := sp.GenerateStream(ctx, req)
		if err != nil {
			r.recorder.Record(CallRecord{
				Provider: p.Name(), Modality: req.Modality, PromptHash: hash,
				LatencyMS: int(time.Since(started).Milliseconds()),
				Status:    "error", ErrorMessage: "stream init: " + err.Error(),
			})
			lastErr = err
			if !r.policy.EnableFallback {
				break
			}
			continue
		}
		// 包一层把最终统计写进 recorder
		return r.wrapStream(ch, p.Name(), req.Modality, hash, started), nil
	}

	if streamTried == 0 {
		// 降级：找首个支持 modality 的 Generate 调一次
		return r.fallbackGenerateAsStream(ctx, req, hash)
	}
	return nil, lastErr
}

// wrapStream 包装上游 chunk，结束时记录一条 CallRecord。
func (r *DefaultRouter) wrapStream(in <-chan StreamChunk, name string, mod Modality, hash string, started time.Time) <-chan StreamChunk {
	out := make(chan StreamChunk, 16)
	go func() {
		defer close(out)
		var totalCost int64
		var sawErr error
		for c := range in {
			if c.Err != nil {
				sawErr = c.Err
			}
			if c.CostTokens > 0 {
				totalCost += c.CostTokens
			}
			out <- c
		}
		status := "ok"
		errMsg := ""
		if sawErr != nil {
			status = "error"
			errMsg = sawErr.Error()
		}
		r.recorder.Record(CallRecord{
			Provider: name, Modality: mod, PromptHash: hash,
			CandidatesN: 1, LatencyMS: int(time.Since(started).Milliseconds()),
			Status: status, ErrorMessage: errMsg,
		})
		_ = totalCost
	}()
	return out
}

// fallbackGenerateAsStream 把 Generate 的一次性结果"伪装"成流。
// 仅在没有 provider 实现 StreamProvider 时使用。
func (r *DefaultRouter) fallbackGenerateAsStream(ctx context.Context, req GenerateRequest, hash string) (<-chan StreamChunk, error) {
	// n=1 即可
	req2 := req
	if req2.N <= 0 {
		req2.N = 1
	}
	out := make(chan StreamChunk, 2)
	go func() {
		defer close(out)
		started := time.Now()
		// 找第一个支持 modality 的（不要求 stream）
		for _, p := range r.providers {
			if !p.Supports(req2.Modality) {
				continue
			}
			cands, err := p.Generate(ctx, req2)
			if err != nil {
				out <- StreamChunk{Err: err}
				r.recorder.Record(CallRecord{
					Provider: p.Name(), Modality: req2.Modality, PromptHash: hash,
					LatencyMS: int(time.Since(started).Milliseconds()),
					Status:    "error", ErrorMessage: err.Error(),
				})
				return
			}
			text := ""
			if len(cands) > 0 {
				text = cands[0].Text
			}
			out <- StreamChunk{Delta: text}
			out <- StreamChunk{Done: true}
			r.recorder.Record(CallRecord{
				Provider: p.Name(), Modality: req2.Modality, PromptHash: hash,
				CandidatesN: len(cands),
				LatencyMS:   int(time.Since(started).Milliseconds()),
				Status:      "ok",
			})
			return
		}
		out <- StreamChunk{Err: fmt.Errorf("router: no provider supports modality %s", req2.Modality)}
	}()
	return out, nil
}
