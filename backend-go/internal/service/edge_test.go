package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
)

// memEdgeRepo 实现 EdgeRepository（in-memory）。
type memEdgeRepo struct {
	mu   sync.Mutex
	data map[string]*domain.Edge
}

func newMemEdgeRepo() *memEdgeRepo { return &memEdgeRepo{data: map[string]*domain.Edge{}} }

func (r *memEdgeRepo) Create(_ context.Context, e *domain.Edge) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.data[e.ID]; ok {
		return domain.ErrAlreadyExists
	}
	cp := *e
	r.data[e.ID] = &cp
	return nil
}

func (r *memEdgeRepo) GetByID(_ context.Context, id string) (*domain.Edge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.data[id]; ok {
		cp := *e
		return &cp, nil
	}
	return nil, domain.ErrNotFound
}

func (r *memEdgeRepo) ListByLake(_ context.Context, lakeID string, includeDeleted bool) ([]domain.Edge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.Edge{}
	for _, e := range r.data {
		if e.LakeID != lakeID {
			continue
		}
		if !includeDeleted && e.DeletedAt != nil {
			continue
		}
		out = append(out, *e)
	}
	return out, nil
}

func (r *memEdgeRepo) ExistsAlive(_ context.Context, src, dst string, kind domain.EdgeKind) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.data {
		if e.SrcNodeID == src && e.DstNodeID == dst && e.Kind == kind && e.DeletedAt == nil {
			return true, nil
		}
	}
	return false, nil
}

func (r *memEdgeRepo) SoftDelete(_ context.Context, id string, when time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.data[id]
	if !ok {
		return domain.ErrNotFound
	}
	t := when
	e.DeletedAt = &t
	return nil
}

func newEdgeSvc(t *testing.T) (*EdgeService, *memLakeRepo, *memMembershipRepo, *memNodeRepo, *memEdgeRepo) {
	t.Helper()
	lakes := newMemLakeRepo()
	memberships := newMemMembershipRepo()
	nodes := newMemNodeRepo()
	edges := newMemEdgeRepo()
	return NewEdgeService(edges, nodes, memberships, lakes, nil), lakes, memberships, nodes, edges
}

// 预埋两个同湖节点 + actor 为 PASSENGER。
func setupEdgeFixture(t *testing.T, svc *EdgeService, lakes *memLakeRepo, memberships *memMembershipRepo, nodes *memNodeRepo) (actor *domain.User, srcID, dstID string) {
	t.Helper()
	ctx := context.Background()
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u-owner"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-1", LakeID: "lake-1", Role: domain.RolePassenger})
	nodes.data["n-a"] = &domain.Node{ID: "n-a", LakeID: "lake-1", OwnerID: "u-1", State: domain.StateDrop}
	nodes.data["n-b"] = &domain.Node{ID: "n-b", LakeID: "lake-1", OwnerID: "u-1", State: domain.StateDrop}
	return &domain.User{ID: "u-1"}, "n-a", "n-b"
}

func TestEdge_Create_OK(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes, _ := newEdgeSvc(t)
	actor, src, dst := setupEdgeFixture(t, svc, lakes, memberships, nodes)

	e, err := svc.Create(ctx, actor, CreateEdgeInput{
		SrcNodeID: src, DstNodeID: dst, Kind: domain.EdgeKindRelates,
	})
	if err != nil {
		t.Fatal(err)
	}
	if e.ID == "" || e.LakeID != "lake-1" || e.OwnerID != "u-1" {
		t.Fatalf("bad edge: %+v", e)
	}
}

func TestEdge_Create_RejectsSelfLoop(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes, _ := newEdgeSvc(t)
	actor, src, _ := setupEdgeFixture(t, svc, lakes, memberships, nodes)

	_, err := svc.Create(ctx, actor, CreateEdgeInput{
		SrcNodeID: src, DstNodeID: src, Kind: domain.EdgeKindRelates,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for self loop, got %v", err)
	}
}

func TestEdge_Create_RejectsCrossLake(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes, _ := newEdgeSvc(t)
	actor, src, _ := setupEdgeFixture(t, svc, lakes, memberships, nodes)
	// 在另一个湖加节点 n-c
	lakes.data["lake-2"] = &domain.Lake{ID: "lake-2", OwnerID: "u-other"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-1", LakeID: "lake-2", Role: domain.RolePassenger})
	nodes.data["n-c"] = &domain.Node{ID: "n-c", LakeID: "lake-2", OwnerID: "u-1", State: domain.StateDrop}

	_, err := svc.Create(ctx, actor, CreateEdgeInput{
		SrcNodeID: src, DstNodeID: "n-c", Kind: domain.EdgeKindRelates,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for cross-lake, got %v", err)
	}
}

func TestEdge_Create_RejectsNoWritePermission(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes, _ := newEdgeSvc(t)
	// 不给 actor 写权限
	lakes.data["lake-x"] = &domain.Lake{ID: "lake-x", OwnerID: "u-x"}
	nodes.data["n-x1"] = &domain.Node{ID: "n-x1", LakeID: "lake-x", OwnerID: "u-x", State: domain.StateDrop}
	nodes.data["n-x2"] = &domain.Node{ID: "n-x2", LakeID: "lake-x", OwnerID: "u-x", State: domain.StateDrop}
	// actor 只在另一个湖有权限
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "outsider", LakeID: "lake-y", Role: domain.RolePassenger})

	_, err := svc.Create(ctx, &domain.User{ID: "outsider"}, CreateEdgeInput{
		SrcNodeID: "n-x1", DstNodeID: "n-x2", Kind: domain.EdgeKindRelates,
	})
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestEdge_Create_RejectsCustomWithoutLabel(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes, _ := newEdgeSvc(t)
	actor, src, dst := setupEdgeFixture(t, svc, lakes, memberships, nodes)

	_, err := svc.Create(ctx, actor, CreateEdgeInput{
		SrcNodeID: src, DstNodeID: dst, Kind: domain.EdgeKindCustom,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for custom without label, got %v", err)
	}
}

func TestEdge_Create_RejectsDuplicate(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes, _ := newEdgeSvc(t)
	actor, src, dst := setupEdgeFixture(t, svc, lakes, memberships, nodes)

	_, err := svc.Create(ctx, actor, CreateEdgeInput{
		SrcNodeID: src, DstNodeID: dst, Kind: domain.EdgeKindRelates,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Create(ctx, actor, CreateEdgeInput{
		SrcNodeID: src, DstNodeID: dst, Kind: domain.EdgeKindRelates,
	})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
	// 不同 kind 应允许
	if _, err := svc.Create(ctx, actor, CreateEdgeInput{
		SrcNodeID: src, DstNodeID: dst, Kind: domain.EdgeKindDerives,
	}); err != nil {
		t.Fatalf("different kind should be allowed: %v", err)
	}
}

func TestEdge_Delete_OwnerOK(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes, _ := newEdgeSvc(t)
	actor, src, dst := setupEdgeFixture(t, svc, lakes, memberships, nodes)
	e, err := svc.Create(ctx, actor, CreateEdgeInput{
		SrcNodeID: src, DstNodeID: dst, Kind: domain.EdgeKindRelates,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, actor, e.ID); err != nil {
		t.Fatal(err)
	}
	// 列表不应再包含
	list, _ := svc.ListByLake(ctx, actor, "lake-1", false)
	if len(list) != 0 {
		t.Fatalf("alive list should be empty, got %d", len(list))
	}
	// 包含 deleted 时应能看到
	list, _ = svc.ListByLake(ctx, actor, "lake-1", true)
	if len(list) != 1 {
		t.Fatalf("with deleted should be 1, got %d", len(list))
	}
}

func TestEdge_Delete_OtherUserRejected(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes, _ := newEdgeSvc(t)
	actor, src, dst := setupEdgeFixture(t, svc, lakes, memberships, nodes)
	e, err := svc.Create(ctx, actor, CreateEdgeInput{
		SrcNodeID: src, DstNodeID: dst, Kind: domain.EdgeKindRelates,
	})
	if err != nil {
		t.Fatal(err)
	}
	// 加一个无关用户进湖，但他不是边 owner，且只有 PASSENGER → 仍能写（PASSENGER 即 write）
	// 所以这里测试一个完全不在湖里的用户。
	outsider := &domain.User{ID: "outsider"}
	if err := svc.Delete(ctx, outsider, e.ID); !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestEdge_Delete_LakeWriterCanDelete(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes, _ := newEdgeSvc(t)
	actor, src, dst := setupEdgeFixture(t, svc, lakes, memberships, nodes)
	e, err := svc.Create(ctx, actor, CreateEdgeInput{
		SrcNodeID: src, DstNodeID: dst, Kind: domain.EdgeKindRelates,
	})
	if err != nil {
		t.Fatal(err)
	}
	// 加一个第二位 PASSENGER（非 edge owner，但有湖写权限）→ 允许删
	other := &domain.User{ID: "u-2"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-2", LakeID: "lake-1", Role: domain.RolePassenger})
	if err := svc.Delete(ctx, other, e.ID); err != nil {
		t.Fatalf("lake writer should delete, got %v", err)
	}
}

func TestEdge_Delete_Idempotent(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes, _ := newEdgeSvc(t)
	actor, src, dst := setupEdgeFixture(t, svc, lakes, memberships, nodes)
	e, _ := svc.Create(ctx, actor, CreateEdgeInput{
		SrcNodeID: src, DstNodeID: dst, Kind: domain.EdgeKindRelates,
	})
	if err := svc.Delete(ctx, actor, e.ID); err != nil {
		t.Fatal(err)
	}
	// 再删一次
	if err := svc.Delete(ctx, actor, e.ID); err != nil {
		t.Fatalf("idempotent delete should not error, got %v", err)
	}
}

func TestEdge_List_RequiresReadable(t *testing.T) {
	ctx := context.Background()
	svc, lakes, _, _, _ := newEdgeSvc(t)
	lakes.data["private-lake"] = &domain.Lake{ID: "private-lake", OwnerID: "u-x", IsPublic: false}

	_, err := svc.ListByLake(ctx, &domain.User{ID: "stranger"}, "private-lake", false)
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("private lake should reject stranger, got %v", err)
	}
}
