package httpapi

import (
	"net/http"
	"strconv"

	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// LLMUsageHandlers Phase 15-D：LLM 用量查询端点。
type LLMUsageHandlers struct {
	Svc *service.LLMUsageService
}

// GetUsage GET /api/v1/organizations/{id}/llm_usage?days=30
func (h *LLMUsageHandlers) GetUsage(w http.ResponseWriter, r *http.Request) {
	_, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	orgID := chi.URLParam(r, "id")

	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			days = n
		}
	}

	usage, err := h.Svc.GetOrgUsage(r.Context(), orgID, days)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, usage)
}
