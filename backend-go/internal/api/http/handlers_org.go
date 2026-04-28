package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

// OrgHandlers 组织 HTTP 处理器（P12-C）。
type OrgHandlers struct {
	Orgs        *service.OrgService
	Lakes       orgLakeLister
	Users       store.UserRepository
	Nodes       orgNodeCounter
	APIKeys     apiKeyOrgCounter
	Attachments orgAttachmentUsageReader
	AuditLogs   store.AuditLogRepository
}

type orgLakeLister interface {
	ListLakesByOrg(ctx context.Context, actor *domain.User, orgID string, orgs *service.OrgService) ([]domain.Lake, error)
}

type orgNodeCounter interface {
	CountByOrg(ctx context.Context, orgID string) (int64, error)
}

type orgAttachmentUsageReader interface {
	attachmentOrgCounter
	attachmentOrgSizer
}

type orgResp struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	OwnerID     string    `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toOrgResp(o *domain.Organization) orgResp {
	return orgResp{
		ID: o.ID, Name: o.Name, Slug: o.Slug,
		Description: o.Description, OwnerID: o.OwnerID,
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
	}
}

type orgMemberResp struct {
	OrgID    string    `json:"org_id"`
	UserID   string    `json:"user_id"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type orgQuotaResp struct {
	OrgID          string        `json:"org_id"`
	MaxMembers     int64         `json:"max_members"`
	MaxLakes       int64         `json:"max_lakes"`
	MaxNodes       int64         `json:"max_nodes"`
	MaxAttachments int64         `json:"max_attachments"`
	MaxAPIKeys     int64         `json:"max_api_keys"`
	MaxStorageMB   int64         `json:"max_storage_mb"`
	Usage          *orgUsageResp `json:"usage,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

type orgUsageResp struct {
	MembersUsed     int64 `json:"members_used"`
	LakesUsed       int64 `json:"lakes_used"`
	NodesUsed       int64 `json:"nodes_used"`
	AttachmentsUsed int64 `json:"attachments_used"`
	APIKeysUsed     int64 `json:"api_keys_used"`
	StorageMBUsed   int64 `json:"storage_mb_used"`
}

type orgAuditLogResp struct {
	ID           string         `json:"id"`
	ActorID      string         `json:"actor_id"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	Detail       map[string]any `json:"detail"`
	CreatedAt    time.Time      `json:"created_at"`
}

type orgOverviewResp struct {
	Organization      orgResp           `json:"organization"`
	Quota             orgQuotaResp      `json:"quota"`
	RecentQuotaAudits []orgAuditLogResp `json:"recent_quota_audits,omitempty"`
}

func toOrgQuotaResp(q *domain.OrgQuota) orgQuotaResp {
	return orgQuotaResp{
		OrgID:          q.OrgID,
		MaxMembers:     q.MaxMembers,
		MaxLakes:       q.MaxLakes,
		MaxNodes:       q.MaxNodes,
		MaxAttachments: q.MaxAttachments,
		MaxAPIKeys:     q.MaxAPIKeys,
		MaxStorageMB:   q.MaxStorageMB,
		CreatedAt:      q.CreatedAt,
		UpdatedAt:      q.UpdatedAt,
	}
}

func (h *OrgHandlers) buildOrgQuotaResp(ctx context.Context, actor *domain.User, quota *domain.OrgQuota) (orgQuotaResp, error) {
	resp := toOrgQuotaResp(quota)
	usage, err := h.collectOrgUsage(ctx, actor, quota.OrgID)
	if err != nil {
		return orgQuotaResp{}, err
	}
	resp.Usage = usage
	return resp, nil
}

func (h *OrgHandlers) buildOrgOverviewResp(ctx context.Context, actor *domain.User, org *domain.Organization, auditLimit int) (orgOverviewResp, error) {
	quota, err := h.Orgs.GetQuota(ctx, actor, org.ID)
	if err != nil {
		return orgOverviewResp{}, err
	}
	quotaResp, err := h.buildOrgQuotaResp(ctx, actor, quota)
	if err != nil {
		return orgOverviewResp{}, err
	}
	resp := orgOverviewResp{
		Organization: toOrgResp(org),
		Quota:        quotaResp,
	}
	if h.AuditLogs != nil && auditLimit > 0 {
		logs, err := h.AuditLogs.ListByResource(ctx, "org_quota", org.ID, auditLimit)
		if err != nil {
			return orgOverviewResp{}, err
		}
		resp.RecentQuotaAudits = make([]orgAuditLogResp, 0, len(logs))
		for _, log := range logs {
			resp.RecentQuotaAudits = append(resp.RecentQuotaAudits, orgAuditLogResp{
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
	return resp, nil
}

func (h *OrgHandlers) collectOrgUsage(ctx context.Context, actor *domain.User, orgID string) (*orgUsageResp, error) {
	if h.Orgs == nil || h.Lakes == nil || h.Nodes == nil || h.APIKeys == nil || h.Attachments == nil {
		return nil, nil
	}
	members, err := h.Orgs.ListMembers(ctx, actor, orgID)
	if err != nil {
		return nil, err
	}
	lakes, err := h.Lakes.ListLakesByOrg(ctx, actor, orgID, h.Orgs)
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
	apiKeysUsed, err := h.APIKeys.CountByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	storageBytes, err := h.Attachments.SumSizeByOrg(ctx, orgID)
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

// CreateOrg POST /api/v1/organizations
func (h *OrgHandlers) CreateOrg(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024)
	var body struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	org, err := h.Orgs.CreateOrg(r.Context(), u, service.CreateOrgInput{
		Name:        body.Name,
		Slug:        body.Slug,
		Description: body.Description,
	})
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toOrgResp(org))
}

// ListMyOrgs GET /api/v1/organizations
func (h *OrgHandlers) ListMyOrgs(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	orgs, err := h.Orgs.ListMyOrgs(r.Context(), u)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	out := make([]orgResp, 0, len(orgs))
	for i := range orgs {
		out = append(out, toOrgResp(&orgs[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"organizations": out})
}

// ListOrgOverviews GET /api/v1/organizations/overview
func (h *OrgHandlers) ListOrgOverviews(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	orgs, err := h.Orgs.ListMyOrgs(r.Context(), u)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	out := make([]orgOverviewResp, 0, len(orgs))
	for i := range orgs {
		overview, err := h.buildOrgOverviewResp(r.Context(), u, &orgs[i], 1)
		if err != nil {
			writeError(w, mapDomainError(err), err.Error())
			return
		}
		out = append(out, overview)
	}
	writeJSON(w, http.StatusOK, map[string]any{"organizations": out})
}

// GetOrgOverview GET /api/v1/organizations/{id}/overview
func (h *OrgHandlers) GetOrgOverview(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	org, err := h.Orgs.GetOrg(r.Context(), u, id)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	resp, err := h.buildOrgOverviewResp(r.Context(), u, org, 10)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetOrg GET /api/v1/organizations/{id}
func (h *OrgHandlers) GetOrg(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	org, err := h.Orgs.GetOrg(r.Context(), u, id)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toOrgResp(org))
}

// GetOrgQuota GET /api/v1/organizations/{id}/quota
func (h *OrgHandlers) GetOrgQuota(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	quota, err := h.Orgs.GetQuota(r.Context(), u, id)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	resp, err := h.buildOrgQuotaResp(r.Context(), u, quota)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// UpdateOrgQuota PATCH /api/v1/organizations/{id}/quota
func (h *OrgHandlers) UpdateOrgQuota(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, 2*1024)
	var body struct {
		MaxMembers     *int64 `json:"max_members"`
		MaxLakes       *int64 `json:"max_lakes"`
		MaxNodes       *int64 `json:"max_nodes"`
		MaxAttachments *int64 `json:"max_attachments"`
		MaxAPIKeys     *int64 `json:"max_api_keys"`
		MaxStorageMB   *int64 `json:"max_storage_mb"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	quota, err := h.Orgs.UpdateQuota(r.Context(), u, id, service.UpdateOrgQuotaInput{
		MaxMembers:     body.MaxMembers,
		MaxLakes:       body.MaxLakes,
		MaxNodes:       body.MaxNodes,
		MaxAttachments: body.MaxAttachments,
		MaxAPIKeys:     body.MaxAPIKeys,
		MaxStorageMB:   body.MaxStorageMB,
	})
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	resp, err := h.buildOrgQuotaResp(r.Context(), u, quota)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// ListMembers GET /api/v1/organizations/{id}/members
func (h *OrgHandlers) ListMembers(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	members, err := h.Orgs.ListMembers(r.Context(), u, id)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	out := make([]orgMemberResp, 0, len(members))
	for _, m := range members {
		out = append(out, orgMemberResp{OrgID: m.OrgID, UserID: m.UserID, Role: string(m.Role), JoinedAt: m.JoinedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": out})
}

// AddMember POST /api/v1/organizations/{id}/members
// Body: {"user_id": "...", "role": "ADMIN|MEMBER"}
func (h *OrgHandlers) AddMember(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var body struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id required")
		return
	}
	if body.Role == "" {
		writeError(w, http.StatusBadRequest, "role required")
		return
	}
	if !domain.OrgRole(body.Role).IsValid() || domain.OrgRole(body.Role) == domain.OrgRoleOwner {
		writeError(w, http.StatusBadRequest, "role must be ADMIN or MEMBER")
		return
	}
	if err := h.Orgs.AddMember(r.Context(), u, id, body.UserID, domain.OrgRole(body.Role)); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AddMemberByEmail POST /api/v1/organizations/{id}/members/by_email
// Body: {"email": "user@example.com", "role": "ADMIN|MEMBER"}
// P12-C：按 email 邀请已注册用户加入组织。未注册用户返回 404（不暴露注册状态以外的细节）。
func (h *OrgHandlers) AddMemberByEmail(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.Users == nil {
		writeError(w, http.StatusServiceUnavailable, "users repository not configured")
		return
	}
	id := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Email == "" {
		writeError(w, http.StatusBadRequest, "email required")
		return
	}
	if body.Role == "" {
		writeError(w, http.StatusBadRequest, "role required")
		return
	}
	if !domain.OrgRole(body.Role).IsValid() || domain.OrgRole(body.Role) == domain.OrgRoleOwner {
		writeError(w, http.StatusBadRequest, "role must be ADMIN or MEMBER")
		return
	}
	target, err := h.Users.GetByEmail(r.Context(), body.Email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.Orgs.AddMember(r.Context(), u, id, target.ID, domain.OrgRole(body.Role)); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateMemberRole PATCH /api/v1/organizations/{id}/members/{userId}/role
// Body: {"role": "ADMIN|MEMBER"}
func (h *OrgHandlers) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	orgID := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")
	r.Body = http.MaxBytesReader(w, r.Body, 512)
	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.Orgs.UpdateMemberRole(r.Context(), u, orgID, userID, domain.OrgRole(body.Role)); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveMember DELETE /api/v1/organizations/{id}/members/{userId}
func (h *OrgHandlers) RemoveMember(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	orgID := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")
	if err := h.Orgs.RemoveMember(r.Context(), u, orgID, userID); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListOrgLakes GET /api/v1/organizations/{id}/lakes  P13-A：列出组织下的所有湖。
// 调用者需是该组织成员。
func (h *OrgHandlers) ListOrgLakes(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	orgID := chi.URLParam(r, "id")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "org id required")
		return
	}
	lakes, err := h.Lakes.ListLakesByOrg(r.Context(), u, orgID, h.Orgs)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	type lakeItem struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		IsPublic    bool   `json:"is_public"`
		OwnerID     string `json:"owner_id"`
		OrgID       string `json:"org_id,omitempty"`
	}
	out := make([]lakeItem, 0, len(lakes))
	for _, l := range lakes {
		out = append(out, lakeItem{
			ID: l.ID, Name: l.Name, Description: l.Description,
			IsPublic: l.IsPublic, OwnerID: l.OwnerID, OrgID: l.OrgID,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"lakes": out})
}
