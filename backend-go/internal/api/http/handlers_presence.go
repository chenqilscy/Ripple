package httpapi

import (
	"net/http"

	"github.com/chenqilscy/ripple/backend-go/internal/presence"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// PresenceHandlers 在线状态 HTTP 处理器。
type PresenceHandlers struct {
	Presence *presence.Service
	Lakes    *service.LakeService // 用于读权限校验
}

// List GET /api/v1/lakes/{id}/presence
// 权限：湖可读（私有湖需成员）即可。
func (h *PresenceHandlers) List(w http.ResponseWriter, r *http.Request) {
	user, _ := CurrentUser(r.Context())
	lakeID := chi.URLParam(r, "id")

	if _, _, err := h.Lakes.Get(r.Context(), user, lakeID); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	users, err := h.Presence.List(r.Context(), lakeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}
