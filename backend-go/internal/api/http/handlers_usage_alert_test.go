package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

// fakeUsageAlertRepo 测试用假仓库。
type fakeUsageAlertRepo struct {
	alert *store.UsageAlert
}

func (f *fakeUsageAlertRepo) GetByOrgID(ctx context.Context, orgID string) (*store.UsageAlert, error) {
	if f.alert != nil && f.alert.OrgID == orgID {
		return f.alert, nil
	}
	return nil, nil // 未设置
}

func (f *fakeUsageAlertRepo) Upsert(ctx context.Context, orgID string, threshold int, enabled bool) (*store.UsageAlert, error) {
	f.alert = &store.UsageAlert{
		ID:               "alert-1",
		OrgID:            orgID,
		ThresholdPercent: threshold,
		Enabled:          enabled,
		CreatedAt:        "2024-01-01T00:00:00Z",
		UpdatedAt:        "2024-01-01T00:00:00Z",
	}
	return f.alert, nil
}

// fakeAlertOrgService 测试用组织服务。
type fakeAlertOrgService struct {
	roles map[string]domain.OrgRole // userID -> role
}

func (f *fakeAlertOrgService) IsMember(ctx context.Context, userID, orgID string) (bool, error) {
	_, ok := f.roles[userID]
	return ok, nil
}

func (f *fakeAlertOrgService) GetMemberRole(ctx context.Context, orgID, userID string) (domain.OrgRole, error) {
	role, ok := f.roles[userID]
	if !ok {
		return "", nil
	}
	return role, nil
}

func usageAlertRequest(method, body, orgID string) *http.Request {
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", orgID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

func TestUsageAlertGetDefault(t *testing.T) {
	repo := &fakeUsageAlertRepo{}
	svc := service.NewUsageAlertService(repo)
	h := &UsageAlertHandlers{Svc: svc, Orgs: &fakeAlertOrgService{
		roles: map[string]domain.OrgRole{"u-1": domain.OrgRoleMember},
	}}

	req := usageAlertRequest(http.MethodGet, "", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u-1"}))
	rr := httptest.NewRecorder()
	h.GetUsageAlert(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp usageAlertResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.OrgID != "org-1" {
		t.Errorf("org_id: expected org-1, got %s", resp.OrgID)
	}
	if resp.ThresholdPercent != 80 {
		t.Errorf("threshold_percent: expected 80 (default), got %d", resp.ThresholdPercent)
	}
	if resp.Enabled {
		t.Errorf("enabled: expected false (default), got true")
	}
}

func TestUsageAlertGetCustom(t *testing.T) {
	repo := &fakeUsageAlertRepo{
		alert: &store.UsageAlert{
			ID:               "alert-1",
			OrgID:            "org-1",
			ThresholdPercent: 60,
			Enabled:          true,
		},
	}
	svc := service.NewUsageAlertService(repo)
	h := &UsageAlertHandlers{Svc: svc, Orgs: &fakeAlertOrgService{
		roles: map[string]domain.OrgRole{"u-1": domain.OrgRoleMember},
	}}

	req := usageAlertRequest(http.MethodGet, "", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u-1"}))
	rr := httptest.NewRecorder()
	h.GetUsageAlert(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp usageAlertResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ThresholdPercent != 60 {
		t.Errorf("threshold_percent: expected 60, got %d", resp.ThresholdPercent)
	}
	if !resp.Enabled {
		t.Errorf("enabled: expected true, got false")
	}
}

func TestUsageAlertUpdate(t *testing.T) {
	repo := &fakeUsageAlertRepo{}
	svc := service.NewUsageAlertService(repo)
	h := &UsageAlertHandlers{Svc: svc, Orgs: &fakeAlertOrgService{
		roles: map[string]domain.OrgRole{"u-1": domain.OrgRoleOwner},
	}}

	body := `{"threshold_percent": 70, "enabled": true}`
	req := usageAlertRequest(http.MethodPut, body, "org-1")
	req = req.WithContext(context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u-1"}))
	rr := httptest.NewRecorder()
	h.UpdateUsageAlert(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp usageAlertResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ThresholdPercent != 70 {
		t.Errorf("threshold_percent: expected 70, got %d", resp.ThresholdPercent)
	}
	if !resp.Enabled {
		t.Errorf("enabled: expected true, got false")
	}
}

func TestUsageAlertUpdateOnlyOwner(t *testing.T) {
	repo := &fakeUsageAlertRepo{}
	svc := service.NewUsageAlertService(repo)
	h := &UsageAlertHandlers{Svc: svc, Orgs: &fakeAlertOrgService{
		roles: map[string]domain.OrgRole{"u-1": domain.OrgRoleMember}, // 非 owner
	}}

	body := `{"threshold_percent": 70, "enabled": true}`
	req := usageAlertRequest(http.MethodPut, body, "org-1")
	req = req.WithContext(context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u-1"}))
	rr := httptest.NewRecorder()
	h.UpdateUsageAlert(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUsageAlertUpdateInvalidThreshold(t *testing.T) {
	repo := &fakeUsageAlertRepo{}
	svc := service.NewUsageAlertService(repo)
	h := &UsageAlertHandlers{Svc: svc, Orgs: &fakeAlertOrgService{
		roles: map[string]domain.OrgRole{"u-1": domain.OrgRoleOwner},
	}}

	// 超出范围 0-100
	body := `{"threshold_percent": 150, "enabled": true}`
	req := usageAlertRequest(http.MethodPut, body, "org-1")
	req = req.WithContext(context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u-1"}))
	rr := httptest.NewRecorder()
	h.UpdateUsageAlert(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUsageAlertUnauthorized(t *testing.T) {
	repo := &fakeUsageAlertRepo{}
	svc := service.NewUsageAlertService(repo)
	h := &UsageAlertHandlers{Svc: svc, Orgs: nil}

	req := usageAlertRequest(http.MethodGet, "", "org-1")
	// 不设置用户
	rr := httptest.NewRecorder()
	h.GetUsageAlert(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUsageAlertForbidden(t *testing.T) {
	repo := &fakeUsageAlertRepo{}
	svc := service.NewUsageAlertService(repo)
	h := &UsageAlertHandlers{Svc: svc, Orgs: &fakeAlertOrgService{
		roles: map[string]domain.OrgRole{}, // 用户不在组织中
	}}

	req := usageAlertRequest(http.MethodGet, "", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u-unknown"}))
	rr := httptest.NewRecorder()
	h.GetUsageAlert(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}