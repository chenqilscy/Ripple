package httpapi

import (
	"net/http"
	"strconv"

	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// LLMUsageHandlers Phase 15-D：LLM 用量查询端点。
type LLMUsageHandlers struct {
	Svc  *service.LLMUsageService
	Orgs *service.OrgService // 用于鉴权校验（可 nil 则跳过）
}

// GetUsage GET /api/v1/organizations/{id}/llm_usage?days=30
func (h *LLMUsageHandlers) GetUsage(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	orgID := chi.URLParam(r, "id")

	// 权限校验：必须是组织成员
	if h.Orgs != nil {
		isMember, err := h.Orgs.IsMember(r.Context(), u.ID, orgID)
		if err != nil || !isMember {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
	}

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
