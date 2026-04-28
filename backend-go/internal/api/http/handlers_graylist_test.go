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

type fakeGraylistRepo struct {
	entries []domain.GraylistEntry
}

func (f *fakeGraylistRepo) List(context.Context, int) ([]domain.GraylistEntry, error) {
	out := make([]domain.GraylistEntry, len(f.entries))
	copy(out, f.entries)
	return out, nil
}

func (f *fakeGraylistRepo) Upsert(_ context.Context, entry *domain.GraylistEntry) (*domain.GraylistEntry, error) {
	for i := range f.entries {
		if f.entries[i].Email == entry.Email {
			f.entries[i].Note = entry.Note
			f.entries[i].CreatedBy = entry.CreatedBy
			f.entries[i].CreatedAt = entry.CreatedAt
			copyEntry := f.entries[i]
			return &copyEntry, nil
		}
	}
	copyEntry := *entry
	f.entries = append(f.entries, copyEntry)
	return &copyEntry, nil
}

func (f *fakeGraylistRepo) Delete(_ context.Context, id string) error {
	for i := range f.entries {
		if f.entries[i].ID == id {
			f.entries = append(f.entries[:i], f.entries[i+1:]...)
			return nil
		}
	}
	return domain.ErrNotFound
}

func (f *fakeGraylistRepo) IsAllowedEmail(context.Context, string) (bool, error) { return false, nil }

var _ store.GraylistRepository = (*fakeGraylistRepo)(nil)

func graylistReq(method, target, body, email string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u-admin", Email: email})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "entry-1")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

func TestGraylistHandlers_List(t *testing.T) {
	h := &GraylistHandlers{
		Repo: &fakeGraylistRepo{entries: []domain.GraylistEntry{{
			ID:        "entry-1",
			Email:     "beta@test.local",
			Note:      "founder",
			CreatedBy: "u-admin",
			CreatedAt: time.Now().UTC(),
		}}},
		AdminEmails: map[string]struct{}{"admin@test.local": {}},
	}
	rr := httptest.NewRecorder()

	h.List(rr, graylistReq(http.MethodGet, "/api/v1/admin/graylist", "", "admin@test.local"))

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Entries []graylistEntryResp `json:"entries"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode entries: %v", err)
	}
	if len(out.Entries) != 1 || out.Entries[0].Email != "beta@test.local" {
		t.Fatalf("unexpected entries: %+v", out.Entries)
	}
}

func TestGraylistHandlers_UpsertAndDelete(t *testing.T) {
	repo := &fakeGraylistRepo{}
	h := &GraylistHandlers{Repo: repo, AdminEmails: map[string]struct{}{"admin@test.local": {}}}
	rr := httptest.NewRecorder()

	h.Upsert(rr, graylistReq(http.MethodPost, "/api/v1/admin/graylist", `{"email":"beta@test.local","note":"wave 1"}`, "admin@test.local"))

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(repo.entries) != 1 || repo.entries[0].Email != "beta@test.local" {
		t.Fatalf("unexpected repo entries: %+v", repo.entries)
	}

	deleteReq := graylistReq(http.MethodDelete, "/api/v1/admin/graylist/"+repo.entries[0].ID, "", "admin@test.local")
	rctx := chi.RouteContext(deleteReq.Context())
	rctx.URLParams = chi.RouteParams{}
	rctx.URLParams.Add("id", repo.entries[0].ID)
	h.Delete(httptest.NewRecorder(), deleteReq)
	if len(repo.entries) != 0 {
		t.Fatalf("entry not deleted: %+v", repo.entries)
	}
}

func TestGraylistHandlers_RejectsNonAdmin(t *testing.T) {
	h := &GraylistHandlers{Repo: &fakeGraylistRepo{}, AdminEmails: map[string]struct{}{"admin@test.local": {}}}
	rr := httptest.NewRecorder()

	h.List(rr, graylistReq(http.MethodGet, "/api/v1/admin/graylist", "", "member@test.local"))

	if rr.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rr.Code)
	}
}