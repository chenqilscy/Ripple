package httpapi

import (
	"context"
	"errors"
	"testing"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

type fakeAttachmentRepo struct {
	count int64
	size  int64
}

func (f *fakeAttachmentRepo) Insert(context.Context, *store.Attachment) error { return nil }
func (f *fakeAttachmentRepo) GetByID(context.Context, string) (*store.Attachment, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeAttachmentRepo) GetBySHA(context.Context, string, string) (*store.Attachment, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeAttachmentRepo) ListByNode(context.Context, string) ([]store.Attachment, error) {
	return nil, nil
}
func (f *fakeAttachmentRepo) Delete(context.Context, string) error { return nil }
func (f *fakeAttachmentRepo) CountByOrg(context.Context, string) (int64, error) {
	return f.count, nil
}
func (f *fakeAttachmentRepo) SumSizeByOrg(context.Context, string) (int64, error) {
	return f.size, nil
}

func TestAttachmentHandlers_CheckAttachmentQuota_RejectsCountExceeded(t *testing.T) {
	h := &AttachmentHandlers{
		Repo: &fakeAttachmentRepo{count: 10},
		Orgs: service.NewOrgService(fakeAPIKeyOrgRepo{}).WithQuotaRepository(fakeAPIKeyQuotaRepo{}),
	}

	err := h.checkAttachmentQuota(context.Background(), "org-1", 1)
	if !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatalf("want ErrQuotaExceeded, got %v", err)
	}
}

func TestBytesToQuotaMB_RoundsUp(t *testing.T) {
	if got := bytesToQuotaMB(1024*1024 + 1); got != 2 {
		t.Fatalf("want 2, got %d", got)
	}
}
