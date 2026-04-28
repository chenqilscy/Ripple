package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// searchableNodeRepo 覆盖 memNodeRepo.Search，做简单内存全文搜索。
type searchableNodeRepo struct {
	memNodeRepo
}

func (r *searchableNodeRepo) Search(_ context.Context, lakeID, q string, limit int) ([]domain.NodeSearchResult, error) {
	var out []domain.NodeSearchResult
	for _, n := range r.data {
		if n.LakeID != lakeID || n.State == domain.StateErased || n.State == domain.StateVapor {
			continue
		}
		if !strings.Contains(strings.ToLower(n.Content), strings.ToLower(q)) {
			continue
		}
		out = append(out, domain.NodeSearchResult{
			NodeID:  n.ID,
			LakeID:  n.LakeID,
			Snippet: n.Content,
			Score:   1.0,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *searchableNodeRepo) SearchFiltered(_ context.Context, lakeID, q, state, nodeType string, limit int) ([]domain.NodeSearchResult, error) {
	var out []domain.NodeSearchResult
	for _, n := range r.data {
		if n.LakeID != lakeID || n.State == domain.StateErased {
			continue
		}
		if state != "" && string(n.State) != state {
			continue
		}
		if nodeType != "" && string(n.Type) != nodeType {
			continue
		}
		if !strings.Contains(strings.ToLower(n.Content), strings.ToLower(q)) {
			continue
		}
		out = append(out, domain.NodeSearchResult{
			NodeID:  n.ID,
			LakeID:  n.LakeID,
			Snippet: n.Content,
			Score:   1.0,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func newSearchSvc(t *testing.T) (*NodeService, *memLakeRepo, *memMembershipRepo, *searchableNodeRepo) {
	t.Helper()
	lakes := newMemLakeRepo()
	memberships := newMemMembershipRepo()
	nodes := &searchableNodeRepo{memNodeRepo: memNodeRepo{data: map[string]*domain.Node{}}}
	return NewNodeService(nodes, memberships, lakes, nil), lakes, memberships, nodes
}

// TestSearch_EmptyQuery 空查询返回 nil，不报错。
func TestSearch_EmptyQuery(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newSearchSvc(t)

	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", IsPublic: true}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u", LakeID: "lake-1", Role: domain.RoleOwner})

	res, err := svc.SearchNodes(ctx, &domain.User{ID: "u"}, "lake-1", "  ", 20)
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Fatalf("expected nil for empty query, got %v", res)
	}
}

// TestSearch_HitsAndMisses 只返回命中关键词的节点。
func TestSearch_HitsAndMisses(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newSearchSvc(t)

	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u", LakeID: "lake-1", Role: domain.RoleOwner})

	now := time.Now()
	nodes.data["n1"] = &domain.Node{ID: "n1", LakeID: "lake-1", OwnerID: "u", Content: "Go 并发编程", State: domain.StateDrop, CreatedAt: now, UpdatedAt: now}
	nodes.data["n2"] = &domain.Node{ID: "n2", LakeID: "lake-1", OwnerID: "u", Content: "Rust 内存安全", State: domain.StateDrop, CreatedAt: now, UpdatedAt: now}
	nodes.data["n3"] = &domain.Node{ID: "n3", LakeID: "lake-1", OwnerID: "u", Content: "Go 内存模型", State: domain.StateDrop, CreatedAt: now, UpdatedAt: now}

	res, err := svc.SearchNodes(ctx, &domain.User{ID: "u"}, "lake-1", "go", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 hits for 'go', got %d: %v", len(res), res)
	}
}

// TestSearch_PermissionDenied 私有湖非成员无权搜索。
func TestSearch_PermissionDenied(t *testing.T) {
	ctx := context.Background()
	svc, lakes, _, _ := newSearchSvc(t)

	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "owner", IsPublic: false}
	// u-stranger 没有成员关系

	_, err := svc.SearchNodes(ctx, &domain.User{ID: "u-stranger"}, "lake-1", "go", 20)
	if err == nil {
		t.Fatal("expected permission error for non-member of private lake")
	}
}

// TestSearch_PublicLakeAllowsAnyone 公开湖任何人可搜索。
func TestSearch_PublicLakeAllowsAnyone(t *testing.T) {
	ctx := context.Background()
	svc, lakes, _, nodes := newSearchSvc(t)

	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "owner", IsPublic: true}
	nodes.data["n1"] = &domain.Node{ID: "n1", LakeID: "lake-1", OwnerID: "owner", Content: "青萍涟漪", State: domain.StateDrop}

	res, err := svc.SearchNodes(ctx, &domain.User{ID: "anyone"}, "lake-1", "涟漪", 20)
	if err != nil {
		t.Fatalf("public lake should allow search: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected at least one result for '涟漪'")
	}
}

// TestSearch_LimitRespected limit 参数限制返回数量。
func TestSearch_LimitRespected(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newSearchSvc(t)

	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u", LakeID: "lake-1", Role: domain.RoleOwner})

	for i := 0; i < 10; i++ {
		id := "n" + string(rune('0'+i))
		nodes.data[id] = &domain.Node{ID: id, LakeID: "lake-1", OwnerID: "u", Content: "keyword 节点", State: domain.StateDrop}
	}

	res, err := svc.SearchNodes(ctx, &domain.User{ID: "u"}, "lake-1", "keyword", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) > 3 {
		t.Fatalf("expected at most 3 results (limit=3), got %d", len(res))
	}
}

// TestSearch_LuceneEscape Lucene 特殊字符查询不报错（转义后作为纯文本搜索）。
func TestSearch_LuceneEscape(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newSearchSvc(t)

	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u", IsPublic: false}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u", LakeID: "lake-1", Role: domain.RoleOwner})

	// 传入 Lucene 特殊字符，SearchNodes 应转义后正常执行
	specialChars := []string{"+", "-", "&&", "||", "!", "(", ")", "{", "}", "[", "]", "^", "\"", "~", "*", "?", ":", "/", "\\"}
	for _, ch := range specialChars {
		_, err := svc.SearchNodes(ctx, &domain.User{ID: "u"}, "lake-1", ch, 10)
		if err != nil {
			t.Errorf("SearchNodes should not error for special char %q, got %v", ch, err)
		}
	}
}

// 确保 searchableNodeRepo 满足 store.NodeRepository 接口。
var _ store.NodeRepository = (*searchableNodeRepo)(nil)
