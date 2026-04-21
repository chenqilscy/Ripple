// Package llm 提供 LLM 调用的统一抽象。
//
// 三层结构：
//   - Provider：厂商适配器（zhipu / openai / mock 等），只关心 HTTP 协议。
//   - Router：业务策略（按 Modality 选 provider、可选 fallback、计费记录）。
//   - CallRecorder：把每次调用写入 llm_calls 表（异步，不阻塞业务）。
//
// 业务层（如 service.AIWeaver）只依赖 Router，不耦合具体厂商。
package llm

import (
	"context"
)

// Modality 表示生成内容的模态。
type Modality string

const (
	ModalityText      Modality = "TEXT"
	ModalityImage     Modality = "IMAGE"
	ModalityAudio     Modality = "AUDIO"
	ModalityEmbedding Modality = "EMBEDDING"
)

// IsValid 是否合法 Modality。
func (m Modality) IsValid() bool {
	switch m {
	case ModalityText, ModalityImage, ModalityAudio, ModalityEmbedding:
		return true
	}
	return false
}

// Hints 是 typed sub-struct 的标记接口，避免 map[string]any 隐式契约。
type Hints interface {
	isHints()
}

// TextHints 文本生成可调参数（白名单）。
type TextHints struct {
	Temperature float64 // 0.0 ~ 1.0
	MaxTokens   int     // 0 = provider 默认
}

func (TextHints) isHints() {}

// ImageHints 图像生成参数。
type ImageHints struct {
	Size string // "512x512" 等
}

func (ImageHints) isHints() {}

// GenerateRequest 业务侧的统一请求。
type GenerateRequest struct {
	Prompt   string
	N        int
	Modality Modality
	Hints    Hints
}

// Candidate 一个候选输出。Text/BlobURL 互斥，按 Modality 决定哪个有效。
type Candidate struct {
	Modality   Modality
	Text       string // ModalityText 时填
	BlobURL    string // ModalityImage/Audio 时填（落 MinIO 后的 URL）
	MIME       string
	CostTokens int64 // provider 返回的计费单位（统一用 token；图像/音频折算）
}

// Provider 厂商适配器。无状态，goroutine-safe。
type Provider interface {
	Name() string                                                // "zhipu" / "openai" / "mock"
	Supports(Modality) bool                                      // 支持哪些模态
	Generate(ctx context.Context, req GenerateRequest) ([]Candidate, error)
}

// Router 业务侧入口。负责选择 Provider、（可选）fallback、记录调用。
type Router interface {
	Generate(ctx context.Context, req GenerateRequest) ([]Candidate, error)
}

// CallRecorder 持久化每次 LLM 调用的元数据，用于审计/计费/A-B 分析。
// 实现应当**异步**写入（chan + worker），不能阻塞 Generate。
type CallRecorder interface {
	Record(rec CallRecord)
}

// CallRecord 单次调用元数据。
type CallRecord struct {
	Provider     string
	Modality     Modality
	PromptHash   string // sha256 hex (前 16 字节)
	CandidatesN  int
	CostTokens   int64
	LatencyMS    int
	Status       string // "ok" | "error"
	ErrorMessage string
}

// NoopRecorder 不记录（默认实现，便于测试）。
type NoopRecorder struct{}

func (NoopRecorder) Record(_ CallRecord) {}
