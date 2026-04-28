package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

type fakeUserRepo struct {
	user *domain.User
}

type handlerOrgRepo struct {
	roles   map[string]domain.OrgRole
	members []domain.OrgMember
	orgs    []domain.Organization
}

func (h *handlerOrgRepo) Create(_ context.Context, _ *domain.Organization) error { return nil }
func (h *handlerOrgRepo) GetByID(_ context.Context, _ string) (*domain.Organization, error) {
	return &domain.Organization{ID: "org-1", Name: "org", Slug: "org-1", OwnerID: "u-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}
func (h *handlerOrgRepo) GetBySlug(_ context.Context, _ string) (*domain.Organization, error) {
	return h.GetByID(context.Background(), "org-1")
}
func (h *handlerOrgRepo) ListByUser(_ context.Context, _ string) ([]domain.Organization, error) {
	out := make([]domain.Organization, len(h.orgs))
	copy(out, h.orgs)
	return out, nil
}
func (h *handlerOrgRepo) AddMember(_ context.Context, _ *domain.OrgMember) error { return nil }
func (h *handlerOrgRepo) GetMemberRole(_ context.Context, _ string, userID string) (domain.OrgRole, error) {
	role, ok := h.roles[userID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return role, nil
}
func (h *handlerOrgRepo) ListMembers(_ context.Context, _ string) ([]domain.OrgMember, error) {
	out := make([]domain.OrgMember, len(h.members))
	copy(out, h.members)
	return out, nil
}
func (h *handlerOrgRepo) UpdateMemberRole(_ context.Context, _, _ string, _ domain.OrgRole) error {
	return nil
}
func (h *handlerOrgRepo) RemoveMember(_ context.Context, _, _ string) error    { return nil }
func (h *handlerOrgRepo) CountOwners(_ context.Context, _ string) (int, error)          { return 1, nil }
func (h *handlerOrgRepo) CountMembersByOrg(_ context.Context, _ string) (int64, error)  { return 0, nil }

type handlerQuotaRepo struct {
	quota *domain.OrgQuota
}

type fakeOrgLakeLister struct{ lakes []domain.Lake }

func (f *fakeOrgLakeLister) ListLakesByOrg(context.Context, *domain.User, string, *service.OrgService) ([]domain.Lake, error) {
	out := make([]domain.Lake, len(f.lakes))
	copy(out, f.lakes)
	return out, nil
}

func (f *fakeOrgLakeLister) CountLakesByOrgIDs(_ context.Context, orgIDs []string) (map[string]int64, error) {
	out := make(map[string]int64, len(orgIDs))
	for _, lake := range f.lakes {
		out[lake.OrgID]++
	}
	return out, nil
}

type fakeOrgNodeCounter struct{ used int64 }

func (f *fakeOrgNodeCounter) CountByOrg(context.Context, string) (int64, error) { return f.used, nil }
func (f *fakeOrgNodeCounter) CountByOrgIDs(_ context.Context, orgIDs []string) (map[string]int64, error) {
	out := make(map[string]int64, len(orgIDs))
	for _, orgID := range orgIDs {
		out[orgID] = f.used
	}
	return out, nil
}

type fakeOrgAttachmentUsage struct {
	count int64
	size  int64
}

func (f *fakeOrgAttachmentUsage) CountByOrg(context.Context, string) (int64, error) {
	return f.count, nil
}
func (f *fakeOrgAttachmentUsage) CountByOrgIDs(_ context.Context, orgIDs []string) (map[string]int64, error) {
	out := make(map[string]int64, len(orgIDs))
	for _, orgID := range orgIDs {
		out[orgID] = f.count
	}
	return out, nil
}
func (f *fakeOrgAttachmentUsage) SumSizeByOrg(context.Context, string) (int64, error) {
	return f.size, nil
}
func (f *fakeOrgAttachmentUsage) SumSizeByOrgIDs(_ context.Context, orgIDs []string) (map[string]int64, error) {
	out := make(map[string]int64, len(orgIDs))
	for _, orgID := range orgIDs {
		out[orgID] = f.size
	}
	return out, nil
}

type fakeOrgAPIKeyCounter struct{ used int64 }

func (f *fakeOrgAPIKeyCounter) CountByOrg(context.Context, string) (int64, error) { return f.used, nil }
func (f *fakeOrgAPIKeyCounter) CountByOrgIDs(_ context.Context, orgIDs []string) (map[string]int64, error) {
	out := make(map[string]int64, len(orgIDs))
	for _, orgID := range orgIDs {
		out[orgID] = f.used
	}
	return out, nil
}

type fakeOrgAuditLogRepo struct{ logs []*domain.AuditLog }

func (f *fakeOrgAuditLogRepo) Write(context.Context, *domain.AuditLog) error { return nil }
func (f *fakeOrgAuditLogRepo) ListByResource(context.Context, string, string, int) ([]*domain.AuditLog, error) {
	out := make([]*domain.AuditLog, len(f.logs))
	copy(out, f.logs)
	return out, nil
}
func (f *fakeOrgAuditLogRepo) PruneOlderThan(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (h *handlerQuotaRepo) EnsureDefault(_ context.Context, orgID string) (*domain.OrgQuota, error) {
	if h.quota == nil {
		h.quota = domain.DefaultOrgQuota(orgID)
	}
	q := *h.quota
	return &q, nil
}

func (h *handlerQuotaRepo) GetByOrgID(ctx context.Context, orgID string) (*domain.OrgQuota, error) {
	return h.EnsureDefault(ctx, orgID)
}

func (h *handlerQuotaRepo) Update(_ context.Context, quota *domain.OrgQuota) error {
	q := *quota
	h.quota = &q
	return nil
}

func (f *fakeUserRepo) Create(_ context.Context, _ *domain.User) error { return nil }
func (f *fakeUserRepo) GetByID(_ context.Context, _ string) (*domain.User, error) {
	if f.user == nil {
		return nil, domain.ErrNotFound
	}
	return f.user, nil
}
func (f *fakeUserRepo) GetByEmail(_ context.Context, _ string) (*domain.User, error) {
	if f.user == nil {
		return nil, domain.ErrNotFound
	}
	return f.user, nil
}

var _ store.UserRepository = (*fakeUserRepo)(nil)

func authReq(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u-1"})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "org-1")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

func readErr(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var out map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode err body: %v", err)
	}
	return out["error"]
}

func TestOrgHandlers_AddMember_RejectsEmptyRole(t *testing.T) {
	h := &OrgHandlers{}
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/organizations/org-1/members", `{"user_id":"u-2","role":""}`)

	h.AddMember(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rr.Code)
	}
	if msg := readErr(t, rr); msg != "role required" {
		t.Fatalf("want role required, got %q", msg)
	}
}

func TestOrgHandlers_AddMember_RejectsOwnerRole(t *testing.T) {
	h := &OrgHandlers{}
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPost, "/api/v1/organizations/org-1/members", `{"user_id":"u-2","role":"OWNER"}`)

	h.AddMember(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rr.Code)
	}
	if msg := readErr(t, rr); msg != "role must be ADMIN or MEMBER" {
		t.Fatalf("unexpected error: %q", msg)
	}
}

func TestOrgHandlers_AddMemberByEmail_RejectsInvalidRole(t *testing.T) {
	h := &OrgHandlers{Users: &fakeUserRepo{user: &domain.User{ID: "u-2", Email: "u2@test.local", CreatedAt: time.Now(), UpdatedAt: time.Now()}}}

	t.Run("empty role", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := authReq(http.MethodPost, "/api/v1/organizations/org-1/members/by_email", `{"email":"u2@test.local","role":""}`)
		h.AddMemberByEmail(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", rr.Code)
		}
		if msg := readErr(t, rr); msg != "role required" {
			t.Fatalf("unexpected error: %q", msg)
		}
	})

	t.Run("owner role", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := authReq(http.MethodPost, "/api/v1/organizations/org-1/members/by_email", `{"email":"u2@test.local","role":"OWNER"}`)
		h.AddMemberByEmail(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", rr.Code)
		}
		if msg := readErr(t, rr); msg != "role must be ADMIN or MEMBER" {
			t.Fatalf("unexpected error: %q", msg)
		}
	})
}

func TestOrgHandlers_GetOrgQuota(t *testing.T) {
	svc := service.NewOrgService(&handlerOrgRepo{roles: map[string]domain.OrgRole{"u-1": domain.OrgRoleMember}}).
		WithQuotaRepository(&handlerQuotaRepo{})
	h := &OrgHandlers{Orgs: svc}
	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/organizations/org-1/quota", "")

	h.GetOrgQuota(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var out orgQuotaResp
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode quota: %v", err)
	}
	if out.OrgID != "org-1" || out.MaxMembers != 20 {
		t.Fatalf("unexpected quota response: %+v", out)
	}
}

func TestOrgHandlers_GetOrgQuota_IncludesUsage(t *testing.T) {
	svc := service.NewOrgService(&handlerOrgRepo{
		roles: map[string]domain.OrgRole{"u-1": domain.OrgRoleAdmin},
		members: []domain.OrgMember{
			{OrgID: "org-1", UserID: "u-1", Role: domain.OrgRoleAdmin},
			{OrgID: "org-1", UserID: "u-2", Role: domain.OrgRoleMember},
		},
	}).WithQuotaRepository(&handlerQuotaRepo{})
	h := &OrgHandlers{
		Orgs:        svc,
		Lakes:       &fakeOrgLakeLister{lakes: []domain.Lake{{ID: "lake-1"}, {ID: "lake-2"}}},
		Nodes:       &fakeOrgNodeCounter{used: 7},
		APIKeys:     &fakeOrgAPIKeyCounter{used: 3},
		Attachments: &fakeOrgAttachmentUsage{count: 5, size: 3*1024*1024 + 1},
	}
	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/organizations/org-1/quota", "")

	h.GetOrgQuota(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var out orgQuotaResp
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode quota with usage: %v", err)
	}
	if out.Usage == nil {
		t.Fatal("want usage in quota response")
	}
	if out.Usage.MembersUsed != 2 || out.Usage.LakesUsed != 2 || out.Usage.NodesUsed != 7 || out.Usage.AttachmentsUsed != 5 || out.Usage.APIKeysUsed != 3 || out.Usage.StorageMBUsed != 4 {
		t.Fatalf("unexpected usage response: %+v", out.Usage)
	}
}

func TestOrgHandlers_GetOrgOverview(t *testing.T) {
	now := time.Now().UTC()
	svc := service.NewOrgService(&handlerOrgRepo{
		roles: map[string]domain.OrgRole{"u-1": domain.OrgRoleAdmin},
		members: []domain.OrgMember{
			{OrgID: "org-1", UserID: "u-1", Role: domain.OrgRoleAdmin},
			{OrgID: "org-1", UserID: "u-2", Role: domain.OrgRoleMember},
		},
	}).WithQuotaRepository(&handlerQuotaRepo{})
	h := &OrgHandlers{
		Orgs:        svc,
		Lakes:       &fakeOrgLakeLister{lakes: []domain.Lake{{ID: "lake-1"}}},
		Nodes:       &fakeOrgNodeCounter{used: 9},
		APIKeys:     &fakeOrgAPIKeyCounter{used: 2},
		Attachments: &fakeOrgAttachmentUsage{count: 4, size: 2 * 1024 * 1024},
		AuditLogs: &fakeOrgAuditLogRepo{logs: []*domain.AuditLog{{
			ID:           "audit-1",
			ActorID:      "u-1",
			Action:       domain.AuditOrgQuotaUpdate,
			ResourceType: "org_quota",
			ResourceID:   "org-1",
			Detail:       map[string]any{"after": map[string]any{"max_members": 30}},
			CreatedAt:    now,
		}}},
	}
	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/organizations/org-1/overview", "")

	h.GetOrgOverview(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var out orgOverviewResp
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode org overview: %v", err)
	}
	if out.Organization.ID != "org-1" {
		t.Fatalf("unexpected organization: %+v", out.Organization)
	}
	if out.Quota.Usage == nil || out.Quota.Usage.MembersUsed != 2 || out.Quota.Usage.NodesUsed != 9 || out.Quota.Usage.StorageMBUsed != 2 {
		t.Fatalf("unexpected overview quota usage: %+v", out.Quota.Usage)
	}
	if len(out.RecentQuotaAudits) != 1 || out.RecentQuotaAudits[0].ID != "audit-1" || out.RecentQuotaAudits[0].Action != domain.AuditOrgQuotaUpdate {
		t.Fatalf("unexpected recent audits: %+v", out.RecentQuotaAudits)
	}
}

func TestOrgHandlers_ListOrgOverviews(t *testing.T) {
	now := time.Now().UTC()
	svc := service.NewOrgService(&handlerOrgRepo{
		orgs: []domain.Organization{{
			ID:        "org-1",
			Name:      "Org One",
			Slug:      "org-one",
			OwnerID:   "u-1",
			CreatedAt: now,
			UpdatedAt: now,
		}},
		roles: map[string]domain.OrgRole{"u-1": domain.OrgRoleAdmin},
		members: []domain.OrgMember{
			{OrgID: "org-1", UserID: "u-1", Role: domain.OrgRoleAdmin},
			{OrgID: "org-1", UserID: "u-2", Role: domain.OrgRoleMember},
		},
	}).WithQuotaRepository(&handlerQuotaRepo{})
	h := &OrgHandlers{
		Orgs:        svc,
		Lakes:       &fakeOrgLakeLister{lakes: []domain.Lake{{ID: "lake-1"}}},
		Nodes:       &fakeOrgNodeCounter{used: 4},
		APIKeys:     &fakeOrgAPIKeyCounter{used: 1},
		Attachments: &fakeOrgAttachmentUsage{count: 2, size: 1 * 1024 * 1024},
		AuditLogs: &fakeOrgAuditLogRepo{logs: []*domain.AuditLog{{
			ID:           "audit-1",
			ActorID:      "u-1",
			Action:       domain.AuditOrgQuotaUpdate,
			ResourceType: "org_quota",
			ResourceID:   "org-1",
			CreatedAt:    now,
		}}},
	}
	rr := httptest.NewRecorder()
	req := authReq(http.MethodGet, "/api/v1/organizations/overview", "")

	h.ListOrgOverviews(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Organizations []orgOverviewResp `json:"organizations"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode org overview list: %v", err)
	}
	if len(out.Organizations) != 1 {
		t.Fatalf("want 1 organization, got %d", len(out.Organizations))
	}
	if out.Organizations[0].Organization.ID != "org-1" {
		t.Fatalf("unexpected organization: %+v", out.Organizations[0].Organization)
	}
	if out.Organizations[0].Quota.Usage == nil || out.Organizations[0].Quota.Usage.MembersUsed != 2 || out.Organizations[0].Quota.Usage.LakesUsed != 1 {
		t.Fatalf("unexpected overview list usage: %+v", out.Organizations[0].Quota.Usage)
	}
	if len(out.Organizations[0].RecentQuotaAudits) != 1 || out.Organizations[0].RecentQuotaAudits[0].ID != "audit-1" {
		t.Fatalf("unexpected overview list audits: %+v", out.Organizations[0].RecentQuotaAudits)
	}
}

func TestOrgHandlers_UpdateOrgQuota(t *testing.T) {
	quotas := &handlerQuotaRepo{}
	svc := service.NewOrgService(&handlerOrgRepo{roles: map[string]domain.OrgRole{"u-1": domain.OrgRoleAdmin}}).
		WithQuotaRepository(quotas)
	h := &OrgHandlers{Orgs: svc}
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPatch, "/api/v1/organizations/org-1/quota", `{"max_members":30,"max_api_keys":3}`)

	h.UpdateOrgQuota(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if quotas.quota.MaxMembers != 30 || quotas.quota.MaxAPIKeys != 3 {
		t.Fatalf("quota not updated: %+v", quotas.quota)
	}
}

func TestOrgHandlers_UpdateOrgQuota_RejectsMember(t *testing.T) {
	svc := service.NewOrgService(&handlerOrgRepo{roles: map[string]domain.OrgRole{"u-1": domain.OrgRoleMember}}).
		WithQuotaRepository(&handlerQuotaRepo{})
	h := &OrgHandlers{Orgs: svc}
	rr := httptest.NewRecorder()
	req := authReq(http.MethodPatch, "/api/v1/organizations/org-1/quota", `{"max_members":30}`)

	h.UpdateOrgQuota(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rr.Code)
	}
}
