package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
)

// memInviteRepo 实现 InviteRepository（in-memory）。
type memInviteRepo struct {
	mu    sync.Mutex
	data  map[string]*domain.Invite // by ID
	byTok map[string]string         // token -> ID
}

func newMemInviteRepo() *memInviteRepo {
	return &memInviteRepo{data: map[string]*domain.Invite{}, byTok: map[string]string{}}
}

func (r *memInviteRepo) Create(_ context.Context, inv *domain.Invite) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byTok[inv.Token]; ok {
		return domain.ErrAlreadyExists
	}
	cp := *inv
	r.data[inv.ID] = &cp
	r.byTok[inv.Token] = inv.ID
	return nil
}

func (r *memInviteRepo) GetByID(_ context.Context, id string) (*domain.Invite, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v, ok := r.data[id]; ok {
		cp := *v
		return &cp, nil
	}
	return nil, domain.ErrNotFound
}

func (r *memInviteRepo) GetByToken(_ context.Context, token string) (*domain.Invite, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byTok[token]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *r.data[id]
	return &cp, nil
}

func (r *memInviteRepo) ListByLake(_ context.Context, lakeID string, includeInactive bool) ([]domain.Invite, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.Invite{}
	now := time.Now().UTC()
	for _, v := range r.data {
		if v.LakeID != lakeID {
			continue
		}
		if !includeInactive && !v.IsAlive(now) {
			continue
		}
		out = append(out, *v)
	}
	return out, nil
}

func (r *memInviteRepo) ConsumeByToken(_ context.Context, token string, now time.Time) (*domain.Invite, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byTok[token]
	if !ok {
		return nil, domain.ErrNotFound
	}
	v := r.data[id]
	if v.RevokedAt != nil || now.After(v.ExpiresAt) || v.UsedCount >= v.MaxUses {
		return nil, domain.ErrNotFound
	}
	v.UsedCount++
	cp := *v
	return &cp, nil
}

func (r *memInviteRepo) Revoke(_ context.Context, id string, when time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.data[id]
	if !ok || v.RevokedAt != nil {
		return domain.ErrNotFound
	}
	t := when
	v.RevokedAt = &t
	return nil
}

func newInviteSvc(t *testing.T) (*InviteService, *memLakeRepo, *memMembershipRepo, *memInviteRepo) {
	t.Helper()
	lakes := newMemLakeRepo()
	memberships := newMemMembershipRepo()
	invites := newMemInviteRepo()
	return NewInviteService(invites, memberships, lakes), lakes, memberships, invites
}

// fixture: lake-1 with owner u-owner as NAVIGATOR (for create/list)
func setupInviteFixture(t *testing.T, lakes *memLakeRepo, memberships *memMembershipRepo) *domain.User {
	t.Helper()
	ctx := context.Background()
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", Name: "湖 1", OwnerID: "u-owner"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-owner", LakeID: "lake-1", Role: domain.RoleOwner})
	return &domain.User{ID: "u-owner"}
}

func TestInvite_Create_OK(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newInviteSvc(t)
	actor := setupInviteFixture(t, lakes, memberships)
	inv, err := svc.Create(ctx, actor, CreateInviteInput{
		LakeID: "lake-1", Role: domain.RolePassenger, MaxUses: 5, TTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Token == "" || len(inv.Token) < 40 {
		t.Fatalf("token too short: %q", inv.Token)
	}
	if inv.Role != domain.RolePassenger || inv.MaxUses != 5 {
		t.Fatalf("bad invite: %+v", inv)
	}
}

func TestInvite_Create_RejectsOwnerRole(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newInviteSvc(t)
	actor := setupInviteFixture(t, lakes, memberships)
	_, err := svc.Create(ctx, actor, CreateInviteInput{
		LakeID: "lake-1", Role: domain.RoleOwner, MaxUses: 1, TTL: time.Hour,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestInvite_Create_RejectsBadMaxUses(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newInviteSvc(t)
	actor := setupInviteFixture(t, lakes, memberships)
	_, err := svc.Create(ctx, actor, CreateInviteInput{
		LakeID: "lake-1", Role: domain.RolePassenger, MaxUses: 0, TTL: time.Hour,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestInvite_Create_RejectsNoWritePermission(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newInviteSvc(t)
	setupInviteFixture(t, lakes, memberships)
	// outsider：未加入 lake-1
	_, err := svc.Create(ctx, &domain.User{ID: "outsider"}, CreateInviteInput{
		LakeID: "lake-1", Role: domain.RolePassenger, MaxUses: 1, TTL: time.Hour,
	})
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestInvite_Accept_OK(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newInviteSvc(t)
	actor := setupInviteFixture(t, lakes, memberships)
	inv, _ := svc.Create(ctx, actor, CreateInviteInput{
		LakeID: "lake-1", Role: domain.RolePassenger, MaxUses: 3, TTL: time.Hour,
	})
	res, err := svc.Accept(ctx, &domain.User{ID: "joiner-1"}, inv.Token)
	if err != nil {
		t.Fatal(err)
	}
	if res.LakeID != "lake-1" || res.Role != domain.RolePassenger || res.AlreadyMember {
		t.Fatalf("bad result: %+v", res)
	}
	role, _ := memberships.GetRole(ctx, "joiner-1", "lake-1")
	if role != domain.RolePassenger {
		t.Fatalf("membership not written: %q", role)
	}
}

func TestInvite_Accept_AlreadyMember(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, invites := newInviteSvc(t)
	actor := setupInviteFixture(t, lakes, memberships)
	inv, _ := svc.Create(ctx, actor, CreateInviteInput{
		LakeID: "lake-1", Role: domain.RolePassenger, MaxUses: 1, TTL: time.Hour,
	})
	// owner 接受：已经是 OWNER → 幂等，不消耗
	res, err := svc.Accept(ctx, actor, inv.Token)
	if err != nil {
		t.Fatal(err)
	}
	if !res.AlreadyMember || res.Role != domain.RoleOwner {
		t.Fatalf("expected already member owner, got %+v", res)
	}
	// 校验未消耗
	cur, _ := invites.GetByID(ctx, inv.ID)
	if cur.UsedCount != 0 {
		t.Fatalf("used_count should remain 0, got %d", cur.UsedCount)
	}
}

func TestInvite_Accept_Exhausted(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newInviteSvc(t)
	actor := setupInviteFixture(t, lakes, memberships)
	inv, _ := svc.Create(ctx, actor, CreateInviteInput{
		LakeID: "lake-1", Role: domain.RolePassenger, MaxUses: 1, TTL: time.Hour,
	})
	// 第一次成功
	if _, err := svc.Accept(ctx, &domain.User{ID: "u1"}, inv.Token); err != nil {
		t.Fatal(err)
	}
	// 第二次应 ErrInvalidInput
	_, err := svc.Accept(ctx, &domain.User{ID: "u2"}, inv.Token)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestInvite_Accept_Expired(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, invites := newInviteSvc(t)
	actor := setupInviteFixture(t, lakes, memberships)
	inv, _ := svc.Create(ctx, actor, CreateInviteInput{
		LakeID: "lake-1", Role: domain.RolePassenger, MaxUses: 5, TTL: time.Hour,
	})
	// 手动倒退 expires_at 到过去
	cur := invites.data[inv.ID]
	cur.ExpiresAt = time.Now().Add(-time.Hour)
	_, err := svc.Accept(ctx, &domain.User{ID: "u1"}, inv.Token)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestInvite_Accept_Revoked(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newInviteSvc(t)
	actor := setupInviteFixture(t, lakes, memberships)
	inv, _ := svc.Create(ctx, actor, CreateInviteInput{
		LakeID: "lake-1", Role: domain.RolePassenger, MaxUses: 5, TTL: time.Hour,
	})
	if err := svc.Revoke(ctx, actor, inv.ID); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Accept(ctx, &domain.User{ID: "u1"}, inv.Token)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestInvite_Revoke_NonOwnerRejected(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newInviteSvc(t)
	actor := setupInviteFixture(t, lakes, memberships)
	inv, _ := svc.Create(ctx, actor, CreateInviteInput{
		LakeID: "lake-1", Role: domain.RolePassenger, MaxUses: 1, TTL: time.Hour,
	})
	// 另一个非创建者、非 OWNER 用户
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-writer", LakeID: "lake-1", Role: domain.RoleNavigator})
	err := svc.Revoke(ctx, &domain.User{ID: "u-writer"}, inv.ID)
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestInvite_Revoke_Idempotent(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newInviteSvc(t)
	actor := setupInviteFixture(t, lakes, memberships)
	inv, _ := svc.Create(ctx, actor, CreateInviteInput{
		LakeID: "lake-1", Role: domain.RolePassenger, MaxUses: 1, TTL: time.Hour,
	})
	if err := svc.Revoke(ctx, actor, inv.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Revoke(ctx, actor, inv.ID); err != nil {
		t.Fatalf("idempotent revoke: %v", err)
	}
}

func TestInvite_Preview_OK(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newInviteSvc(t)
	actor := setupInviteFixture(t, lakes, memberships)
	inv, _ := svc.Create(ctx, actor, CreateInviteInput{
		LakeID: "lake-1", Role: domain.RolePassenger, MaxUses: 2, TTL: time.Hour,
	})
	p, err := svc.Preview(ctx, inv.Token)
	if err != nil {
		t.Fatal(err)
	}
	if p.LakeID != "lake-1" || p.LakeName != "湖 1" || !p.Alive || p.MaxUses != 2 {
		t.Fatalf("bad preview: %+v", p)
	}
}

func TestInvite_List_RequiresWrite(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newInviteSvc(t)
	setupInviteFixture(t, lakes, memberships)
	// outsider 无权限
	_, err := svc.ListByLake(ctx, &domain.User{ID: "outsider"}, "lake-1", false)
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}
