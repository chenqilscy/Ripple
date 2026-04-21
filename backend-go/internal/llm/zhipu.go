// Package llm 提供大模型客户端实现。
//
// 当前仅 ZhipuAI（GLM 系列），走 OpenAI 兼容协议
// （endpoint: https://open.bigmodel.cn/api/paas/v4/chat/completions）。
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

// Client LLM 客户端抽象。可注入 mock。
type Client interface {
	// Generate 调用 LLM 生成 n 个发散候选。返回 n 个字符串（可能少于 n）。
	Generate(ctx context.Context, prompt string, n int) ([]string, error)
}

// ZhipuClient 智谱 GLM HTTP 客户端。
type ZhipuClient struct {
	apiKey   string
	model    string
	endpoint string
	http     *http.Client
}

// NewZhipuClient 装配。endpoint 为空走默认。
func NewZhipuClient(apiKey, model, endpoint string) *ZhipuClient {
	if endpoint == "" {
		endpoint = "https://open.bigmodel.cn/api/paas/v4/chat/completions"
	}
	if model == "" {
		model = "glm-4-flash"
	}
	return &ZhipuClient{
		apiKey:   apiKey,
		model:    model,
		endpoint: endpoint,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

type zhipuReq struct {
	Model       string        `json:"model"`
	Messages    []zhipuMsg    `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	N           int           `json:"n,omitempty"`
}

type zhipuMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type zhipuResp struct {
	Choices []struct {
		Message zhipuMsg `json:"message"`
	} `json:"choices"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Generate 让 LLM 一次产 n 个发散方向。
// 实现策略：使用单次 prompt + 强约束，让模型用换行返回 n 行候选。
// 比 N 字段（不一定支持）更稳，且节省 token。
func (c *ZhipuClient) Generate(ctx context.Context, prompt string, n int) ([]string, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("zhipu: api key not configured")
	}
	if n <= 0 {
		n = 5
	}
	sys := fmt.Sprintf(`你是创意发散助手。用户给出一个想法/主题，请你产出 %d 个不同角度的发散方向。
要求：
1. 每个方向独立成行，前缀编号 "1. " "2. " 等。
2. 每行 ≤ 60 字。
3. 不要解释、不要总结、不要前后缀文本。
4. 严格输出 %d 行。`, n, n)
	body := zhipuReq{
		Model: c.model,
		Messages: []zhipuMsg{
			{Role: "system", Content: sys},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.9,
	}
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("zhipu request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("zhipu http %d: %s", resp.StatusCode, string(raw))
	}
	var r zhipuResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("zhipu decode: %w · body=%s", err, string(raw))
	}
	if r.Error != nil {
		return nil, fmt.Errorf("zhipu api error %s: %s", r.Error.Code, r.Error.Message)
	}
	if len(r.Choices) == 0 {
		return nil, fmt.Errorf("zhipu: empty choices")
	}
	return parseLines(r.Choices[0].Message.Content, n), nil
}

// parseLines 把 LLM 多行回复解析成 n 个候选。
// 容错：剥离编号前缀；空行跳过；多余的截断；不足的不补。
func parseLines(s string, n int) []string {
	out := []string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 剥离 "1. " "1) " "- " 等
		line = stripPrefix(line)
		if line == "" {
			continue
		}
		out = append(out, line)
		if len(out) >= n {
			break
		}
	}
	return out
}

func stripPrefix(line string) string {
	// 数字 + (.|))
	for i, ch := range line {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if i > 0 && (ch == '.' || ch == ')' || ch == '、') {
			rest := strings.TrimSpace(line[i+1:])
			return rest
		}
		break
	}
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
		return strings.TrimSpace(line[2:])
	}
	return line
}
