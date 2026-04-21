package llm

import (
	"context"
	"fmt"
)

// 让 ZhipuClient 实现 Provider 接口。
// 旧的 `Generate(ctx, prompt, n) ([]string, error)` 保留作为内部实现细节，
// Provider.Generate 是新统一入口。

// Name implements Provider.
func (c *ZhipuClient) Name() string { return "zhipu" }

// Supports implements Provider. ZhipuAI v4 当前仅暴露文本与 embedding。
// 本轮仅 TEXT；EMBEDDING 留给后续 PR。
func (c *ZhipuClient) Supports(m Modality) bool {
	return m == ModalityText
}

// GenerateProvider 是 Provider.Generate 的实现（通过 wrapper 暴露，避免与旧
// Generate(prompt,n) 重名冲突）。
//
// 我们用方法别名重命名（rename refactor）会更干净，但为了不破坏既有 cloud_test.go，
// 这里保留两套签名：
//   - 旧 Generate(prompt,n) -> 现在内部叫 generateText
//   - Provider.Generate(req) 走 generateText
func (c *ZhipuClient) generateText(ctx context.Context, prompt string, n int) ([]string, error) {
	return c.Generate(ctx, prompt, n)
}

// providerAdapter 把 ZhipuClient 适配成 Provider（避免方法重名）。
type providerAdapter struct {
	c *ZhipuClient
}

func (p *providerAdapter) Name() string                    { return p.c.Name() }
func (p *providerAdapter) Supports(m Modality) bool        { return p.c.Supports(m) }
func (p *providerAdapter) Generate(ctx context.Context, req GenerateRequest) ([]Candidate, error) {
	if !p.c.Supports(req.Modality) {
		return nil, fmt.Errorf("zhipu: modality %s not supported", req.Modality)
	}
	if req.Modality != ModalityText {
		return nil, fmt.Errorf("zhipu: only TEXT in PR1")
	}
	lines, err := p.c.generateText(ctx, req.Prompt, req.N)
	if err != nil {
		return nil, err
	}
	out := make([]Candidate, 0, len(lines))
	for _, l := range lines {
		out = append(out, Candidate{
			Modality: ModalityText,
			Text:     l,
			MIME:     "text/plain",
		})
	}
	return out, nil
}

// AsProvider 暴露 Provider 视角。
func (c *ZhipuClient) AsProvider() Provider {
	return &providerAdapter{c: c}
}
