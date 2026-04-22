// Package metrics · 全局默认 Registry 与标准计数器。
//
// 业务代码通过 `metrics.HTTPRequests.Inc()` 之类静态变量埋点，避免每次拼装。
// 真正的注册在 init() 一次完成，并发访问安全。
package metrics

// Default 是进程级默认 Registry。HTTP /metrics handler 从这里读。
var Default = NewRegistry()

// 标准 buckets：HTTP 请求 / LLM 调用都用毫秒级。
var defaultDurationBucketsMs = []float64{
	5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 30000,
}

// HTTPRequests HTTP 请求总数（不分维度的最低基线）。
var HTTPRequests = Default.CounterVec("ripple_http_requests_total", "Total HTTP requests received.", nil)

// HTTPDuration HTTP 请求耗时分布（毫秒）。
var HTTPDuration = Default.HistogramVec(
	"ripple_http_request_duration_ms",
	"HTTP request duration in milliseconds.",
	defaultDurationBucketsMs,
	nil,
)

// LLMCalls 按 provider 维度计数 LLM 调用。调用方用 LLMCallsBy(provider).Inc()。
func LLMCallsBy(provider string) *Counter {
	return Default.CounterVec(
		"ripple_llm_calls_total",
		"Total LLM provider calls (by provider).",
		map[string]string{"provider": provider},
	)
}

// LLMErrors 按 provider 维度计数 LLM 错误。
func LLMErrorsBy(provider string) *Counter {
	return Default.CounterVec(
		"ripple_llm_errors_total",
		"Total LLM provider errors (by provider).",
		map[string]string{"provider": provider},
	)
}

// LLMDuration LLM 调用耗时分布（毫秒）。
func LLMDurationBy(provider string) *Histogram {
	return Default.HistogramVec(
		"ripple_llm_call_duration_ms",
		"LLM call duration in milliseconds.",
		defaultDurationBucketsMs,
		map[string]string{"provider": provider},
	)
}

// WSConnections 当前活跃 WebSocket 连接数（gauge）。
var WSConnections = Default.GaugeVec("ripple_ws_connections", "Active WebSocket connections.", nil)

// WSMessagesIn / WSMessagesOut 消息计数。
var WSMessagesIn = Default.CounterVec("ripple_ws_messages_in_total", "WebSocket messages received from clients.", nil)
var WSMessagesOut = Default.CounterVec("ripple_ws_messages_out_total", "WebSocket messages sent to clients.", nil)
