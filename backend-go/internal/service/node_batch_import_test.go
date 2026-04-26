package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
)

// TestNode_BatchImport_1000Rows P12-A 验收：1000 行批量导入成功率 100%。
func TestNode_BatchImport_1000Rows(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newNodeSvc(t)
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u-1"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-1", LakeID: "lake-1", Role: domain.RoleNavigator})

	items := make([]BatchImportItem, 1000)
	for i := range items {
		items[i] = BatchImportItem{Content: fmt.Sprintf("node-%04d", i), Type: domain.NodeTypeText}
	}

	result, err := svc.BatchImportNodes(ctx, &domain.User{ID: "u-1"}, "lake-1", items)
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if result.Created != 1000 {
		t.Fatalf("expected 1000 created, got %d", result.Created)
	}
	if len(nodes.data) != 1000 {
		t.Fatalf("expected repo to hold 1000 nodes, got %d", len(nodes.data))
	}
}

// TestNode_BatchImport_BadRowsSkipped 校验空 content 与超长 content 的处理：
//   - 空 / 只含空白：跳过；
//   - 超过 10000 字符：截断后写入；
//   - 不含坏行的总行数 > 1000：返回 ErrInvalidInput。
func TestNode_BatchImport_BadRowsSkipped(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newNodeSvc(t)
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u-1"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-1", LakeID: "lake-1", Role: domain.RoleNavigator})

	long := make([]rune, 12000)
	for i := range long {
		long[i] = 'a'
	}

	items := []BatchImportItem{
		{Content: "ok-1", Type: domain.NodeTypeText},
		{Content: "   ", Type: domain.NodeTypeText},   // 跳过
		{Content: "", Type: domain.NodeTypeText},      // 跳过
		{Content: string(long), Type: domain.NodeTypeText}, // 截断
	}
	result, err := svc.BatchImportNodes(ctx, &domain.User{ID: "u-1"}, "lake-1", items)
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if result.Created != 2 {
		t.Fatalf("expected 2 created (ok-1 + truncated), got %d", result.Created)
	}
	for _, n := range result.Nodes {
		if len([]rune(n.Content)) > 10000 {
			t.Fatalf("content not truncated: %d runes", len([]rune(n.Content)))
		}
	}
}

// TestNode_BatchImport_OverLimit 超过 maxBatchImport (1000) 应返回 ErrInvalidInput。
func TestNode_BatchImport_OverLimit(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, _ := newNodeSvc(t)
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u-1"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-1", LakeID: "lake-1", Role: domain.RoleNavigator})

	items := make([]BatchImportItem, 1001)
	for i := range items {
		items[i] = BatchImportItem{Content: fmt.Sprintf("n-%d", i), Type: domain.NodeTypeText}
	}
	_, err := svc.BatchImportNodes(ctx, &domain.User{ID: "u-1"}, "lake-1", items)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for >1000 rows, got %v", err)
	}
}
