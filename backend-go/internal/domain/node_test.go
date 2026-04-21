package domain

import (
	"testing"
	"time"
)

func TestRole_AtLeast(t *testing.T) {
	cases := []struct {
		actor, min Role
		ok         bool
	}{
		{RoleOwner, RoleObserver, true},
		{RoleOwner, RoleOwner, true},
		{RolePassenger, RoleNavigator, false},
		{RoleObserver, RolePassenger, false},
		{Role("X"), RoleObserver, false}, // 非法角色 rank=-1
	}
	for _, c := range cases {
		if got := c.actor.AtLeast(c.min); got != c.ok {
			t.Errorf("Role(%s).AtLeast(%s) = %v, want %v", c.actor, c.min, got, c.ok)
		}
	}
}

func TestNode_Evaporate(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	n := &Node{State: StateDrop}
	if err := n.Evaporate(now, 30*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if n.State != StateVapor {
		t.Fatalf("state = %s", n.State)
	}
	if n.DeletedAt == nil || !n.DeletedAt.Equal(now) {
		t.Fatalf("deletedAt mismatch")
	}
	expected := now.Add(30 * 24 * time.Hour)
	if n.TTLAt == nil || !n.TTLAt.Equal(expected) {
		t.Fatalf("ttlAt mismatch")
	}
}

func TestNode_Evaporate_InvalidState(t *testing.T) {
	n := &Node{State: StateMist}
	if err := n.Evaporate(time.Now(), time.Hour); err != ErrInvalidStateTransition {
		t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
	}
}

func TestNode_Restore(t *testing.T) {
	n := &Node{
		State:     StateVapor,
		DeletedAt: ptrTime(time.Now()),
		TTLAt:     ptrTime(time.Now().Add(time.Hour)),
	}
	if err := n.Restore(time.Now()); err != nil {
		t.Fatal(err)
	}
	if n.State != StateDrop {
		t.Fatalf("state = %s", n.State)
	}
	if n.DeletedAt != nil || n.TTLAt != nil {
		t.Fatalf("expected DeletedAt and TTLAt cleared")
	}
}

func TestNode_Condense(t *testing.T) {
	n := &Node{
		State: StateMist,
		TTLAt: ptrTime(time.Now().Add(time.Hour)),
	}
	if err := n.Condense(time.Now(), "lake-1"); err != nil {
		t.Fatal(err)
	}
	if n.State != StateDrop || n.LakeID != "lake-1" || n.TTLAt != nil {
		t.Fatalf("condense fields mismatch: %+v", n)
	}
}

func TestNode_Condense_EmptyLake(t *testing.T) {
	n := &Node{State: StateMist}
	if err := n.Condense(time.Now(), ""); err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
