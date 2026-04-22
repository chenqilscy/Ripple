package service

import (
	"context"
	"encoding/json"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// NotificationService P13-B：通知服务。
type NotificationService struct {
	repo store.NotificationRepository
}

// NewNotificationService 构造。
func NewNotificationService(repo store.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

// Notify 内部 API：为 userID 创建一条通知（供其他 service 调用）。
// notifType 示例："lake_invite", "member_removed"
func (s *NotificationService) Notify(ctx context.Context, userID, notifType string, payload map[string]any) error {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		raw = b
	} else {
		raw = json.RawMessage("{}")
	}
	_, err := s.repo.Create(ctx, userID, notifType, raw)
	return err
}

// List 返回 actor 自己的通知列表（分页）。
func (s *NotificationService) List(ctx context.Context, actor *domain.User, limit int, before int64) ([]store.Notification, error) {
	return s.repo.ListForUser(ctx, actor.ID, limit, before)
}

// MarkRead 标记单条通知已读（仅自己的）。
func (s *NotificationService) MarkRead(ctx context.Context, actor *domain.User, id int64) error {
	return s.repo.MarkRead(ctx, id, actor.ID)
}

// MarkAllRead 标记所有通知已读。
func (s *NotificationService) MarkAllRead(ctx context.Context, actor *domain.User) error {
	return s.repo.MarkAllRead(ctx, actor.ID)
}

// CountUnread 返回未读通知数量。
func (s *NotificationService) CountUnread(ctx context.Context, actor *domain.User) (int, error) {
	return s.repo.CountUnread(ctx, actor.ID)
}
