package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/google/uuid"
)

// RecommenderHandlers M3-S3 推荐与反馈端点骨架。
type RecommenderHandlers struct {
	Svc      *service.RecommenderService
	Feedback store.FeedbackRepository
}

// Recommend GET /api/v1/recommendations?target_type=perma_node&limit=20
func (h *RecommenderHandlers) Recommend(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	tt := r.URL.Query().Get("target_type")
	if tt == "" {
		writeError(w, http.StatusBadRequest, "target_type required")
		return
	}
	limit := parseIntSafe(r.URL.Query().Get("limit"), 20)
	recs, err := h.Svc.Recommend(r.Context(), u, service.RecommendInput{TargetType: tt, Limit: limit})
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recommendations": recs})
}

type feedbackReq struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	EventType  string `json:"event_type"`
	Payload    string `json:"payload,omitempty"`
}

// AddFeedback POST /api/v1/feedback
func (h *RecommenderHandlers) AddFeedback(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	var in feedbackReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.TargetType == "" || in.TargetID == "" || in.EventType == "" {
		writeError(w, http.StatusBadRequest, "target_type/target_id/event_type required")
		return
	}
	id := uuid.NewString()
	if err := h.Feedback.AddEvent(r.Context(), store.FeedbackEvent{
		ID: id, UserID: u.ID,
		TargetType: in.TargetType, TargetID: in.TargetID,
		EventType: in.EventType, Payload: in.Payload,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// parseIntSafe 简单解析正整数；失败/0 返回 def。
func parseIntSafe(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
		if n > 1<<20 {
			return def
		}
	}
	if n == 0 {
		return def
	}
	return n
}
