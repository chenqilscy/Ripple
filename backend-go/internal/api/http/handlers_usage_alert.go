package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// OrgMembershipChecker 组织成员权限检查接口。
type OrgMembershipChecker interface {
	IsMember(ctx context.Context, userID, orgID string) (bool, error)
	GetMemberRole(ctx context.Context, orgID, userID string) (domain.OrgRole, error)
}

// UsageAlertHandlers Phase 15.2: 用量告警端点。
type UsageAlertHandlers struct {
	Svc  *service.UsageAlertService
	Orgs OrgMembershipChecker // 用于鉴权校验（可 nil 则跳过）
}

type usageAlertResp struct {
	OrgID            string     `json:"org_id"`
	ThresholdPercent int        `json:"threshold_percent"`
	Enabled          bool       `json:"enabled"`
	LastTriggeredAt  *time.Time `json:"last_triggered_at,omitempty"`
}

func toUsageAlertResp(a *service.UsageAlert) usageAlertResp {
	return usageAlertResp{
		OrgID:            a.OrgID,
		ThresholdPercent: a.ThresholdPercent,
		Enabled:          a.Enabled,
	}
}

// GetUsageAlert GET /api/v1/organizations/{id}/usage-alert
func (h *UsageAlertHandlers) GetUsageAlert(w http.ResponseWriter, r *http.Request) {
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

	alert, err := h.Svc.GetAlert(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get usage alert")
		return
	}
	if alert == nil {
		// 返回默认值：阈值 80%，未启用
		writeJSON(w, http.StatusOK, usageAlertResp{
			OrgID:            orgID,
			ThresholdPercent: 80,
			Enabled:          false,
		})
		return
	}
	writeJSON(w, http.StatusOK, toUsageAlertResp(alert))
}

type usageAlertReq struct {
	ThresholdPercent int  `json:"threshold_percent"`
	Enabled          bool `json:"enabled"`
}

// UpdateUsageAlert PUT /api/v1/organizations/{id}/usage-alert
func (h *UsageAlertHandlers) UpdateUsageAlert(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	orgID := chi.URLParam(r, "id")

	// 权限校验：必须是组织 OWNER
	if h.Orgs != nil {
		role, err := h.Orgs.GetMemberRole(r.Context(), orgID, u.ID)
		if err != nil || role != domain.OrgRoleOwner {
			writeError(w, http.StatusForbidden, "only org owner can manage usage alert")
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var in usageAlertReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if in.ThresholdPercent < 0 || in.ThresholdPercent > 100 {
		writeError(w, http.StatusBadRequest, "threshold_percent must be between 0 and 100")
		return
	}

	alert, err := h.Svc.UpdateAlert(r.Context(), orgID, in.ThresholdPercent, in.Enabled)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update usage alert")
		return
	}
	writeJSON(w, http.StatusOK, toUsageAlertResp(alert))
}
