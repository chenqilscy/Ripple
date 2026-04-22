package service

import (
	"context"
	"errors"
	"testing"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// memTagRepo 实现 store.TagRepository（内存版）。
type memTagRepo struct {
	// nodeTags: nodeID -> []tag
	nodeTags map[string][]string
	// lakeTags: lakeID|nodeID -> struct{}  (用于 ListLakeTags / ListNodesByTag)
	// 简化：直接从 nodeTags 计算
	lakeOf map[string]string // nodeID -> lakeID
}

func newMemTagRepo() *memTagRepo {
	return &memTagRepo{
		nodeTags: map[string][]string{},
		lakeOf:   map[string]string{},
	}
}

func (r *memTagRepo) SetTags(_ context.Context, nodeID, lakeID string, tags []string) error {
	r.nodeTags[nodeID] = append([]string{}, tags...) // 复制
	r.lakeOf[nodeID] = lakeID
	return nil
}

func (r *memTagRepo) GetTags(_ context.Context, nodeID string) ([]string, error) {
	return append([]string{}, r.nodeTags[nodeID]...), nil
}

func (r *memTagRepo) ListLakeTags(_ context.Context, lakeID string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	for nid, tags := range r.nodeTags {
		if r.lakeOf[nid] != lakeID {
			continue
		}
		for _, t := range tags {
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				out = append(out, t)
			}
		}
	}
	return out, nil
}

func (r *memTagRepo) ListNodesByTag(_ context.Context, lakeID, tag string) ([]string, error) {
	var out []string
	for nid, tags := range r.nodeTags {
		if r.lakeOf[nid] != lakeID {
			continue
		}
		for _, t := range tags {
			if t == tag {
				out = append(out, nid)
				break
			}
		}
	}
	return out, nil
}

// newTagSvc 组装 TagService + 所需依赖。
func newTagSvc(t *testing.T) (*TagService, *memTagRepo, *memMembershipRepo, *memNodeRepo) {
	t.Helper()
	tags := newMemTagRepo()
	memberships := newMemMembershipRepo()
	nodes := newMemNodeRepo()
	return NewTagService(tags, memberships, nodes), tags, memberships, nodes
}

// TestTagService_SetNodeTags_OK 正常写标签。
func TestTagService_SetNodeTags_OK(t *testing.T) {
	ctx := context.Background()
	svc, tagsRepo, memberships, nodes := newTagSvc(t)

	nodes.data["n-1"] = &domain.Node{ID: "n-1", LakeID: "lake-1", OwnerID: "u-1", State: domain.StateDrop}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-1", LakeID: "lake-1", Role: domain.RoleOwner})

	err := svc.SetNodeTags(ctx, &domain.User{ID: "u-1"}, "n-1", []string{"go", "后端"})
	if err != nil {
		t.Fatalf("SetNodeTags unexpected error: %v", err)
	}

	tags, _ := tagsRepo.GetTags(ctx, "n-1")
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %v", tags)
	}
}

// TestTagService_SetNodeTags_InvalidTag 非法标签格式应返回 ErrInvalidInput。
func TestTagService_SetNodeTags_InvalidTag(t *testing.T) {
	ctx := context.Background()
	svc, _, memberships, nodes := newTagSvc(t)

	nodes.data["n-1"] = &domain.Node{ID: "n-1", LakeID: "lake-1", OwnerID: "u-1", State: domain.StateDrop}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-1", LakeID: "lake-1", Role: domain.RoleOwner})

	err := svc.SetNodeTags(ctx, &domain.User{ID: "u-1"}, "n-1", []string{"valid", "invalid tag!"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for bad tag, got %v", err)
	}
}

// TestTagService_SetNodeTags_TooMany 超过 20 个标签应拒绝。
func TestTagService_SetNodeTags_TooMany(t *testing.T) {
	ctx := context.Background()
	svc, _, memberships, nodes := newTagSvc(t)

	nodes.data["n-1"] = &domain.Node{ID: "n-1", LakeID: "lake-1", OwnerID: "u-1", State: domain.StateDrop}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-1", LakeID: "lake-1", Role: domain.RoleOwner})

	tags := make([]string, 21)
	for i := range tags {
		tags[i] = "tag" + string(rune('A'+i%26))
	}
	err := svc.SetNodeTags(ctx, &domain.User{ID: "u-1"}, "n-1", tags)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for too many tags, got %v", err)
	}
}

// TestTagService_SetNodeTags_NotMember 非成员不能写标签。
func TestTagService_SetNodeTags_NotMember(t *testing.T) {
	ctx := context.Background()
	svc, _, _, nodes := newTagSvc(t)

	nodes.data["n-1"] = &domain.Node{ID: "n-1", LakeID: "lake-1", OwnerID: "u-1", State: domain.StateDrop}
	// 不添加 u-2 的成员关系

	err := svc.SetNodeTags(ctx, &domain.User{ID: "u-2"}, "n-1", []string{"go"})
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

// TestTagService_ListNodesByTag 按标签过滤节点。
func TestTagService_ListNodesByTag(t *testing.T) {
	ctx := context.Background()
	svc, tagsRepo, memberships, nodes := newTagSvc(t)

	for _, id := range []string{"n-1", "n-2", "n-3"} {
		nodes.data[id] = &domain.Node{ID: id, LakeID: "lake-1", OwnerID: "u", State: domain.StateDrop}
	}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u", LakeID: "lake-1", Role: domain.RoleOwner})

	_ = tagsRepo.SetTags(ctx, "n-1", "lake-1", []string{"go", "api"})
	_ = tagsRepo.SetTags(ctx, "n-2", "lake-1", []string{"go"})
	_ = tagsRepo.SetTags(ctx, "n-3", "lake-1", []string{"rust"})

	ids, err := svc.ListNodesByTag(ctx, &domain.User{ID: "u"}, "lake-1", "go")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 nodes with tag 'go', got %d: %v", len(ids), ids)
	}
}

// TestTagService_ListLakeTags 湖内标签去重列表。
func TestTagService_ListLakeTags(t *testing.T) {
	ctx := context.Background()
	svc, tagsRepo, memberships, nodes := newTagSvc(t)

	for _, id := range []string{"n-1", "n-2"} {
		nodes.data[id] = &domain.Node{ID: id, LakeID: "lake-1", OwnerID: "u", State: domain.StateDrop}
	}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u", LakeID: "lake-1", Role: domain.RoleOwner})
	_ = tagsRepo.SetTags(ctx, "n-1", "lake-1", []string{"go", "api"})
	_ = tagsRepo.SetTags(ctx, "n-2", "lake-1", []string{"go", "rust"})

	tags, err := svc.ListLakeTags(ctx, &domain.User{ID: "u"}, "lake-1")
	if err != nil {
		t.Fatal(err)
	}
	// 去重后应为 3 个（go, api, rust）
	if len(tags) != 3 {
		t.Fatalf("expected 3 unique tags, got %d: %v", len(tags), tags)
	}
}

// 确保 memTagRepo 满足接口。
var _ store.TagRepository = (*memTagRepo)(nil)
