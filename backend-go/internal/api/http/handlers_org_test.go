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
	roles map[string]domain.OrgRole
}

func (h *handlerOrgRepo) Create(_ context.Context, _ *domain.Organization) error { return nil }
func (h *handlerOrgRepo) GetByID(_ context.Context, _ string) (*domain.Organization, error) {
	return &domain.Organization{ID: "org-1", Name: "org", Slug: "org-1", OwnerID: "u-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}
func (h *handlerOrgRepo) GetBySlug(_ context.Context, _ string) (*domain.Organization, error) {
	return h.GetByID(context.Background(), "org-1")
}
func (h *handlerOrgRepo) ListByUser(_ context.Context, _ string) ([]domain.Organization, error) {
	return nil, nil
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
	return nil, nil
}
func (h *handlerOrgRepo) UpdateMemberRole(_ context.Context, _, _ string, _ domain.OrgRole) error {
	return nil
}
func (h *handlerOrgRepo) RemoveMember(_ context.Context, _, _ string) error    { return nil }
func (h *handlerOrgRepo) CountOwners(_ context.Context, _ string) (int, error) { return 1, nil }

type handlerQuotaRepo struct {
	quota *domain.OrgQuota
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
