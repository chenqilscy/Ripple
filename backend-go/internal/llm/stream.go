// Package llm · 流式输出可选接口（TD-001 偿还第一步）。
//
// Provider 可选实现 StreamProvider 接口；不实现则视为不支持流式。
// Router 暂未集成（待 M3 CrystallizeWorker 上线时再接入）；当前仅
// OpenAICompatClient 提供 GenerateStream 实现，便于业务直接调用。
package llm

import (
	"context"
)

// StreamChunk 流式输出单元。
//   - Delta 是增量文本；Done=true 时 Delta 可能为空且携带累计 token usage（CostTokens）。
//   - 同一次请求 yield 的 chunk 顺序保证。
type StreamChunk struct {
	Delta      string
	Done       bool
	CostTokens int64
	Err        error
}

// StreamProvider 可选接口。Provider 实现此接口表示支持流式输出。
//
// 调用约定：
//   - ctx cancel 必须及时关闭返回 channel
//   - 出错通过最后一个 chunk 的 Err 字段传递；之后 channel 必须关闭
//   - 调用方负责消费完 channel；不消费会泄漏 goroutine
type StreamProvider interface {
	GenerateStream(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)
}
