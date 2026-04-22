package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FeedbackRepository 反馈事件读写（M3-S3 推荐器骨架最小集）。
type FeedbackRepository interface {
	// AddEvent 写入一条反馈事件。
	AddEvent(ctx context.Context, ev FeedbackEvent) error
	// ListUserPositiveTargets 返回用户曾经 LIKE 过的 target_id（按 target_type 过滤）。
	ListUserPositiveTargets(ctx context.Context, userID, targetType string, limit int) ([]string, error)
	// ListUsersWhoLiked 返回 LIKE 过指定 target 的用户 ID。
	ListUsersWhoLiked(ctx context.Context, targetType, targetID string, limit int) ([]string, error)
	// ListLikedByUsers 返回这些用户合计 LIKE 过的 target_id（按出现次数倒序），可排除某些已知 ID。
	ListLikedByUsers(ctx context.Context, userIDs []string, targetType string, exclude []string, limit int) ([]TargetCount, error)
}

// FeedbackEvent 一条反馈事件（写入用）。
type FeedbackEvent struct {
	ID         string // 调用方生成 UUID
	UserID     string
	TargetType string
	TargetID   string
	EventType  string // "LIKE" / "DISLIKE" / "RARE" / ...
	Payload    string // JSON 字符串；空字符串视为 '{}'
}

// TargetCount 推荐结果：(target_id, like_count)
type TargetCount struct {
	TargetID string
	Count    int64
}

type feedbackRepoPG struct{ pool *pgxpool.Pool }

// NewFeedbackRepository 构造 PG 实现。
func NewFeedbackRepository(pool *pgxpool.Pool) FeedbackRepository {
	return &feedbackRepoPG{pool: pool}
}

func (r *feedbackRepoPG) AddEvent(ctx context.Context, ev FeedbackEvent) error {
	payload := ev.Payload
	if payload == "" {
		payload = "{}"
	}
	_, err := r.pool.Exec(ctx, `
INSERT INTO feedback_events(id, user_id, target_type, target_id, event_type, payload)
VALUES ($1, $2, $3, $4, $5, $6::jsonb)
`, ev.ID, ev.UserID, ev.TargetType, ev.TargetID, ev.EventType, payload)
	if err != nil {
		return fmt.Errorf("feedback insert: %w", err)
	}
	return nil
}

func (r *feedbackRepoPG) ListUserPositiveTargets(ctx context.Context, userID, targetType string, limit int) ([]string, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := r.pool.Query(ctx, `
SELECT DISTINCT target_id::text
FROM feedback_events
WHERE user_id = $1 AND target_type = $2 AND event_type = 'LIKE'
ORDER BY 1
LIMIT $3
`, userID, targetType, limit)
	if err != nil {
		return nil, fmt.Errorf("feedback list user pos: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *feedbackRepoPG) ListUsersWhoLiked(ctx context.Context, targetType, targetID string, limit int) ([]string, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := r.pool.Query(ctx, `
SELECT DISTINCT user_id::text
FROM feedback_events
WHERE target_type = $1 AND target_id = $2 AND event_type = 'LIKE'
LIMIT $3
`, targetType, targetID, limit)
	if err != nil {
		return nil, fmt.Errorf("feedback list users-liked: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *feedbackRepoPG) ListLikedByUsers(ctx context.Context, userIDs []string, targetType string, exclude []string, limit int) ([]TargetCount, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	// 用 ANY($1::uuid[]) 避免拼接
	rows, err := r.pool.Query(ctx, `
SELECT target_id::text, COUNT(*) AS cnt
FROM feedback_events
WHERE user_id = ANY($1::uuid[])
  AND target_type = $2
  AND event_type = 'LIKE'
  AND ($3::uuid[] IS NULL OR NOT (target_id = ANY($3::uuid[])))
GROUP BY target_id
ORDER BY cnt DESC, target_id
LIMIT $4
`, userIDs, targetType, nullableUUIDArray(exclude), limit)
	if err != nil {
		return nil, fmt.Errorf("feedback list liked-by-users: %w", err)
	}
	defer rows.Close()
	out := make([]TargetCount, 0, limit)
	for rows.Next() {
		var tc TargetCount
		if err := rows.Scan(&tc.TargetID, &tc.Count); err != nil {
			return nil, err
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}

// nullableUUIDArray 空切片返回 nil（让 SQL 走 NULL 分支）。
func nullableUUIDArray(s []string) any {
	if len(s) == 0 {
		return nil
	}
	return s
}
