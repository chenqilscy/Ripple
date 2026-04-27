package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/go-chi/chi/v5"
)

type fakeAPIKeyRepo struct {
	revokeCalls int
}

func (f *fakeAPIKeyRepo) Create(context.Context, *domain.APIKey) error { return nil }
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
