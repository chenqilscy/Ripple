package presence

import (
	"context"
	"testing"
	"time"
)

func TestPresence_JoinListLeave_Memory(t *testing.T) {
	svc := NewService(nil, nil, 5*time.Second)
	ctx := context.Background()

	if err := svc.Join(ctx, "lake-1", "u-1"); err != nil {
		t.Fatalf("join: %v", err)
	}
	if err := svc.Join(ctx, "lake-1", "u-2"); err != nil {
		t.Fatalf("join: %v", err)
	}

	got, err := svc.List(ctx, "lake-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 users online, got %d: %v", len(got), got)
	}

	if err := svc.Leave(ctx, "lake-1", "u-1"); err != nil {
		t.Fatalf("leave: %v", err)
	}
	got, _ = svc.List(ctx, "lake-1")
	if len(got) != 1 || got[0] != "u-2" {
		t.Fatalf("expected only u-2, got %v", got)
	}

	// Leave 不存在的 member 幂等。
	if err := svc.Leave(ctx, "lake-1", "ghost"); err != nil {
		t.Fatalf("idempotent leave: %v", err)
	}
}

func TestPresence_HeartbeatExtendsTTL(t *testing.T) {
	svc := NewService(nil, nil, 50*time.Millisecond)
	ctx := context.Background()

	_ = svc.Join(ctx, "lake-1", "u-1")
	time.Sleep(30 * time.Millisecond)
	// 心跳续期。
	_ = svc.Heartbeat(ctx, "lake-1", "u-1")
	time.Sleep(30 * time.Millisecond)
	// 此时原 TTL 应已过，但因心跳后应仍在线。
	got, _ := svc.List(ctx, "lake-1")
	if len(got) != 1 {
		t.Fatalf("expected 1 online after heartbeat, got %v", got)
	}
	// 等到心跳也过期。
	time.Sleep(60 * time.Millisecond)
	got, _ = svc.List(ctx, "lake-1")
	if len(got) != 0 {
		t.Fatalf("expected empty after TTL, got %v", got)
	}
}

func TestPresence_EmptyLakeReturnsEmpty(t *testing.T) {
	svc := NewService(nil, nil, time.Second)
	got, err := svc.List(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty list, got %v", got)
	}
}

func TestPresence_JoinValidatesInput(t *testing.T) {
	svc := NewService(nil, nil, time.Second)
	if err := svc.Join(context.Background(), "", "u"); err == nil {
		t.Fatalf("expected error on empty lake_id")
	}
	if err := svc.Join(context.Background(), "l", ""); err == nil {
		t.Fatalf("expected error on empty user_id")
	}
}
