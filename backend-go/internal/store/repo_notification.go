package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Notification P13-B：用户通知条目。
type Notification struct {
	ID        int64
	UserID    string
	Type      string
	Payload   json.RawMessage
	IsRead    bool
	CreatedAt time.Time
}

// NotificationRepository P13-B：通知存储接口。
type NotificationRepository interface {
	// Create 写入一条通知。
	Create(ctx context.Context, userID, notifType string, payload json.RawMessage) (*Notification, error)
	// ListForUser 分页获取指定用户的通知（id DESC，before=0 表示从最新开始）。
	ListForUser(ctx context.Context, userID string, limit int, before int64) ([]Notification, error)
	// MarkRead 将指定 id 的通知标记为已读（只能操作自己的通知）。
	MarkRead(ctx context.Context, id int64, userID string) error
	// MarkAllRead 将指定用户所有未读通知标记为已读。
	MarkAllRead(ctx context.Context, userID string) error
	// CountUnread 返回指定用户的未读通知数量。
	CountUnread(ctx context.Context, userID string) (int, error)
}

type notificationRepoPG struct {
	db *pgxpool.Pool
}

// NewNotificationRepository 构造 PG 实现。
func NewNotificationRepository(db *pgxpool.Pool) NotificationRepository {
	return &notificationRepoPG{db: db}
}

func (r *notificationRepoPG) Create(ctx context.Context, userID, notifType string, payload json.RawMessage) (*Notification, error) {
	if payload == nil {
		payload = json.RawMessage("{}")
	}
	row := r.db.QueryRow(ctx,
		`INSERT INTO notifications (user_id, type, payload) VALUES ($1, $2, $3)
		 RETURNING id, user_id, type, payload, is_read, created_at`,
		userID, notifType, payload,
	)
	return scanNotification(row)
}

func (r *notificationRepoPG) ListForUser(ctx context.Context, userID string, limit int, before int64) ([]Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	if before <= 0 {
		rs, err := r.db.Query(ctx,
			`SELECT id, user_id, type, payload, is_read, created_at
			 FROM notifications WHERE user_id = $1
			 ORDER BY id DESC LIMIT $2`,
			userID, limit,
		)
		if err != nil {
			return nil, err
		}
		defer rs.Close()
		return scanNotifications(rs)
	}

	rs, err := r.db.Query(ctx,
		`SELECT id, user_id, type, payload, is_read, created_at
		 FROM notifications WHERE user_id = $1 AND id < $2
		 ORDER BY id DESC LIMIT $3`,
		userID, before, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	return scanNotifications(rs)
}

func (r *notificationRepoPG) MarkRead(ctx context.Context, id int64, userID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE notifications SET is_read = TRUE WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	return err
}

func (r *notificationRepoPG) MarkAllRead(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE notifications SET is_read = TRUE WHERE user_id = $1 AND is_read = FALSE`,
		userID,
	)
	return err
}

func (r *notificationRepoPG) CountUnread(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE`,
		userID,
	).Scan(&count)
	return count, err
}

// --- helpers ---

type pgRow interface {
	Scan(dest ...any) error
}

func scanNotification(row pgRow) (*Notification, error) {
	var n Notification
	var payload []byte
	if err := row.Scan(&n.ID, &n.UserID, &n.Type, &payload, &n.IsRead, &n.CreatedAt); err != nil {
		return nil, err
	}
	n.Payload = json.RawMessage(payload)
	return &n, nil
}

type pgRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanNotifications(rows pgRows) ([]Notification, error) {
	out := make([]Notification, 0)
	for rows.Next() {
		var n Notification
		var payload []byte
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &payload, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, err
		}
		n.Payload = json.RawMessage(payload)
		out = append(out, n)
	}
	return out, rows.Err()
}
