package service

import (
	"context"
	"errors"
	"testing"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
)

// memNodeRepo 实现 NodeRepository。
type memNodeRepo struct{ data map[string]*domain.Node }

func newMemNodeRepo() *memNodeRepo { return &memNodeRepo{data: map[string]*domain.Node{}} }
func (r *memNodeRepo) Create(_ context.Context, n *domain.Node) error {
	if _, ok := r.data[n.ID]; ok {
		return domain.ErrAlreadyExists
	}
	r.data[n.ID] = n
	return nil
}
func (r *memNodeRepo) GetByID(_ context.Context, id string) (*domain.Node, error) {
	if n, ok := r.data[id]; ok {
		return n, nil
	}
	return nil, domain.ErrNotFound
}
func (r *memNodeRepo) ListByLake(_ context.Context, lakeID string, includeVapor bool) ([]domain.Node, error) {
	out := []domain.Node{}
	for _, n := range r.data {
		if n.LakeID != lakeID {
			continue
		}
		if !includeVapor && n.State == domain.StateVapor {
			continue
		}
		out = append(out, *n)
	}
	return out, nil
}
func (r *memNodeRepo) UpdateState(_ context.Context, n *domain.Node) error {
	if _, ok := r.data[n.ID]; !ok {
		return domain.ErrNotFound
	}
	r.data[n.ID] = n
	return nil
}
func (r *memNodeRepo) UpdateContent(_ context.Context, n *domain.Node) error {
	if _, ok := r.data[n.ID]; !ok {
		return domain.ErrNotFound
	}
	r.data[n.ID] = n
	return nil
}
func (r *memNodeRepo) Search(_ context.Context, lakeID, q string, _ int) ([]domain.NodeSearchResult, error) {
	return []domain.NodeSearchResult{}, nil
}
func (r *memNodeRepo) BatchCreate(_ context.Context, nodes []*domain.Node) error {
	for _, n := range nodes {
		r.data[n.ID] = n
	}
	return nil
}

func newNodeSvc(t *testing.T) (*NodeService, *memLakeRepo, *memMembershipRepo, *memNodeRepo) {
	t.Helper()
	lakes := newMemLakeRepo()
	memberships := newMemMembershipRepo()
	nodes := newMemNodeRepo()
	// broker=nil：测试不关心广播
	return NewNodeService(nodes, memberships, lakes, nil), lakes, memberships, nodes
}

func TestNode_Create_RequiresWrite(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newNodeSvc(t)
	// 预埋 lake 与 OBSERVER 角色（不足以写）
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u-1"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-2", LakeID: "lake-1", Role: domain.RoleObserver})

	_, err := svc.Create(ctx, &domain.User{ID: "u-2"},
		CreateNodeInput{LakeID: "lake-1", Content: "x", Type: domain.NodeTypeText})
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestNode_Create_OK_AsPassenger(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newNodeSvc(t)
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u-1"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-2", LakeID: "lake-1", Role: domain.RolePassenger})

	n, err := svc.Create(ctx, &domain.User{ID: "u-2"},
		CreateNodeInput{LakeID: "lake-1", Content: "hi", Type: domain.NodeTypeText})
	if err != nil {
		t.Fatal(err)
	}
	if n.State != domain.StateDrop {
		t.Fatalf("expected DROP state")
	}
	if n.OwnerID != "u-2" {
		t.Fatalf("owner wrong")
	}
}

func TestNode_Evaporate_OnlyOwnerOrLakeOwner(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newNodeSvc(t)
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "lake-owner"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "lake-owner", LakeID: "lake-1", Role: domain.RoleOwner})
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "node-owner", LakeID: "lake-1", Role: domain.RolePassenger})
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "other", LakeID: "lake-1", Role: domain.RolePassenger})
	nodes.data["n-1"] = &domain.Node{ID: "n-1", LakeID: "lake-1", OwnerID: "node-owner", State: domain.StateDrop}

	// other 不能蒸发别人节点
	_, err := svc.Evaporate(ctx, &domain.User{ID: "other"}, "n-1")
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for other, got %v", err)
	}

	// node-owner 自己可以
	n, err := svc.Evaporate(ctx, &domain.User{ID: "node-owner"}, "n-1")
	if err != nil {
		t.Fatal(err)
	}
	if n.State != domain.StateVapor {
		t.Fatalf("expected VAPOR")
	}

	// 再蒸发：状态机拒绝
	_, err = svc.Evaporate(ctx, &domain.User{ID: "node-owner"}, "n-1")
	if !errors.Is(err, domain.ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
	}
}

func TestNode_Evaporate_LakeOwnerCanDoOthers(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newNodeSvc(t)
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "lake-owner"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "lake-owner", LakeID: "lake-1", Role: domain.RoleOwner})
	nodes.data["n-1"] = &domain.Node{ID: "n-1", LakeID: "lake-1", OwnerID: "some-other-user", State: domain.StateDrop}

	if _, err := svc.Evaporate(ctx, &domain.User{ID: "lake-owner"}, "n-1"); err != nil {
		t.Fatalf("lake-owner should mutate: %v", err)
	}
}

func TestNode_Restore_FromVapor(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newNodeSvc(t)
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u", LakeID: "lake-1", Role: domain.RoleOwner})
	nodes.data["n"] = &domain.Node{ID: "n", LakeID: "lake-1", OwnerID: "u", State: domain.StateVapor}

	n, err := svc.Restore(ctx, &domain.User{ID: "u"}, "n")
	if err != nil {
		t.Fatal(err)
	}
	if n.State != domain.StateDrop {
		t.Fatalf("expected DROP, got %s", n.State)
	}
}

func TestNode_Create_InvalidType(t *testing.T) {
	svc, lakes, memberships, _ := newNodeSvc(t)
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u"}
	_ = memberships.Upsert(context.Background(), &domain.LakeMembership{UserID: "u", LakeID: "lake-1", Role: domain.RoleOwner})
	_, err := svc.Create(context.Background(), &domain.User{ID: "u"},
		CreateNodeInput{LakeID: "lake-1", Content: "hi", Type: "WRONG"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}
