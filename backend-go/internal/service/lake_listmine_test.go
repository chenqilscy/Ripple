package service

import (
	"context"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
)

// TestLake_ListMineFull_ZippedWithRole 验证 ListMineFull 正确 zip 湖和角色。
func TestLake_ListMineFull_ZippedWithRole(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newLakeSvc()

	lakes.data["lk-1"] = &domain.Lake{ID: "lk-1", Name: "一号湖", OwnerID: "u"}
	lakes.data["lk-2"] = &domain.Lake{ID: "lk-2", Name: "二号湖", OwnerID: "v"}
	now := time.Now().UTC()
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u", LakeID: "lk-1", Role: domain.RoleOwner, UpdatedAt: now})
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u", LakeID: "lk-2", Role: domain.RolePassenger, UpdatedAt: now})

	items, err := svc.ListMineFull(ctx, &domain.User{ID: "u"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2, got %d", len(items))
	}
	roles := map[string]domain.Role{}
	for _, it := range items {
		roles[it.Lake.ID] = it.Role
	}
	if roles["lk-1"] != domain.RoleOwner || roles["lk-2"] != domain.RolePassenger {
		t.Fatalf("role zip wrong: %+v", roles)
	}
}

// TestLake_ListMineFull_SkipsMissingLake outbox 滞后：membership 有但 Lake 还没投到仓库。
func TestLake_ListMineFull_SkipsMissingLake(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newLakeSvc()

	lakes.data["lk-exist"] = &domain.Lake{ID: "lk-exist", Name: "存在", OwnerID: "u"}
	// lk-missing 没建 Lake 节点
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u", LakeID: "lk-exist", Role: domain.RoleOwner})
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u", LakeID: "lk-missing", Role: domain.RoleOwner})

	items, err := svc.ListMineFull(ctx, &domain.User{ID: "u"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Lake.ID != "lk-exist" {
		t.Fatalf("should skip missing, got %+v", items)
	}
}

// TestLake_ListMineFull_Empty 用户无任何加入湖。
func TestLake_ListMineFull_Empty(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := newLakeSvc()
	items, err := svc.ListMineFull(ctx, &domain.User{ID: "nobody"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("want empty, got %d", len(items))
	}
}
