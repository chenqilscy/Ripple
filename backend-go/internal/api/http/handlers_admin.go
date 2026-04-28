package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

type adminOrgLister interface {
	ListAll(ctx context.Context, limit int) ([]domain.Organization, error)
	CountAll(ctx context.Context) (int64, error)
}

type adminUserCounter interface {
	CountAll(ctx context.Context) (int64, error)
}

type adminGraylistCounter interface {
	CountAll(ctx context.Context) (int64, error)
}

type adminOverviewStatsResp struct {
	OrganizationsCount   int64 `json:"organizations_count"`
	UsersCount           int64 `json:"users_count"`
	GraylistEntriesCount int64 `json:"graylist_entries_count"`
}

type adminOverviewResp struct {
	Stats         adminOverviewStatsResp `json:"stats"`
	Organizations []orgOverviewResp      `json:"organizations"`
}

type AdminHandlers struct {
	OrgRepo      store.OrgRepository
	Quotas       store.OrgQuotaRepository
	Lakes        orgLakeLister
	Users        store.UserRepository
	Nodes        orgNodeCounter
	APIKeys      apiKeyOrgCounter
	Attachments  orgAttachmentUsageReader
	AuditLogs    store.AuditLogRepository
	Graylist     store.GraylistRepository
	AdminEmails  map[string]struct{}
}

func isPlatformAdminEmail(email string, adminEmails map[string]struct{}) bool {
	_, ok := adminEmails[strings.ToLower(strings.TrimSpace(email))]
	return ok
}

// Overview GET /api/v1/admin/overview
func (h *AdminHandlers) Overview(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !isPlatformAdminEmail(actor.Email, h.AdminEmails) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	lister, ok := h.OrgRepo.(adminOrgLister)
	if !ok || h.Quotas == nil {
		writeError(w, http.StatusServiceUnavailable, "admin overview not configured")
		return
	}
	orgs, err := lister.ListAll(r.Context(), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list orgs failed")
		return
	}
	orgCount, err := lister.CountAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "count orgs failed")
		return
	}
	userCount := int64(0)
	if counter, ok := h.Users.(adminUserCounter); ok {
		userCount, err = counter.CountAll(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "count users failed")
			return
		}
	}
	graylistCount := int64(0)
	if counter, ok := h.Graylist.(adminGraylistCounter); ok {
		graylistCount, err = counter.CountAll(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "count graylist failed")
			return
		}
	}
	out := make([]orgOverviewResp, 0, len(orgs))
	for i := range orgs {
		quota, err := h.Quotas.EnsureDefault(r.Context(), orgs[i].ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "load quota failed")
			return
		}
		quotaResp := toOrgQuotaResp(quota)
		usage, err := h.collectOrgUsage(r.Context(), orgs[i].ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "collect usage failed")
			return
		}
		quotaResp.Usage = usage
		overview := orgOverviewResp{
			Organization: toOrgResp(&orgs[i]),
			Quota:        quotaResp,
		}
		if h.AuditLogs != nil {
			logs, err := h.AuditLogs.ListByResource(r.Context(), "org_quota", orgs[i].ID, 1)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "list audits failed")
				return
			}
			overview.RecentQuotaAudits = make([]orgAuditLogResp, 0, len(logs))
			for _, log := range logs {
				overview.RecentQuotaAudits = append(overview.RecentQuotaAudits, orgAuditLogResp{
					ID:           log.ID,
					ActorID:      log.ActorID,
					Action:       log.Action,
					ResourceType: log.ResourceType,
					ResourceID:   log.ResourceID,
					Detail:       log.Detail,
					CreatedAt:    log.CreatedAt,
				})
			}
		}
		out = append(out, overview)
	}
	writeJSON(w, http.StatusOK, adminOverviewResp{
		Stats: adminOverviewStatsResp{
			OrganizationsCount:   orgCount,
			UsersCount:           userCount,
			GraylistEntriesCount: graylistCount,
		},
		Organizations: out,
	})
}

func (h *AdminHandlers) collectOrgUsage(ctx context.Context, orgID string) (*orgUsageResp, error) {
	if h.OrgRepo == nil || h.Lakes == nil || h.Nodes == nil || h.APIKeys == nil || h.Attachments == nil {
		return nil, nil
	}
	members, err := h.OrgRepo.ListMembers(ctx, orgID)
	if err != nil {
		return nil, err
	}
	lakes, err := h.Lakes.ListLakesByOrg(ctx, nil, orgID, nil)
	if err != nil {
		return nil, err
	}
	nodesUsed, err := h.Nodes.CountByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	attachmentsUsed, err := h.Attachments.CountByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	storageBytes, err := h.Attachments.SumSizeByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	apiKeysUsed, err := h.APIKeys.CountByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return &orgUsageResp{
		MembersUsed:     int64(len(members)),
		LakesUsed:       int64(len(lakes)),
		NodesUsed:       nodesUsed,
		AttachmentsUsed: attachmentsUsed,
		APIKeysUsed:     apiKeysUsed,
		StorageMBUsed:   bytesToQuotaMB(storageBytes),
	}, nil
}