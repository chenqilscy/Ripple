package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/go-chi/chi/v5"
)

type platformAdminRepoFake struct {
	admins  map[string]domain.PlatformAdmin
	granted *domain.PlatformAdmin
	revoked string
}

func (f *platformAdminRepoFake) IsActive(ctx context.Context, userID string) (bool, error) {
	_, ok := f.admins[userID]
	return ok, nil
}
func (f *platformAdminRepoFake) GetActive(ctx context.Context, userID string) (*domain.PlatformAdmin, error) {
	admin, ok := f.admins[userID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &admin, nil
}
func (f *platformAdminRepoFake) ListActive(ctx context.Context, limit int) ([]domain.PlatformAdmin, error) {
	out := make([]domain.PlatformAdmin, 0, len(f.admins))
	for _, admin := range f.admins {
		out = append(out, admin)
	}
	return out, nil
}
func (f *platformAdminRepoFake) Grant(ctx context.Context, admin *domain.PlatformAdmin) error {
	cp := *admin
	f.granted = &cp
	if f.admins == nil {
		f.admins = map[string]domain.PlatformAdmin{}
	}
	f.admins[admin.UserID] = cp
	return nil
}
func (f *platformAdminRepoFake) Revoke(ctx context.Context, userID string, revokedAt time.Time) error {
	if _, ok := f.admins[userID]; !ok {
		return domain.ErrNotFound
	}
	delete(f.admins, userID)
	f.revoked = userID
	return nil
}

type platformAdminUserRepoFake struct{ users map[string]*domain.User }

func (f platformAdminUserRepoFake) Create(context.Context, *domain.User) error { return nil }
func (f platformAdminUserRepoFake) GetByID(ctx context.Context, id string) (*domain.User, error) {
	if user, ok := f.users[id]; ok {
		cp := *user
		return &cp, nil
	}
	return nil, domain.ErrNotFound
}
func (f platformAdminUserRepoFake) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	for _, user := range f.users {
		if user.Email == email {
			cp := *user
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (f platformAdminUserRepoFake) CountAll(context.Context) (int64, error) {
	return int64(len(f.users)), nil
}

type platformAdminAuditRepoFake struct{ logs []*domain.AuditLog }

func (f *platformAdminAuditRepoFake) Write(ctx context.Context, log *domain.AuditLog) error {
	cp := *log
	f.logs = append(f.logs, &cp)
	return nil
}
func (f *platformAdminAuditRepoFake) PruneOlderThan(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (f *platformAdminAuditRepoFake) ListByResource(context.Context, string, string, int) ([]*domain.AuditLog, error) {
	return nil, nil
}
func (f *platformAdminAuditRepoFake) ListLatestByResources(context.Context, string, []string, int) (map[string][]*domain.AuditLog, error) {
	return nil, nil
}

func platformAdminReq(method, path, actorID, email string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: actorID, Email: email})
	return req.WithContext(ctx)
}

func TestPlatformAdminHandlers_Grant_AllowsEnvOwner(t *testing.T) {
	repo := &platformAdminRepoFake{}
	audits := &platformAdminAuditRepoFake{}
	h := &PlatformAdminHandlers{
		Repo: repo,
		Users: platformAdminUserRepoFake{users: map[string]*domain.User{
			"u-target": {ID: "u-target", Email: "target@test.local"},
		}},
		AuditLogs:   audits,
		AdminEmails: map[string]struct{}{"owner@test.local": {}},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/platform_admins", strings.NewReader(`{"email":"target@test.local","role":"ADMIN","note":"ops"}`))
	req = req.WithContext(context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u-owner", Email: "owner@test.local"}))
	rr := httptest.NewRecorder()

	h.Grant(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if repo.granted == nil || repo.granted.UserID != "u-target" || repo.granted.Role != domain.PlatformAdminRoleAdmin {
		t.Fatalf("unexpected grant: %+v", repo.granted)
	}
	if len(audits.logs) != 1 || audits.logs[0].Action != domain.AuditPlatformAdminGrant {
		t.Fatalf("unexpected audits: %+v", audits.logs)
	}
}

func TestPlatformAdminHandlers_Grant_RejectsNonOwnerAdmin(t *testing.T) {
	repo := &platformAdminRepoFake{admins: map[string]domain.PlatformAdmin{
		"u-admin": {UserID: "u-admin", Role: domain.PlatformAdminRoleAdmin, CreatedAt: time.Now().UTC()},
	}}
	h := &PlatformAdminHandlers{Repo: repo, Users: platformAdminUserRepoFake{users: map[string]*domain.User{}}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/platform_admins", strings.NewReader(`{"user_id":"u-target"}`))
	req = req.WithContext(context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u-admin", Email: "admin@test.local"}))
	rr := httptest.NewRecorder()

	h.Grant(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPlatformAdminHandlers_Revoke_WritesAudit(t *testing.T) {
	repo := &platformAdminRepoFake{admins: map[string]domain.PlatformAdmin{
		"u-target": {UserID: "u-target", Role: domain.PlatformAdminRoleAdmin, CreatedAt: time.Now().UTC()},
	}}
	audits := &platformAdminAuditRepoFake{}
	h := &PlatformAdminHandlers{Repo: repo, AuditLogs: audits, AdminEmails: map[string]struct{}{"owner@test.local": {}}}
	req := platformAdminReq(http.MethodDelete, "/api/v1/admin/platform_admins/u-target", "u-owner", "owner@test.local")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("user_id", "u-target")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	h.Revoke(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if repo.revoked != "u-target" {
		t.Fatalf("unexpected revoked id: %s", repo.revoked)
	}
	if len(audits.logs) != 1 || audits.logs[0].Action != domain.AuditPlatformAdminRevoke {
		t.Fatalf("unexpected audits: %+v", audits.logs)
	}
}

func TestPlatformAdminHandlers_List_RejectsAPIKey(t *testing.T) {
	h := &PlatformAdminHandlers{Repo: &platformAdminRepoFake{}, AdminEmails: map[string]struct{}{"owner@test.local": {}}}
	req := platformAdminReq(http.MethodGet, "/api/v1/admin/platform_admins", "u-owner", "owner@test.local")
	req = req.WithContext(context.WithValue(req.Context(), ctxAPIKeyKey, &domain.APIKey{ID: "key-1", OwnerID: "u-owner"}))
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", rr.Code, rr.Body.String())
	}
}
