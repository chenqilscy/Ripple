package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/llm"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// NodeAIHandlers P16-B：节点 AI 摘要处理器。
type NodeAIHandlers struct {
	Nodes  *service.NodeService
	Router llm.StreamProvider // 可选；nil 时返回 503
}

type aiSummaryResp struct {
	NodeID    string    `json:"node_id"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
}

// AISummary POST /api/v1/nodes/{id}/ai_summary
//
// 调用 LLM 为节点内容生成一句话摘要（≤60字）。
// - 节点需可读（节点服务 Get 已包含权限检查）
// - LLM Router 未配置时返回 503
// - LLM 超时 30s
func (h *NodeAIHandlers) AISummary(w http.ResponseWriter, r *http.Request) {
	if h.Router == nil {
		writeError(w, http.StatusServiceUnavailable, "AI summary service not configured")
		return
	}

	u, _ := CurrentUser(r.Context())
	nodeID := chi.URLParam(r, "id")

	node, err := h.Nodes.Get(r.Context(), u, nodeID)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}

	content := strings.TrimSpace(node.Content)
	if content == "" {
		writeJSON(w, http.StatusOK, aiSummaryResp{
			NodeID:    nodeID,
			Summary:   "（节点内容为空）",
			CreatedAt: time.Now().UTC(),
		})
		return
	}

	// 截取前 2000 字符，避免超出模型上下文
	runes := []rune(content)
	if len(runes) > 2000 {
		content = string(runes[:2000])
	}

	prompt := "请用一句话（不超过60字）概括以下内容的核心含义，只输出摘要本身，不加任何说明：\n\n" + content

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	chunks, err := h.Router.GenerateStream(ctx, llm.GenerateRequest{
		Prompt:   prompt,
		N:        1,
		Modality: llm.ModalityText,
		Hints:    llm.TextHints{Temperature: 0.3, MaxTokens: 120},
	})
	if err != nil {
		zerolog.Ctx(r.Context()).Error().Err(err).Str("node_id", nodeID).Msg("ai_summary llm error")
		writeError(w, http.StatusInternalServerError, "AI 服务暂时不可用，请稍后重试")
		return
	}

	var sb strings.Builder
	var streamErr error
	for chunk := range chunks {
		if chunk.Err != nil {
			streamErr = chunk.Err
			// drain remaining chunks to unblock the provider goroutine
			for range chunks { //nolint:revive
			}
			break
		}
		sb.WriteString(chunk.Delta)
	}
	if streamErr != nil {
		zerolog.Ctx(r.Context()).Error().Err(streamErr).Str("node_id", nodeID).Msg("ai_summary stream error")
		writeError(w, http.StatusInternalServerError, "AI 生成中断，请稍后重试")
		return
	}

	summary := strings.TrimSpace(sb.String())
	if summary == "" {
		summary = "（AI 未生成摘要）"
	}

	writeJSON(w, http.StatusOK, aiSummaryResp{
		NodeID:    nodeID,
		Summary:   summary,
		CreatedAt: time.Now().UTC(),
	})
}
