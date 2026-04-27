package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

type fakeAPIKeyRepo struct {
	revokeCalls int
	createCalls int
	orgCount    int64
}

func (f *fakeAPIKeyRepo) Create(context.Context, *domain.APIKey) error {
	f.createCalls++
	return nil
}
func (f *fakeAPIKeyRepo) GetByPrefix(context.Context, string) (*domain.APIKey, error) {
	return nil, nil
}
func (f *fakeAPIKeyRepo) ListByOwner(context.Context, string) ([]*domain.APIKey, error) {
	return nil, nil
}
func (f *fakeAPIKeyRepo) Revoke(context.Context, string, string) error {
	f.revokeCalls++
	return nil
}
func (f *fakeAPIKeyRepo) UpdateLastUsed(context.Context, string, time.Time) error { return nil }
func (f *fakeAPIKeyRepo) CountByOrg(context.Context, string) (int64, error)       { return f.orgCount, nil }

type fakeAPIKeyOrgRepo struct{}

func (f fakeAPIKeyOrgRepo) Create(context.Context, *domain.Organization) error { return nil }
func (f fakeAPIKeyOrgRepo) GetByID(context.Context, string) (*domain.Organization, error) {
	return &domain.Organization{ID: "org-1", Name: "Org", Slug: "org", OwnerID: "owner", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}
func (f fakeAPIKeyOrgRepo) GetBySlug(context.Context, string) (*domain.Organization, error) {
	return f.GetByID(context.Background(), "org-1")
}
func (f fakeAPIKeyOrgRepo) ListByUser(context.Context, string) ([]domain.Organization, error) {
	return nil, nil
}
func (f fakeAPIKeyOrgRepo) AddMember(context.Context, *domain.OrgMember) error { return nil }
func (f fakeAPIKeyOrgRepo) GetMemberRole(context.Context, string, string) (domain.OrgRole, error) {
	return domain.OrgRoleAdmin, nil
}
func (f fakeAPIKeyOrgRepo) ListMembers(context.Context, string) ([]domain.OrgMember, error) {
	return nil, nil
}
func (f fakeAPIKeyOrgRepo) UpdateMemberRole(context.Context, string, string, domain.OrgRole) error {
	return nil
}
func (f fakeAPIKeyOrgRepo) RemoveMember(context.Context, string, string) error { return nil }
func (f fakeAPIKeyOrgRepo) CountOwners(context.Context, string) (int, error)   { return 1, nil }

type fakeAPIKeyQuotaRepo struct{}

func (f fakeAPIKeyQuotaRepo) EnsureDefault(context.Context, string) (*domain.OrgQuota, error) {
	return &domain.OrgQuota{OrgID: "org-1", MaxMembers: 10, MaxLakes: 10, MaxNodes: 10, MaxAttachments: 10, MaxAPIKeys: 1, MaxStorageMB: 10}, nil
}
func (f fakeAPIKeyQuotaRepo) GetByOrgID(ctx context.Context, orgID string) (*domain.OrgQuota, error) {
	return f.EnsureDefault(ctx, orgID)
}
func (f fakeAPIKeyQuotaRepo) Update(context.Context, *domain.OrgQuota) error { return nil }

func TestAPIKeyHandlers_Revoke_RejectsInvalidID(t *testing.T) {
	repo := &fakeAPIKeyRepo{}
	h := &APIKeyHandlers{Repo: repo}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/api_keys/not-a-uuid", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "not-a-uuid")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, ctxUserKey, &domain.User{ID: "u-1"})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.Revoke(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rr.Code)
	}
	if repo.revokeCalls != 0 {
		t.Fatalf("repo revoke should not be called for invalid id")
	}
}

func TestAPIKeyHandlers_Create_RejectsOrgQuotaExceeded(t *testing.T) {
	repo := &fakeAPIKeyRepo{orgCount: 1}
	h := &APIKeyHandlers{
		Repo: repo,
		Orgs: service.NewOrgService(fakeAPIKeyOrgRepo{}).WithQuotaRepository(fakeAPIKeyQuotaRepo{}),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/api_keys", bytes.NewReader([]byte(`{"name":"ops","org_id":"org-1"}`)))
	req = req.WithContext(context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u-1"}))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d body=%s", rr.Code, rr.Body.String())
	}
	if repo.createCalls != 0 {
		t.Fatalf("repo Create should not be called when quota is exceeded")
	}
}
