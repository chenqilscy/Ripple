package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
)

type auditHandlerRepo struct{}

func (auditHandlerRepo) Write(context.Context, *domain.AuditLog) error            { return nil }
func (auditHandlerRepo) PruneOlderThan(context.Context, time.Time) (int64, error) { return 0, nil }
func (auditHandlerRepo) ListByResource(context.Context, string, string, int) ([]*domain.AuditLog, error) {
	return []*domain.AuditLog{{
		ID:           "audit-1",
		ActorID:      "u-admin",
		Action:       domain.AuditGraylistUpsert,
		ResourceType: "graylist",
		ResourceID:   "entry-1",
		Detail:       map[string]any{"email": "beta@test.local"},
		CreatedAt:    time.Now().UTC(),
	}}, nil
}

func auditReq(email string) *http.Request {
	return auditReqFor(email, "graylist", "entry-1")
}

func auditReqFor(email, resourceType, resourceID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit_logs?resource_type="+resourceType+"&resource_id="+resourceID, nil)
	ctx := context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u-admin", Email: email})
	return req.WithContext(ctx)
}

func TestAuditLogHandlers_List_AllowsGraylistForPlatformAdmin(t *testing.T) {
	h := &AuditLogHandlers{Repo: auditHandlerRepo{}, AdminEmails: map[string]struct{}{"admin@test.local": {}}}
	rr := httptest.NewRecorder()

	h.List(rr, auditReq("admin@test.local"))

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAuditLogHandlers_List_RejectsGraylistForNonAdmin(t *testing.T) {
	h := &AuditLogHandlers{Repo: auditHandlerRepo{}, AdminEmails: map[string]struct{}{"admin@test.local": {}}}
	rr := httptest.NewRecorder()

	h.List(rr, auditReq("member@test.local"))

	if rr.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAuditLogHandlers_List_AllowsPlatformAdminAuditForPlatformAdmin(t *testing.T) {
	h := &AuditLogHandlers{Repo: auditHandlerRepo{}, AdminEmails: map[string]struct{}{"admin@test.local": {}}}
	rr := httptest.NewRecorder()

	h.List(rr, auditReqFor("admin@test.local", "platform_admin", "u-target"))

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
