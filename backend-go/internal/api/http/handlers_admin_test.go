package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

type adminOrgRepo struct {
	orgs    []domain.Organization
	members map[string][]domain.OrgMember
}

func (r *adminOrgRepo) Create(context.Context, *domain.Organization) error { return nil }
func (r *adminOrgRepo) GetByID(_ context.Context, id string) (*domain.Organization, error) {
	for i := range r.orgs {
		if r.orgs[i].ID == id {
			org := r.orgs[i]
			return &org, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (r *adminOrgRepo) GetBySlug(context.Context, string) (*domain.Organization, error) { return nil, domain.ErrNotFound }
func (r *adminOrgRepo) ListByUser(context.Context, string) ([]domain.Organization, error) { return nil, nil }
func (r *adminOrgRepo) AddMember(context.Context, *domain.OrgMember) error                { return nil }
func (r *adminOrgRepo) GetMemberRole(context.Context, string, string) (domain.OrgRole, error) {
	return domain.OrgRoleAdmin, nil
}
func (r *adminOrgRepo) ListMembers(_ context.Context, orgID string) ([]domain.OrgMember, error) {
	return append([]domain.OrgMember(nil), r.members[orgID]...), nil
}
func (r *adminOrgRepo) UpdateMemberRole(context.Context, string, string, domain.OrgRole) error { return nil }
func (r *adminOrgRepo) RemoveMember(context.Context, string, string) error                       { return nil }
func (r *adminOrgRepo) CountOwners(context.Context, string) (int, error)                         { return 1, nil }
func (r *adminOrgRepo) ListAll(context.Context, int) ([]domain.Organization, error) {
	return append([]domain.Organization(nil), r.orgs...), nil
}
func (r *adminOrgRepo) CountAll(context.Context) (int64, error) { return int64(len(r.orgs)), nil }

var _ store.OrgRepository = (*adminOrgRepo)(nil)

type adminUserRepo struct{ count int64 }

func (r *adminUserRepo) Create(context.Context, *domain.User) error           { return nil }
func (r *adminUserRepo) GetByID(context.Context, string) (*domain.User, error) { return nil, domain.ErrNotFound }
func (r *adminUserRepo) GetByEmail(context.Context, string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (r *adminUserRepo) CountAll(context.Context) (int64, error) { return r.count, nil }

type adminQuotaRepo struct{ quota *domain.OrgQuota }

func (r *adminQuotaRepo) EnsureDefault(context.Context, string) (*domain.OrgQuota, error) { return r.quota, nil }
func (r *adminQuotaRepo) GetByOrgID(context.Context, string) (*domain.OrgQuota, error)     { return r.quota, nil }
func (r *adminQuotaRepo) Update(context.Context, *domain.OrgQuota) error                    { return nil }

type adminGraylistRepo struct{ count int64 }

func (r *adminGraylistRepo) List(context.Context, int) ([]domain.GraylistEntry, error) { return nil, nil }
func (r *adminGraylistRepo) Upsert(context.Context, *domain.GraylistEntry) (*domain.GraylistEntry, error) {
	return nil, nil
}
func (r *adminGraylistRepo) Delete(context.Context, string) error                { return nil }
func (r *adminGraylistRepo) IsAllowedEmail(context.Context, string) (bool, error) { return false, nil }
func (r *adminGraylistRepo) CountAll(context.Context) (int64, error)              { return r.count, nil }

type adminAuditRepo struct{ logs []*domain.AuditLog }

func (r *adminAuditRepo) Write(context.Context, *domain.AuditLog) error { return nil }
func (r *adminAuditRepo) PruneOlderThan(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (r *adminAuditRepo) ListByResource(_ context.Context, resourceType, resourceID string, limit int) ([]*domain.AuditLog, error) {
	out := make([]*domain.AuditLog, 0, limit)
	for _, log := range r.logs {
		if log.ResourceType == resourceType && log.ResourceID == resourceID {
			out = append(out, log)
		}
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func adminReq(email string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/overview", nil)
	ctx := context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u-admin", Email: email})
	return req.WithContext(ctx)
}

func TestAdminHandlers_Overview(t *testing.T) {
	now := time.Now().UTC()
	h := &AdminHandlers{
		OrgRepo: &adminOrgRepo{
			orgs: []domain.Organization{{
				ID: "org-1", Name: "Alpha", Slug: "alpha", OwnerID: "u-1", CreatedAt: now, UpdatedAt: now,
			}},
			members: map[string][]domain.OrgMember{"org-1": {{OrgID: "org-1", UserID: "u-1", Role: domain.OrgRoleOwner}, {OrgID: "org-1", UserID: "u-2", Role: domain.OrgRoleMember}}},
		},
		Quotas: &adminQuotaRepo{quota: &domain.OrgQuota{OrgID: "org-1", MaxMembers: 3, MaxLakes: 5, MaxNodes: 10, MaxAttachments: 2, MaxAPIKeys: 4, MaxStorageMB: 8, CreatedAt: now, UpdatedAt: now}},
		Lakes: &fakeOrgLakeLister{lakes: []domain.Lake{{ID: "lake-1", OrgID: "org-1"}}},
		Users: &adminUserRepo{count: 7},
		Nodes: &fakeOrgNodeCounter{used: 4},
		APIKeys: &fakeOrgAPIKeyCounter{used: 2},
		Attachments: &fakeOrgAttachmentUsage{count: 1, size: 2 * 1024 * 1024},
		AuditLogs: &adminAuditRepo{logs: []*domain.AuditLog{{
			ID: "log-1", ActorID: "u-admin", Action: domain.AuditOrgQuotaUpdate, ResourceType: "org_quota", ResourceID: "org-1", CreatedAt: now,
		}}},
		Graylist: &adminGraylistRepo{count: 3},
		AdminEmails: map[string]struct{}{"admin@test.local": {}},
	}
	rr := httptest.NewRecorder()

	h.Overview(rr, adminReq("admin@test.local"))

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var out adminOverviewResp
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode admin overview: %v", err)
	}
	if out.Stats.OrganizationsCount != 1 || out.Stats.UsersCount != 7 || out.Stats.GraylistEntriesCount != 3 {
		t.Fatalf("unexpected stats: %+v", out.Stats)
	}
	if len(out.Organizations) != 1 {
		t.Fatalf("unexpected organizations: %+v", out.Organizations)
	}
	if out.Organizations[0].Quota.Usage == nil || out.Organizations[0].Quota.Usage.MembersUsed != 2 || out.Organizations[0].Quota.Usage.LakesUsed != 1 {
		t.Fatalf("unexpected usage: %+v", out.Organizations[0].Quota.Usage)
	}
	if len(out.Organizations[0].RecentQuotaAudits) != 1 {
		t.Fatalf("unexpected audits: %+v", out.Organizations[0].RecentQuotaAudits)
	}
}

func TestAdminHandlers_Overview_RejectsNonAdmin(t *testing.T) {
	h := &AdminHandlers{
		OrgRepo:     &adminOrgRepo{},
		Quotas:      &adminQuotaRepo{},
		AdminEmails: map[string]struct{}{"admin@test.local": {}},
	}
	rr := httptest.NewRecorder()

	h.Overview(rr, adminReq("member@test.local"))

	if rr.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", rr.Code, rr.Body.String())
	}
}