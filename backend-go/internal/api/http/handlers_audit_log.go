package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// AuditLogHandlers P10-B 审计日志查询端点。
//
//	GET /api/v1/audit_logs?resource_type=<type>&resource_id=<id>&limit=<n>
type AuditLogHandlers struct {
	Repo           store.AuditLogRepository
	Lakes          *service.LakeService
	Nodes          *service.NodeService
	Orgs           *service.OrgService
	PlatformAdmins store.PlatformAdminRepository
	AdminEmails    map[string]struct{}
}

// List GET /api/v1/audit_logs
// 查询参数：
//
//	resource_type  （必填）资源类型，e.g. "node"
//	resource_id    （必填）资源 ID
//	limit          最多返回条数，默认 50，上限 200
func (h *AuditLogHandlers) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	q := r.URL.Query()
	resourceType := q.Get("resource_type")
	resourceID := q.Get("resource_id")
	if resourceType == "" || resourceID == "" {
		writeError(w, http.StatusBadRequest, "resource_type and resource_id are required")
		return
	}
	if err := h.verifyResourceAccess(r.Context(), actor, resourceType, resourceID); err != nil {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	limit := parseIntQuery(r, "limit", 50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	logs, err := h.Repo.ListByResource(r.Context(), resourceType, resourceID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}

	type logView struct {
		ID           string         `json:"id"`
		ActorID      string         `json:"actor_id"`
		Action       string         `json:"action"`
		ResourceType string         `json:"resource_type"`
		ResourceID   string         `json:"resource_id"`
		Detail       map[string]any `json:"detail"`
		CreatedAt    time.Time      `json:"created_at"`
	}

	out := make([]logView, 0, len(logs))
	for _, l := range logs {
		out = append(out, logView{
			ID:           l.ID,
			ActorID:      l.ActorID,
			Action:       l.Action,
			ResourceType: l.ResourceType,
			ResourceID:   l.ResourceID,
			Detail:       l.Detail,
			CreatedAt:    l.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": out, "total": len(out)})
}

func (h *AuditLogHandlers) verifyResourceAccess(ctx context.Context, actor *domain.User, resourceType, resourceID string) error {
	rt := strings.ToLower(strings.TrimSpace(resourceType))

	switch rt {
	case "node":
		if h.Nodes == nil {
			return domain.ErrPermissionDenied
		}
		_, err := h.Nodes.Get(ctx, actor, resourceID)
		return err
	case "lake":
		if h.Lakes == nil {
			return domain.ErrPermissionDenied
		}
		_, _, err := h.Lakes.Get(ctx, actor, resourceID)
		return err
	case "organization", "org", "org_quota":
		if h.Orgs == nil {
			return domain.ErrPermissionDenied
		}
		_, err := h.Orgs.GetOrg(ctx, actor, resourceID)
		return err
	case "graylist", "platform_admin":
		ok, err := isPlatformAdmin(ctx, actor, h.AdminEmails, h.PlatformAdmins)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		return domain.ErrPermissionDenied
	default:
		return domain.ErrPermissionDenied
	}
}
