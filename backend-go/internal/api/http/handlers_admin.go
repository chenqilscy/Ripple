package httpapi

import (
	"context"
	"net/http"

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

type adminQuotaBatchEnsurer interface {
	EnsureDefaults(ctx context.Context, orgIDs []string) (map[string]*domain.OrgQuota, error)
}

type adminMemberBatchCounter interface {
	CountMembersByOrgIDs(ctx context.Context, orgIDs []string) (map[string]int64, error)
}

type adminLakeBatchCounter interface {
	CountLakesByOrgIDs(ctx context.Context, orgIDs []string) (map[string]int64, error)
}

type adminNodeBatchCounter interface {
	CountByOrgIDs(ctx context.Context, orgIDs []string) (map[string]int64, error)
}

type adminAPIKeyBatchCounter interface {
	CountByOrgIDs(ctx context.Context, orgIDs []string) (map[string]int64, error)
}

type adminAttachmentBatchUsageReader interface {
	CountByOrgIDs(ctx context.Context, orgIDs []string) (map[string]int64, error)
	SumSizeByOrgIDs(ctx context.Context, orgIDs []string) (map[string]int64, error)
}

type adminAuditBatchLister interface {
	ListLatestByResources(ctx context.Context, resourceType string, resourceIDs []string, limitPerResource int) (map[string][]*domain.AuditLog, error)
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
	OrgRepo        store.OrgRepository
	Quotas         store.OrgQuotaRepository
	Lakes          orgLakeLister
	Users          store.UserRepository
	Nodes          orgNodeCounter
	APIKeys        apiKeyOrgCounter
	Attachments    orgAttachmentUsageReader
	AuditLogs      store.AuditLogRepository
	Graylist       store.GraylistRepository
	PlatformAdmins store.PlatformAdminRepository
	AdminEmails    map[string]struct{}
}

// Overview GET /api/v1/admin/overview
func (h *AdminHandlers) Overview(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	isAdmin, err := isPlatformAdmin(r.Context(), actor, h.AdminEmails, h.PlatformAdmins)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "platform admin check failed")
		return
	}
	if !isAdmin {
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
	orgIDs := make([]string, 0, len(orgs))
	for i := range orgs {
		orgIDs = append(orgIDs, orgs[i].ID)
	}
	quotas, err := h.loadOrgQuotas(r.Context(), orgIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load quota failed")
		return
	}
	usageByOrg, err := h.collectOrgUsages(r.Context(), orgIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "collect usage failed")
		return
	}
	auditsByOrg, err := h.loadRecentQuotaAudits(r.Context(), orgIDs, 1)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list audits failed")
		return
	}
	out := make([]orgOverviewResp, 0, len(orgs))
	for i := range orgs {
		quota := quotas[orgs[i].ID]
		if quota == nil {
			writeError(w, http.StatusInternalServerError, "load quota failed")
			return
		}
		quotaResp := toOrgQuotaResp(quota)
		quotaResp.Usage = usageByOrg[orgs[i].ID]
		overview := orgOverviewResp{
			Organization: toOrgResp(&orgs[i]),
			Quota:        quotaResp,
		}
		overview.RecentQuotaAudits = auditLogsToResp(auditsByOrg[orgs[i].ID])
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

func (h *AdminHandlers) loadOrgQuotas(ctx context.Context, orgIDs []string) (map[string]*domain.OrgQuota, error) {
	out := make(map[string]*domain.OrgQuota, len(orgIDs))
	if batcher, ok := h.Quotas.(adminQuotaBatchEnsurer); ok {
		return batcher.EnsureDefaults(ctx, orgIDs)
	}
	for _, orgID := range orgIDs {
		quota, err := h.Quotas.EnsureDefault(ctx, orgID)
		if err != nil {
			return nil, err
		}
		out[orgID] = quota
	}
	return out, nil
}

func (h *AdminHandlers) collectOrgUsages(ctx context.Context, orgIDs []string) (map[string]*orgUsageResp, error) {
	out := make(map[string]*orgUsageResp, len(orgIDs))
	for _, orgID := range orgIDs {
		out[orgID] = &orgUsageResp{}
	}
	if h.OrgRepo == nil || h.Lakes == nil || h.Nodes == nil || h.APIKeys == nil || h.Attachments == nil {
		return out, nil
	}
	if members, ok := h.OrgRepo.(adminMemberBatchCounter); ok {
		counts, err := members.CountMembersByOrgIDs(ctx, orgIDs)
		if err != nil {
			return nil, err
		}
		for orgID, n := range counts {
			out[orgID].MembersUsed = n
		}
	} else {
		for _, orgID := range orgIDs {
			members, err := h.OrgRepo.ListMembers(ctx, orgID)
			if err != nil {
				return nil, err
			}
			out[orgID].MembersUsed = int64(len(members))
		}
	}
	if lakes, ok := h.Lakes.(adminLakeBatchCounter); ok {
		counts, err := lakes.CountLakesByOrgIDs(ctx, orgIDs)
		if err != nil {
			return nil, err
		}
		for orgID, n := range counts {
			out[orgID].LakesUsed = n
		}
	} else {
		for _, orgID := range orgIDs {
			lakes, err := h.Lakes.ListLakesByOrg(ctx, nil, orgID, nil)
			if err != nil {
				return nil, err
			}
			out[orgID].LakesUsed = int64(len(lakes))
		}
	}
	if nodes, ok := h.Nodes.(adminNodeBatchCounter); ok {
		counts, err := nodes.CountByOrgIDs(ctx, orgIDs)
		if err != nil {
			return nil, err
		}
		for orgID, n := range counts {
			out[orgID].NodesUsed = n
		}
	} else {
		for _, orgID := range orgIDs {
			n, err := h.Nodes.CountByOrg(ctx, orgID)
			if err != nil {
				return nil, err
			}
			out[orgID].NodesUsed = n
		}
	}
	if attachments, ok := h.Attachments.(adminAttachmentBatchUsageReader); ok {
		counts, err := attachments.CountByOrgIDs(ctx, orgIDs)
		if err != nil {
			return nil, err
		}
		sizes, err := attachments.SumSizeByOrgIDs(ctx, orgIDs)
		if err != nil {
			return nil, err
		}
		for orgID, n := range counts {
			out[orgID].AttachmentsUsed = n
		}
		for orgID, n := range sizes {
			out[orgID].StorageMBUsed = bytesToQuotaMB(n)
		}
	} else {
		for _, orgID := range orgIDs {
			attachmentsUsed, err := h.Attachments.CountByOrg(ctx, orgID)
			if err != nil {
				return nil, err
			}
			storageBytes, err := h.Attachments.SumSizeByOrg(ctx, orgID)
			if err != nil {
				return nil, err
			}
			out[orgID].AttachmentsUsed = attachmentsUsed
			out[orgID].StorageMBUsed = bytesToQuotaMB(storageBytes)
		}
	}
	if keys, ok := h.APIKeys.(adminAPIKeyBatchCounter); ok {
		counts, err := keys.CountByOrgIDs(ctx, orgIDs)
		if err != nil {
			return nil, err
		}
		for orgID, n := range counts {
			out[orgID].APIKeysUsed = n
		}
	} else {
		for _, orgID := range orgIDs {
			n, err := h.APIKeys.CountByOrg(ctx, orgID)
			if err != nil {
				return nil, err
			}
			out[orgID].APIKeysUsed = n
		}
	}
	return out, nil
}

func (h *AdminHandlers) loadRecentQuotaAudits(ctx context.Context, orgIDs []string, limit int) (map[string][]*domain.AuditLog, error) {
	out := make(map[string][]*domain.AuditLog, len(orgIDs))
	if h.AuditLogs == nil || limit <= 0 {
		return out, nil
	}
	if batcher, ok := h.AuditLogs.(adminAuditBatchLister); ok {
		return batcher.ListLatestByResources(ctx, "org_quota", orgIDs, limit)
	}
	for _, orgID := range orgIDs {
		logs, err := h.AuditLogs.ListByResource(ctx, "org_quota", orgID, limit)
		if err != nil {
			return nil, err
		}
		out[orgID] = logs
	}
	return out, nil
}

func auditLogsToResp(logs []*domain.AuditLog) []orgAuditLogResp {
	if len(logs) == 0 {
		return nil
	}
	out := make([]orgAuditLogResp, 0, len(logs))
	for _, log := range logs {
		out = append(out, orgAuditLogResp{
			ID:           log.ID,
			ActorID:      log.ActorID,
			Action:       log.Action,
			ResourceType: log.ResourceType,
			ResourceID:   log.ResourceID,
			Detail:       log.Detail,
			CreatedAt:    log.CreatedAt,
		})
	}
	return out
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
