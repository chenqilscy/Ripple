// Package llm · M4-S2 多模态：图片生成 provider（占位实现）。
//
// 设计：
//   - 真实接入 DALL·E / 混元前的占位 Provider，行为可控。
//   - Modality=IMAGE 时返回一个 "data:image/svg+xml;base64,..." 的 BlobURL，
//     不依赖外部网络，便于压测 / E2E。
//   - 通过 PlaceholderImageProvider.Endpoint != "" 时切换为对外 HTTP 调用骨架（待接入）。
package llm

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PlaceholderImageProvider 占位图片 provider。
type PlaceholderImageProvider struct {
	NameStr  string // "dalle-stub" / "hunyuan-stub" 等
	SleepMS  int    // 模拟 LLM 延迟
	APIToken string // 真实接入时使用（当前未启用）
	Endpoint string // 真实接入时使用（当前未启用）
}

// NewPlaceholderImageProvider 构造。name 为空时回退 "image-stub"。
func NewPlaceholderImageProvider(name string, sleepMS int) *PlaceholderImageProvider {
	if name == "" {
		name = "image-stub"
	}
	if sleepMS < 0 {
		sleepMS = 0
	}
	return &PlaceholderImageProvider{NameStr: name, SleepMS: sleepMS}
}

// Name 实现 Provider。
func (p *PlaceholderImageProvider) Name() string { return p.NameStr }

// Supports 仅支持 IMAGE 模态。
func (p *PlaceholderImageProvider) Supports(m Modality) bool { return m == ModalityImage }

// Generate 返回 N 个 SVG data-URI 占位图片。
func (p *PlaceholderImageProvider) Generate(ctx context.Context, req GenerateRequest) ([]Candidate, error) {
	if req.Modality != ModalityImage {
		return nil, errors.New("placeholder image provider only supports IMAGE")
	}
	if req.N <= 0 {
		req.N = 1
	}
	if p.SleepMS > 0 {
		select {
		case <-time.After(time.Duration(p.SleepMS) * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	out := make([]Candidate, 0, req.N)
	for i := 0; i < req.N; i++ {
		svg := buildPlaceholderSVG(req.Prompt, i)
		uri := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
		out = append(out, Candidate{
			Modality:   ModalityImage,
			BlobURL:    uri,
			MIME:       "image/svg+xml",
			CostTokens: 1, // 占位计费
		})
	}
	return out, nil
}

func buildPlaceholderSVG(prompt string, idx int) string {
	// 简易：根据 prompt 取首 24 字符做标签
	label := strings.ReplaceAll(prompt, "<", "")
	label = strings.ReplaceAll(label, ">", "")
	if len(label) > 24 {
		label = label[:24] + "…"
	}
	colors := []string{"#4a8eff", "#9ec5ee", "#3a7eef", "#1d2433"}
	bg := colors[idx%len(colors)]
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 512 512">`+
		`<rect width="512" height="512" fill="%s"/>`+
		`<circle cx="256" cy="256" r="120" fill="rgba(255,255,255,0.15)"/>`+
		`<text x="256" y="256" fill="white" font-family="sans-serif" font-size="24" `+
		`text-anchor="middle" dominant-baseline="middle">%s</text>`+
		`<text x="256" y="490" fill="rgba(255,255,255,0.5)" font-family="monospace" `+
		`font-size="11" text-anchor="middle">ripple-image-stub #%d</text>`+
		`</svg>`, bg, label, idx)
}
