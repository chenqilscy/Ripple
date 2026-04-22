package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/jackc/pgx/v5"
)

// ------- 内存仓库桩（本文件内共享用） -------

type memLakeRepo struct{ data map[string]*domain.Lake }

func newMemLakeRepo() *memLakeRepo { return &memLakeRepo{data: map[string]*domain.Lake{}} }
func (r *memLakeRepo) Create(_ context.Context, l *domain.Lake) error {
	if _, ok := r.data[l.ID]; ok {
		return domain.ErrAlreadyExists
	}
	r.data[l.ID] = l
	return nil
}
func (r *memLakeRepo) GetByID(_ context.Context, id string) (*domain.Lake, error) {
	if l, ok := r.data[id]; ok {
		return l, nil
	}
	return nil, domain.ErrNotFound
}
func (r *memLakeRepo) GetManyByIDs(_ context.Context, ids []string) ([]domain.Lake, error) {
	out := make([]domain.Lake, 0, len(ids))
	for _, id := range ids {
		if l, ok := r.data[id]; ok {
			out = append(out, *l)
		}
	}
	return out, nil
}
func (r *memLakeRepo) UpdateSpaceID(_ context.Context, id, spaceID string) (*domain.Lake, error) {
	l, ok := r.data[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	l.SpaceID = spaceID
	return l, nil
}
func (r *memLakeRepo) UpdateOrgID(_ context.Context, id, orgID string) (*domain.Lake, error) {
	l, ok := r.data[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	l.OrgID = orgID
	return l, nil
}
func (r *memLakeRepo) ListByOrg(_ context.Context, orgID string) ([]domain.Lake, error) {
	out := make([]domain.Lake, 0)
	for _, l := range r.data {
		if l.OrgID == orgID {
			out = append(out, *l)
		}
	}
	return out, nil
}

type memMembershipRepo struct {
	data map[string]map[string]*domain.LakeMembership // userID -> lakeID -> m
}

func newMemMembershipRepo() *memMembershipRepo {
	return &memMembershipRepo{data: map[string]map[string]*domain.LakeMembership{}}
}
func (r *memMembershipRepo) Upsert(_ context.Context, m *domain.LakeMembership) error {
	if _, ok := r.data[m.UserID]; !ok {
		r.data[m.UserID] = map[string]*domain.LakeMembership{}
	}
	r.data[m.UserID][m.LakeID] = m
	return nil
}
func (r *memMembershipRepo) UpsertInTx(_ context.Context, _ pgx.Tx, m *domain.LakeMembership) error {
	return r.Upsert(context.Background(), m)
}
func (r *memMembershipRepo) GetRole(_ context.Context, userID, lakeID string) (domain.Role, error) {
	if u, ok := r.data[userID]; ok {
		if m, ok := u[lakeID]; ok {
			return m.Role, nil
		}
	}
	return "", domain.ErrNotFound
}
func (r *memMembershipRepo) ListLakesByUser(_ context.Context, userID string) ([]string, error) {
	out := []string{}
	if u, ok := r.data[userID]; ok {
		for lid := range u {
			out = append(out, lid)
		}
	}
	return out, nil
}
func (r *memMembershipRepo) ListLakesByUserWithRole(_ context.Context, userID string) ([]domain.LakeMembership, error) {
	out := []domain.LakeMembership{}
	if u, ok := r.data[userID]; ok {
		for _, m := range u {
			out = append(out, *m)
		}
	}
	return out, nil
}
func (r *memMembershipRepo) ListMembers(_ context.Context, lakeID string) ([]domain.LakeMembership, error) {
	out := []domain.LakeMembership{}
	for _, u := range r.data {
		if m, ok := u[lakeID]; ok {
			out = append(out, *m)
		}
	}
	return out, nil
}
func (r *memMembershipRepo) UpdateRole(_ context.Context, userID, lakeID string, role domain.Role) error {
	if u, ok := r.data[userID]; ok {
		if m, ok := u[lakeID]; ok {
			m.Role = role
			return nil
		}
	}
	return domain.ErrNotFound
}

// Outbox 桩：直接记录事件，不真正走 tx。
type memOutboxRepo struct {
	events []store.OutboxEvent
	nextID int64
}

func newMemOutboxRepo() *memOutboxRepo { return &memOutboxRepo{} }
func (r *memOutboxRepo) EnqueueInTx(_ context.Context, _ pgx.Tx, eventType string, payload []byte) error {
	r.nextID++
	r.events = append(r.events, store.OutboxEvent{
		ID: r.nextID, EventType: eventType, Payload: payload, Status: "pending",
	})
	return nil
}
func (r *memOutboxRepo) Dequeue(_ context.Context, _ int) ([]store.OutboxEvent, error) {
	return r.events, nil
}
func (r *memOutboxRepo) MarkDone(_ context.Context, _ int64) error   { return nil }
func (r *memOutboxRepo) MarkFailed(_ context.Context, _ int64, _ string) error { return nil }

// Fake TxRunner：直接调 fn(nil)，等同于不走真正事务。
type fakeTxRunner struct{}

func (fakeTxRunner) RunInTx(ctx context.Context, fn func(tx pgx.Tx) error) error { return fn(nil) }

// ------- 测试用例 -------

func newLakeSvc() (*LakeService, *memLakeRepo, *memMembershipRepo, *memOutboxRepo) {
	lakes := newMemLakeRepo()
	memberships := newMemMembershipRepo()
	outbox := newMemOutboxRepo()
	return NewLakeService(lakes, memberships, outbox, fakeTxRunner{}), lakes, memberships, outbox
}

func TestLake_Create_EnqueuesOutboxAndMembership(t *testing.T) {
	ctx := context.Background()
	svc, _, memberships, outbox := newLakeSvc()
	u := &domain.User{ID: "u-1", Email: "a@b.c", DisplayName: "A", IsActive: true}

	l, err := svc.Create(ctx, u, CreateLakeInput{Name: "My Lake", IsPublic: false})
	if err != nil {
		t.Fatal(err)
	}
	if l.OwnerID != "u-1" || l.ID == "" {
		t.Fatalf("lake fields wrong: %+v", l)
	}

	// membership 已写入
	role, err := memberships.GetRole(ctx, "u-1", l.ID)
	if err != nil || role != domain.RoleOwner {
		t.Fatalf("expected OWNER membership, got %v %v", role, err)
	}
	// outbox 已入队
	if len(outbox.events) != 1 || outbox.events[0].EventType != OutboxEventLakeCreated {
		t.Fatalf("outbox not enqueued: %+v", outbox.events)
	}
	// payload 可反序列化
	var parsed domain.Lake
	if err := json.Unmarshal(outbox.events[0].Payload, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.ID != l.ID {
		t.Fatalf("payload id mismatch")
	}
}

func TestLake_Create_EmptyName(t *testing.T) {
	svc, _, _, _ := newLakeSvc()
	_, err := svc.Create(context.Background(), &domain.User{ID: "u"}, CreateLakeInput{Name: "   "})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestLake_Get_PrivateLake_NonMemberDenied(t *testing.T) {
	ctx := context.Background()
	svc, lakes, _, _ := newLakeSvc()
	// 预埋：u-1 拥有一个私有湖；u-2 尝试读
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u-1", IsPublic: false}
	_, _, err := svc.Get(ctx, &domain.User{ID: "u-2"}, "lake-1")
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestLake_Get_PublicLake_AsObserver(t *testing.T) {
	ctx := context.Background()
	svc, lakes, _, _ := newLakeSvc()
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u-1", IsPublic: true}
	_, role, err := svc.Get(ctx, &domain.User{ID: "u-2"}, "lake-1")
	if err != nil {
		t.Fatal(err)
	}
	if role != domain.RoleObserver {
		t.Fatalf("expected OBSERVER, got %s", role)
	}
}
