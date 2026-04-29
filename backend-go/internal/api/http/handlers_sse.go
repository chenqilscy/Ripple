package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/llm"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// WeaveStreamHandlers M3 流式编织（SSE）。
//
// 路由：GET /api/v1/lakes/{id}/weave/stream?prompt=...&modality=text&n=1
// 鉴权：复用 AuthMiddleware；同时校验 actor 是 lake 成员（任意角色可读）。
type WeaveStreamHandlers struct {
	Lakes  *service.LakeService
	Router llm.StreamProvider // 通常是 *llm.DefaultRouter
}

// Stream GET /api/v1/lakes/{id}/weave/stream
func (h *WeaveStreamHandlers) Stream(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	lakeID := chi.URLParam(r, "id")
	if lakeID == "" {
		writeError(w, http.StatusBadRequest, "lake id required")
		return
	}
	// 成员校验
	if _, _, err := h.Lakes.Get(r.Context(), u, lakeID); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}

	prompt := r.URL.Query().Get("prompt")
	if prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt required")
		return
	}
	if len([]rune(prompt)) > 4000 {
		writeError(w, http.StatusBadRequest, "prompt too long")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	ch, err := h.Router.GenerateStream(ctx, llm.GenerateRequest{
		Prompt:   prompt,
		Modality: llm.ModalityText,
		N:        1,
	})
	if err != nil {
		zerolog.Ctx(r.Context()).Error().Err(err).Str("lake_id", lakeID).Msg("weave stream error")
		writeSSE(w, flusher, "error", map[string]string{"message": "AI 暂歇，请稍候"})
		return
	}

	for chunk := range ch {
		if chunk.Err != nil {
			zerolog.Ctx(r.Context()).Error().Err(chunk.Err).Str("lake_id", lakeID).Msg("weave stream chunk error")
			writeSSE(w, flusher, "error", map[string]string{"message": "AI 生成中断，请稍后重试"})
			return
		}
		if chunk.Delta != "" {
			writeSSE(w, flusher, "delta", map[string]string{"text": chunk.Delta})
		}
		if chunk.Done {
			writeSSE(w, flusher, "done", map[string]int64{"cost_tokens": chunk.CostTokens})
			return
		}
	}
	// channel 关闭但未收到 Done：视为异常
	zerolog.Ctx(r.Context()).Warn().Str("lake_id", lakeID).Msg("weave stream closed without done")
	writeSSE(w, flusher, "error", map[string]string{"message": "AI 连接中断，请稍后重试"})
}

// writeSSE 写一条 SSE 事件并 flush。
func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		b = []byte(`{"message":"marshal error"}`)
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
	flusher.Flush()
}

// 防止未使用警告（errors 留作未来 errors.Is 使用）
var _ = errors.New
