// Package llm · OpenAI 兼容协议通用客户端。
//
// 涵盖以下 provider（v1/chat/completions 协议）：
//   - openai      : https://api.openai.com/v1
//   - deepseek    : https://api.deepseek.com/v1
//   - volc/doubao : https://ark.cn-beijing.volces.com/api/v3
//   - minimax v2  : https://api.minimaxi.com/v1
//   - 任意 openai-compatible : 自配 baseURL（Ollama / vLLM / SGLang 等本地）
//
// 与 ZhipuClient 在 zhipu.go 中独立，因 Zhipu 已先实现且 e2e 验证；
// 共用一份逻辑会引入历史包袱，保持各自独立更稳。

package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAICompatClient OpenAI 兼容 Chat Completion 客户端。
// 多个 provider 共用此结构，区别在于 baseURL/apiKey/model/name。
type OpenAICompatClient struct {
	name     string // "openai" / "deepseek" / "volc" / "minimax" / "openai-compat"
	apiKey   string
	model    string
	endpoint string // 完整的 chat/completions URL
	http     *http.Client
}

// OpenAICompatConfig 构造参数。
type OpenAICompatConfig struct {
	Name      string        // provider 名（写入 llm_calls.provider）
	APIKey    string        // Bearer token
	Model     string        // 模型名（如 gpt-4o-mini / deepseek-chat / doubao-1.5-pro-32k / abab6.5s-chat）
	Endpoint  string        // 完整 chat/completions URL；不传则按 Name 默认
	Timeout   time.Duration // 默认 30s
}

// NewOpenAICompatClient 构造。endpoint 为空时按 name 选择默认。
func NewOpenAICompatClient(cfg OpenAICompatConfig) *OpenAICompatClient {
	ep := cfg.Endpoint
	if ep == "" {
		ep = defaultEndpointFor(cfg.Name)
	}
	to := cfg.Timeout
	if to == 0 {
		to = 30 * time.Second
	}
	return &OpenAICompatClient{
		name:     cfg.Name,
		apiKey:   cfg.APIKey,
		model:    cfg.Model,
		endpoint: ep,
		http:     &http.Client{Timeout: to},
	}
}

func defaultEndpointFor(name string) string {
	switch strings.ToLower(name) {
	case "openai":
		return "https://api.openai.com/v1/chat/completions"
	case "deepseek":
		return "https://api.deepseek.com/v1/chat/completions"
	case "volc", "doubao":
		return "https://ark.cn-beijing.volces.com/api/v3/chat/completions"
	case "minimax":
		return "https://api.minimaxi.com/v1/text/chatcompletion_v2"
	default:
		return ""
	}
}

// Name implements Provider.
func (c *OpenAICompatClient) Name() string { return c.name }

// Supports implements Provider. 当前所有 OpenAI 兼容 provider 仅支持文本。
// 图像/音频走专门的 provider（待后续 PR）。
func (c *OpenAICompatClient) Supports(m Modality) bool {
	return m == ModalityText
}

type ocChatReq struct {
	Model       string      `json:"model"`
	Messages    []ocMessage `json:"messages"`
	Temperature float64     `json:"temperature,omitempty"`
	MaxTokens   int         `json:"max_tokens,omitempty"`
}

type ocMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ocChatResp struct {
	Choices []struct {
		Message ocMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int64 `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Generate implements Provider. 仅支持 ModalityText。
func (c *OpenAICompatClient) Generate(ctx context.Context, req GenerateRequest) ([]Candidate, error) {
	if req.Modality != ModalityText {
		return nil, fmt.Errorf("%s: modality %s not supported", c.name, req.Modality)
	}
	if c.apiKey == "" {
		return nil, fmt.Errorf("%s: api key not configured", c.name)
	}
	if c.endpoint == "" {
		return nil, fmt.Errorf("%s: endpoint not set", c.name)
	}
	n := req.N
	if n <= 0 {
		n = 5
	}
	temp := 0.9
	maxTok := 0
	if hints, ok := req.Hints.(TextHints); ok {
		if hints.Temperature > 0 {
			temp = hints.Temperature
		}
		maxTok = hints.MaxTokens
	}

	sys := fmt.Sprintf(`你是创意发散助手。用户给出一个想法/主题，请你产出 %d 个不同角度的发散方向。
要求：
1. 每个方向独立成行，前缀编号 "1. " "2. " 等。
2. 每行 ≤ 60 字。
3. 不要解释、不要总结、不要前后缀文本。
4. 严格输出 %d 行。`, n, n)

	body := ocChatReq{
		Model: c.model,
		Messages: []ocMessage{
			{Role: "system", Content: sys},
			{Role: "user", Content: req.Prompt},
		},
		Temperature: temp,
		MaxTokens:   maxTok,
	}
	buf, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s request: %w", c.name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s http %d: %s", c.name, resp.StatusCode, truncate(string(raw), 300))
	}
	var r ocChatResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("%s decode: %w", c.name, err)
	}
	if r.Error != nil {
		return nil, fmt.Errorf("%s api error %s: %s", c.name, r.Error.Code, r.Error.Message)
	}
	if len(r.Choices) == 0 {
		return nil, fmt.Errorf("%s: empty choices", c.name)
	}
	lines := parseLines(r.Choices[0].Message.Content, n)
	out := make([]Candidate, 0, len(lines))
	costPer := r.Usage.TotalTokens
	if len(lines) > 0 {
		costPer = r.Usage.TotalTokens / int64(len(lines)) // 平均摊到每个候选
	}
	for _, l := range lines {
		out = append(out, Candidate{
			Modality:   ModalityText,
			Text:       l,
			MIME:       "text/plain",
			CostTokens: costPer,
		})
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
