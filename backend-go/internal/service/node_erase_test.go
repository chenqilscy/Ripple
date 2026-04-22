package service

import (
	"context"
	"errors"
	"testing"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
)

// TestNode_Erase_OwnerCanErase 节点 owner 可以删除自己的节点。
func TestNode_Erase_OwnerCanErase(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newNodeSvc(t)

	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "lake-owner"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "lake-owner", LakeID: "lake-1", Role: domain.RoleOwner})
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-1", LakeID: "lake-1", Role: domain.RolePassenger})
	nodes.data["n-1"] = &domain.Node{ID: "n-1", LakeID: "lake-1", OwnerID: "u-1", State: domain.StateDrop}

	n, err := svc.Erase(ctx, &domain.User{ID: "u-1"}, "n-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.State != domain.StateErased {
		t.Fatalf("expected ERASED state, got %s", n.State)
	}
	if n.DeletedAt == nil {
		t.Fatal("expected DeletedAt to be set")
	}
}

// TestNode_Erase_NavigatorCanErase NAVIGATOR+ 可删他人节点。
func TestNode_Erase_NavigatorCanErase(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newNodeSvc(t)

	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "owner"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "owner", LakeID: "lake-1", Role: domain.RoleOwner})
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "nav", LakeID: "lake-1", Role: domain.RoleNavigator})
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "node-owner", LakeID: "lake-1", Role: domain.RolePassenger})
	nodes.data["n-1"] = &domain.Node{ID: "n-1", LakeID: "lake-1", OwnerID: "node-owner", State: domain.StateDrop}

	_, err := svc.Erase(ctx, &domain.User{ID: "nav"}, "n-1")
	if err != nil {
		t.Fatalf("navigator should be able to erase: %v", err)
	}
}

// TestNode_Erase_PassengerCannotEraseOthers PASSENGER 不能删他人节点。
func TestNode_Erase_PassengerCannotEraseOthers(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newNodeSvc(t)

	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "owner"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "owner", LakeID: "lake-1", Role: domain.RoleOwner})
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-pass", LakeID: "lake-1", Role: domain.RolePassenger})
	nodes.data["n-1"] = &domain.Node{ID: "n-1", LakeID: "lake-1", OwnerID: "other-user", State: domain.StateDrop}

	_, err := svc.Erase(ctx, &domain.User{ID: "u-pass"}, "n-1")
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for passenger erasing others, got %v", err)
	}
}

// TestNode_Erase_AlreadyErased 已 ERASED 的节点不能再 Erase。
func TestNode_Erase_AlreadyErased(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newNodeSvc(t)

	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u", LakeID: "lake-1", Role: domain.RoleOwner})
	nodes.data["n-1"] = &domain.Node{ID: "n-1", LakeID: "lake-1", OwnerID: "u", State: domain.StateErased}

	_, err := svc.Erase(ctx, &domain.User{ID: "u"}, "n-1")
	if !errors.Is(err, domain.ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition for double-erase, got %v", err)
	}
}

// TestNode_Erase_VaporCanBeErased VAPOR 节点也可以手动 Erase（快速清理）。
func TestNode_Erase_VaporCanBeErased(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newNodeSvc(t)

	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u", LakeID: "lake-1", Role: domain.RoleOwner})
	nodes.data["n-1"] = &domain.Node{ID: "n-1", LakeID: "lake-1", OwnerID: "u", State: domain.StateVapor}

	n, err := svc.Erase(ctx, &domain.User{ID: "u"}, "n-1")
	if err != nil {
		t.Fatalf("VAPOR node should be erasable: %v", err)
	}
	if n.State != domain.StateErased {
		t.Fatalf("expected ERASED, got %s", n.State)
	}
}

// TestNode_BatchOperate_Erase_Succeeds 批量 erase 成功统计。
func TestNode_BatchOperate_Erase_Succeeds(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newNodeSvc(t)

	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u", LakeID: "lake-1", Role: domain.RoleOwner})
	for _, id := range []string{"n1", "n2", "n3"} {
		nodes.data[id] = &domain.Node{ID: id, LakeID: "lake-1", OwnerID: "u", State: domain.StateDrop}
	}

	res, err := svc.BatchOperate(ctx, &domain.User{ID: "u"}, "lake-1", "erase", []string{"n1", "n2", "n3"})
	if err != nil {
		t.Fatalf("BatchOperate erase error: %v", err)
	}
	if res.Succeeded != 3 || res.Failed != 0 {
		t.Fatalf("expected 3 succeeded, got succeeded=%d failed=%d", res.Succeeded, res.Failed)
	}
	// 验证状态已变更
	for _, id := range []string{"n1", "n2", "n3"} {
		if nodes.data[id].State != domain.StateErased {
			t.Errorf("node %s expected ERASED, got %s", id, nodes.data[id].State)
		}
	}
}

// TestNode_BatchOperate_InvalidAction 非法 action 应返回 ErrInvalidInput。
func TestNode_BatchOperate_InvalidAction(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := newNodeSvc(t)

	_, err := svc.BatchOperate(ctx, &domain.User{ID: "u"}, "lake-1", "delete_forever", []string{"n1"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for invalid action, got %v", err)
	}
}

// TestNode_BatchOperate_TooMany 超过 200 个节点应返回 ErrInvalidInput。
func TestNode_BatchOperate_TooMany(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := newNodeSvc(t)

	ids := make([]string, 201)
	for i := range ids {
		ids[i] = "n"
	}
	_, err := svc.BatchOperate(ctx, &domain.User{ID: "u"}, "lake-1", "evaporate", ids)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for too many nodes, got %v", err)
	}
}

// TestNode_BatchOperate_PartialFailure 部分节点状态不可蒸发时 failed 计数正确。
func TestNode_BatchOperate_PartialFailure(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newNodeSvc(t)

	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u", LakeID: "lake-1", Role: domain.RoleOwner})
	// n1 状态 DROP（可蒸发），n2 状态 VAPOR（不可再蒸发）
	nodes.data["n1"] = &domain.Node{ID: "n1", LakeID: "lake-1", OwnerID: "u", State: domain.StateDrop}
	nodes.data["n2"] = &domain.Node{ID: "n2", LakeID: "lake-1", OwnerID: "u", State: domain.StateVapor}

	res, err := svc.BatchOperate(ctx, &domain.User{ID: "u"}, "lake-1", "evaporate", []string{"n1", "n2"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Succeeded != 1 || res.Failed != 1 {
		t.Fatalf("expected succeeded=1 failed=1, got succeeded=%d failed=%d", res.Succeeded, res.Failed)
	}
}
