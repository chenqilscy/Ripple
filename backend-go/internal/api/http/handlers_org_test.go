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
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

type fakeUserRepo struct {
	user *domain.User
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
