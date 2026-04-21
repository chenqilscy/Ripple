package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
)

func TestNode_Condense_MISTToDROP(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newNodeSvc(t)
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u-1"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-1", LakeID: "lake-1", Role: domain.RoleOwner})
	now := time.Now().UTC()
	exp := now.Add(7 * 24 * time.Hour)
	nodes.data["n-mist"] = &domain.Node{
		ID: "n-mist", LakeID: "lake-1", OwnerID: "u-1",
		Content: "ai 候选", Type: domain.NodeTypeText, State: domain.StateMist,
		CreatedAt: now, UpdatedAt: now, TTLAt: &exp,
	}

	got, err := svc.Condense(ctx, &domain.User{ID: "u-1"}, "n-mist", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != domain.StateDrop {
		t.Fatalf("want DROP got %s", got.State)
	}
	if got.TTLAt != nil {
		t.Fatalf("DROP must clear TTL")
	}
	if got.LakeID != "lake-1" {
		t.Fatalf("lake should remain")
	}
}

func TestNode_Condense_RequiresWrite(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newNodeSvc(t)
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u-1"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-2", LakeID: "lake-1", Role: domain.RoleObserver})
	now := time.Now().UTC()
	exp := now.Add(7 * 24 * time.Hour)
	nodes.data["n-mist"] = &domain.Node{
		ID: "n-mist", LakeID: "lake-1", OwnerID: "u-2",
		Content: "x", Type: domain.NodeTypeText, State: domain.StateMist,
		CreatedAt: now, UpdatedAt: now, TTLAt: &exp,
	}

	_, err := svc.Condense(ctx, &domain.User{ID: "u-2"}, "n-mist", "")
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("OBSERVER should be denied, got %v", err)
	}
}

func TestNode_Condense_NonMISTRejected(t *testing.T) {
	ctx := context.Background()
	svc, lakes, memberships, nodes := newNodeSvc(t)
	lakes.data["lake-1"] = &domain.Lake{ID: "lake-1", OwnerID: "u-1"}
	_ = memberships.Upsert(ctx, &domain.LakeMembership{UserID: "u-1", LakeID: "lake-1", Role: domain.RoleOwner})
	nodes.data["n-drop"] = &domain.Node{
		ID: "n-drop", LakeID: "lake-1", OwnerID: "u-1",
		Content: "x", Type: domain.NodeTypeText, State: domain.StateDrop,
	}
	_, err := svc.Condense(ctx, &domain.User{ID: "u-1"}, "n-drop", "")
	if !errors.Is(err, domain.ErrInvalidStateTransition) {
		t.Fatalf("DROP should not be condensable, got %v", err)
	}
}
