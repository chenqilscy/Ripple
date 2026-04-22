// Package llm · OpenAICompatClient 流式输出实现（TD-001 第一步）。
//
// SSE 协议：每行 `data: <json>` 或 `data: [DONE]` 结束。
// 仅支持 ModalityText；其他模态返回错误。
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type ocStreamChoice struct {
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

type ocStreamFrame struct {
	Choices []ocStreamChoice `json:"choices"`
	Usage   *struct {
		TotalTokens int64 `json:"total_tokens"`
	} `json:"usage"`
}

// GenerateStream 实现 StreamProvider。
func (c *OpenAICompatClient) GenerateStream(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error) {
	if req.Modality != ModalityText {
		return nil, fmt.Errorf("%s: stream modality %s not supported", c.name, req.Modality)
	}
	if c.apiKey == "" {
		return nil, fmt.Errorf("%s: api key not configured", c.name)
	}
	if c.endpoint == "" {
		return nil, fmt.Errorf("%s: endpoint not set", c.name)
	}

	temp := 0.9
	maxTok := 0
	if hints, ok := req.Hints.(TextHints); ok {
		if hints.Temperature > 0 {
			temp = hints.Temperature
		}
		maxTok = hints.MaxTokens
	}

	body := struct {
		ocChatReq
		Stream bool `json:"stream"`
	}{
		ocChatReq: ocChatReq{
			Model: c.model,
			Messages: []ocMessage{
				{Role: "user", Content: req.Prompt},
			},
			Temperature: temp,
			MaxTokens:   maxTok,
		},
		Stream: true,
	}
	buf, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s stream request: %w", c.name, err)
	}
	if resp.StatusCode >= 400 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%s stream http %d", c.name, resp.StatusCode)
	}

	out := make(chan StreamChunk, 16)
	go func() {
		defer close(out)
		defer func() { _ = resp.Body.Close() }()
		scanner := bufio.NewScanner(resp.Body)
		// 单行最大 1MB，足够单个 SSE 帧
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var totalTokens int64
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" {
				continue
			}
			if payload == "[DONE]" {
				select {
				case out <- StreamChunk{Done: true, CostTokens: totalTokens}:
				case <-ctx.Done():
				}
				return
			}
			var frame ocStreamFrame
			if err := json.Unmarshal([]byte(payload), &frame); err != nil {
				select {
				case out <- StreamChunk{Err: fmt.Errorf("%s decode frame: %w", c.name, err)}:
				case <-ctx.Done():
				}
				return
			}
			if frame.Usage != nil {
				totalTokens = frame.Usage.TotalTokens
			}
			for _, ch := range frame.Choices {
				if ch.Delta.Content == "" {
					continue
				}
				select {
				case out <- StreamChunk{Delta: ch.Delta.Content}:
				case <-ctx.Done():
					return
				}
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			select {
			case out <- StreamChunk{Err: fmt.Errorf("%s stream scan: %w", c.name, err)}:
			case <-ctx.Done():
			}
		}
	}()
	return out, nil
}
