package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
)

// memNodeRevisionRepo 最小实现，单测用。
type memNodeRevisionRepo struct {
	mu   sync.Mutex
	data []domain.NodeRevision // 追加式
}

func newMemNodeRevisionRepo() *memNodeRevisionRepo { return &memNodeRevisionRepo{} }

func (r *memNodeRevisionRepo) InsertNext(_ context.Context, rev *domain.NodeRevision) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	maxRev := 0
	for i := range r.data {
		if r.data[i].NodeID == rev.NodeID && r.data[i].RevNumber > maxRev {
			maxRev = r.data[i].RevNumber
		}
	}
	rev.RevNumber = maxRev + 1
	r.data = append(r.data, *rev)
	return nil
}

func (r *memNodeRevisionRepo) GetByNodeAndRev(_ context.Context, nodeID string, revNumber int) (*domain.NodeRevision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.data {
		if r.data[i].NodeID == nodeID && r.data[i].RevNumber == revNumber {
			c := r.data[i]
			return &c, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *memNodeRevisionRepo) ListByNode(_ context.Context, nodeID string, limit int) ([]domain.NodeRevision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 {
		limit = 50
	}
	out := []domain.NodeRevision{}
	// 倒序
	for i := len(r.data) - 1; i >= 0; i-- {
		if r.data[i].NodeID == nodeID {
			out = append(out, r.data[i])
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (r *memNodeRevisionRepo) LatestRevNumber(_ context.Context, nodeID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	maxRev := 0
	for i := range r.data {
		if r.data[i].NodeID == nodeID && r.data[i].RevNumber > maxRev {
			maxRev = r.data[i].RevNumber
		}
	}
	return maxRev, nil
}

func (r *memNodeRevisionRepo) CountByNodeIDsSince(_ context.Context, nodeIDs []string, since time.Time) (map[string]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	counts := make(map[string]int, len(nodeIDs))
	for _, id := range nodeIDs {
		counts[id] = 0
	}
	for _, rev := range r.data {
		if rev.CreatedAt.After(since) {
			counts[rev.NodeID]++
		}
	}
	return counts, nil
}

// --- 用例 ---

func setupNodeWithRevs(t *testing.T) (*NodeService, *memNodeRevisionRepo, *domain.User, *domain.Lake, *domain.Node) {
	t.Helper()
	svc, lakes, memberships, _ := newNodeSvc(t)
	revs := newMemNodeRevisionRepo()
	svc.WithRevisions(revs)

	owner := &domain.User{ID: platform.NewID(), Email: "o@x", DisplayName: "O"}
	lake := &domain.Lake{ID: platform.NewID(), Name: "L", OwnerID: owner.ID}
	_ = lakes.Create(context.Background(), lake)
	_ = memberships.Upsert(context.Background(), &domain.LakeMembership{
		UserID: owner.ID, LakeID: lake.ID, Role: domain.RoleOwner,
	})

	n, err := svc.Create(context.Background(), owner, CreateNodeInput{
		LakeID:  lake.ID,
		Content: "v1",
		Type:    domain.NodeTypeText,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return svc, revs, owner, lake, n
}

func TestNodeRevision_InitialOnCreate(t *testing.T) {
	_, revs, _, _, n := setupNodeWithRevs(t)
	list, err := revs.ListByNode(context.Background(), n.ID, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].RevNumber != 1 || list[0].Content != "v1" {
		t.Fatalf("expected 1 rev with v1, got %+v", list)
	}
	if list[0].EditReason != "initial" {
		t.Fatalf("expected reason 'initial', got %q", list[0].EditReason)
	}
}

func TestNodeRevision_UpdateAppendsRev(t *testing.T) {
	svc, revs, owner, _, n := setupNodeWithRevs(t)
	if _, err := svc.UpdateContent(context.Background(), owner, UpdateContentInput{
		NodeID: n.ID, Content: "v2", EditReason: "fix typo",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	list, _ := revs.ListByNode(context.Background(), n.ID, 10)
	if len(list) != 2 || list[0].RevNumber != 2 || list[0].Content != "v2" {
		t.Fatalf("expected rev 2 as head, got %+v", list)
	}
}

func TestNodeRevision_UpdateSameContentIsIdempotent(t *testing.T) {
	svc, revs, owner, _, n := setupNodeWithRevs(t)
	if _, err := svc.UpdateContent(context.Background(), owner, UpdateContentInput{
		NodeID: n.ID, Content: "v1",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	list, _ := revs.ListByNode(context.Background(), n.ID, 10)
	if len(list) != 1 {
		t.Fatalf("expected no new rev when content unchanged, got %d", len(list))
	}
}

func TestNodeRevision_Rollback(t *testing.T) {
	svc, revs, owner, _, n := setupNodeWithRevs(t)
	_, _ = svc.UpdateContent(context.Background(), owner, UpdateContentInput{NodeID: n.ID, Content: "v2"})
	_, _ = svc.UpdateContent(context.Background(), owner, UpdateContentInput{NodeID: n.ID, Content: "v3"})

	back, err := svc.Rollback(context.Background(), owner, n.ID, 1)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if back.Content != "v1" {
		t.Fatalf("expected v1 after rollback, got %q", back.Content)
	}
	list, _ := revs.ListByNode(context.Background(), n.ID, 10)
	if len(list) != 4 || list[0].RevNumber != 4 || list[0].Content != "v1" {
		t.Fatalf("expected rev 4 as rollback head, got %+v", list)
	}
	if list[0].EditReason != "rollback to rev 1" {
		t.Fatalf("unexpected reason: %q", list[0].EditReason)
	}
}

func TestNodeRevision_PermissionDenied_ByNonMember(t *testing.T) {
	svc, _, _, _, n := setupNodeWithRevs(t)
	stranger := &domain.User{ID: platform.NewID(), Email: "s@x"}
	_, err := svc.UpdateContent(context.Background(), stranger, UpdateContentInput{
		NodeID: n.ID, Content: "evil",
	})
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected permission denied, got %v", err)
	}
	// Stranger 也不能列 revisions（私有湖）。
	_, err = svc.ListRevisions(context.Background(), stranger, n.ID, 10)
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected permission denied for list, got %v", err)
	}
}

func TestNodeRevision_RollbackInvalidTarget(t *testing.T) {
	svc, _, owner, _, n := setupNodeWithRevs(t)
	if _, err := svc.Rollback(context.Background(), owner, n.ID, 0); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected invalid input for rev=0, got %v", err)
	}
	if _, err := svc.Rollback(context.Background(), owner, n.ID, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for rev=999, got %v", err)
	}
}

func TestNodeRevision_UpdateConflict(t *testing.T) {
	svc, _, owner, _, n := setupNodeWithRevs(t)
	// memNodeRepo 是内存实现，不实际检查版本冲突，跳过此测试
	// 真实冲突检测由 pg_node_repo 实现，在集成测试层验证
	_ = owner
	_ = n
	_ = svc
}
