package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// memNotificationRepo 实现 store.NotificationRepository（内存版）。
type memNotificationRepo struct {
	nextID int64
	items  []*store.Notification
}

func newMemNotificationRepo() *memNotificationRepo {
	return &memNotificationRepo{nextID: 1}
}

func (r *memNotificationRepo) Create(_ context.Context, userID, notifType string, payload json.RawMessage) (*store.Notification, error) {
	n := &store.Notification{
		ID:        r.nextID,
		UserID:    userID,
		Type:      notifType,
		Payload:   payload,
		IsRead:    false,
		CreatedAt: time.Now(),
	}
	r.nextID++
	r.items = append(r.items, n)
	return n, nil
}

func (r *memNotificationRepo) ListForUser(_ context.Context, userID string, limit int, before int64) ([]store.Notification, error) {
	var out []store.Notification
	// iterate in reverse (newest first), skip if id >= before (when before > 0)
	for i := len(r.items) - 1; i >= 0; i-- {
		n := r.items[i]
		if n.UserID != userID {
			continue
		}
		if before > 0 && n.ID >= before {
			continue
		}
		out = append(out, *n)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *memNotificationRepo) MarkRead(_ context.Context, id int64, userID string) error {
	for _, n := range r.items {
		if n.ID == id && n.UserID == userID {
			n.IsRead = true
			return nil
		}
	}
	return domain.ErrNotFound
}

func (r *memNotificationRepo) MarkAllRead(_ context.Context, userID string) error {
	for _, n := range r.items {
		if n.UserID == userID {
			n.IsRead = true
		}
	}
	return nil
}

func (r *memNotificationRepo) CountUnread(_ context.Context, userID string) (int, error) {
	count := 0
	for _, n := range r.items {
		if n.UserID == userID && !n.IsRead {
			count++
		}
	}
	return count, nil
}

// newNotifSvc 构造 NotificationService + 内存 repo（broker=nil）。
func newNotifSvc(t *testing.T) (*NotificationService, *memNotificationRepo) {
	t.Helper()
	repo := newMemNotificationRepo()
	return NewNotificationService(repo, nil), repo
}

// TestNotification_Notify_Create 正常创建通知。
func TestNotification_Notify_Create(t *testing.T) {
	ctx := context.Background()
	svc, repo := newNotifSvc(t)

	err := svc.Notify(ctx, "u-1", "node.evaporated", map[string]any{"node_id": "n-1"})
	if err != nil {
		t.Fatalf("Notify error: %v", err)
	}
	if len(repo.items) != 1 {
		t.Fatalf("expected 1 notification in repo, got %d", len(repo.items))
	}
	if repo.items[0].Type != "node.evaporated" {
		t.Fatalf("wrong type: %s", repo.items[0].Type)
	}
	if repo.items[0].IsRead {
		t.Fatal("new notification should be unread")
	}
}

// TestNotification_List_Pagination 分页加载通知。
func TestNotification_List_Pagination(t *testing.T) {
	ctx := context.Background()
	svc, _ := newNotifSvc(t)
	actor := &domain.User{ID: "u-1"}

	// 创建 5 条通知
	for i := 0; i < 5; i++ {
		_ = svc.Notify(ctx, "u-1", "test.event", map[string]any{"i": i})
	}

	// 取前 3 条（最新3条）
	page1, err := svc.List(ctx, actor, 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(page1) != 3 {
		t.Fatalf("page1 expected 3, got %d", len(page1))
	}

	// cursor = page1 最后一条的 ID
	cursor := page1[len(page1)-1].ID
	page2, err := svc.List(ctx, actor, 3, cursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 2 {
		t.Fatalf("page2 expected 2, got %d", len(page2))
	}

	// page3 应为空（已全部取完）
	cursor2 := page2[len(page2)-1].ID
	page3, err := svc.List(ctx, actor, 3, cursor2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page3) != 0 {
		t.Fatalf("page3 expected 0, got %d", len(page3))
	}
}

// TestNotification_MarkRead 标记单条已读。
func TestNotification_MarkRead(t *testing.T) {
	ctx := context.Background()
	svc, repo := newNotifSvc(t)
	actor := &domain.User{ID: "u-1"}

	_ = svc.Notify(ctx, "u-1", "test", nil)
	id := repo.items[0].ID

	if err := svc.MarkRead(ctx, actor, id); err != nil {
		t.Fatalf("MarkRead error: %v", err)
	}
	if !repo.items[0].IsRead {
		t.Fatal("notification should be marked read")
	}
}

// TestNotification_MarkAllRead 全部已读。
func TestNotification_MarkAllRead(t *testing.T) {
	ctx := context.Background()
	svc, repo := newNotifSvc(t)
	actor := &domain.User{ID: "u-1"}

	for i := 0; i < 3; i++ {
		_ = svc.Notify(ctx, "u-1", "test", nil)
	}

	if err := svc.MarkAllRead(ctx, actor); err != nil {
		t.Fatal(err)
	}
	for _, n := range repo.items {
		if !n.IsRead {
			t.Errorf("notification %d should be read", n.ID)
		}
	}
}

// TestNotification_CountUnread 未读数量正确。
func TestNotification_CountUnread(t *testing.T) {
	ctx := context.Background()
	svc, repo := newNotifSvc(t)
	actor := &domain.User{ID: "u-1"}

	for i := 0; i < 4; i++ {
		_ = svc.Notify(ctx, "u-1", "test", nil)
	}
	// 标记 2 条已读
	_ = repo.MarkRead(ctx, repo.items[0].ID, "u-1")
	_ = repo.MarkRead(ctx, repo.items[1].ID, "u-1")

	count, err := svc.CountUnread(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 unread, got %d", count)
	}
}

// TestNotification_IsolatedByUser 不同用户通知互不干扰。
func TestNotification_IsolatedByUser(t *testing.T) {
	ctx := context.Background()
	svc, _ := newNotifSvc(t)

	_ = svc.Notify(ctx, "u-1", "test", nil)
	_ = svc.Notify(ctx, "u-2", "test", nil)
	_ = svc.Notify(ctx, "u-1", "test", nil)

	u1Notifs, err := svc.List(ctx, &domain.User{ID: "u-1"}, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(u1Notifs) != 2 {
		t.Fatalf("u-1 expected 2 notifications, got %d", len(u1Notifs))
	}

	u2Notifs, err := svc.List(ctx, &domain.User{ID: "u-2"}, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(u2Notifs) != 1 {
		t.Fatalf("u-2 expected 1 notification, got %d", len(u2Notifs))
	}
}

// TestNotification_NilPayload nil payload 写入不报错（转为 "{}"）。
func TestNotification_NilPayload(t *testing.T) {
	ctx := context.Background()
	svc, repo := newNotifSvc(t)

	if err := svc.Notify(ctx, "u-1", "test", nil); err != nil {
		t.Fatalf("nil payload error: %v", err)
	}
	if string(repo.items[0].Payload) != "{}" {
		t.Fatalf("expected '{}', got %q", string(repo.items[0].Payload))
	}
}

// 确保 memNotificationRepo 满足接口。
var _ store.NotificationRepository = (*memNotificationRepo)(nil)
