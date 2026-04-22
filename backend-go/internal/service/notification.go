package service

import (
	"context"
	"encoding/json"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/realtime"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// NotificationService P13-B/P14-A：通知服务。
type NotificationService struct {
	repo   store.NotificationRepository
	broker realtime.Broker // P14-A：可空；非空时实时推送
}

// NewNotificationService 构造。broker 可为 nil（降级为纯轮询模式）。
func NewNotificationService(repo store.NotificationRepository, broker realtime.Broker) *NotificationService {
	return &NotificationService{repo: repo, broker: broker}
}

// Notify 内部 API：为 userID 创建一条通知，并通过 Broker 实时推送（P14-A）。
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
	n, err := s.repo.Create(ctx, userID, notifType, raw)
	if err != nil {
		return err
	}
	// P14-A：非阻塞推送；Broker 失败不影响通知持久化。
	if s.broker != nil {
		msg := realtime.Message{
			Type: "notification.new",
			Payload: map[string]any{
				"id":         n.ID,
				"type":       n.Type,
				"payload":    payload,
				"created_at": n.CreatedAt,
			},
		}
		_ = s.broker.Publish(ctx, realtime.UserTopic(userID), msg)
	}
	return nil
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
