package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OutboxEvent 出站事件记录。ID 由 DB 自增（BIGSERIAL）。
type OutboxEvent struct {
	ID         int64
	EventType  string
	Payload    []byte // JSON
	Status     string // pending | processing | done | failed
	RetryCount int
	LastError  string
	CreatedAt  time.Time
}

// OutboxRepository 用于跨库 saga。所有写入必须在 PG 事务内，
// 读取由 dispatcher 周期轮询。
type OutboxRepository interface {
	EnqueueInTx(ctx context.Context, tx pgx.Tx, eventType string, payload []byte) error
	Dequeue(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkDone(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, reason string) error
}

type outboxRepoPG struct{ pool *pgxpool.Pool }

// NewOutboxRepository 创建 PG 实现。
func NewOutboxRepository(pool *pgxpool.Pool) OutboxRepository {
	return &outboxRepoPG{pool: pool}
}

const sqlEnqueueOutbox = `
INSERT INTO outbox_events (event_type, payload, status)
VALUES ($1, $2::jsonb, 'pending')
`

func (r *outboxRepoPG) EnqueueInTx(ctx context.Context, tx pgx.Tx, eventType string, payload []byte) error {
	_, err := tx.Exec(ctx, sqlEnqueueOutbox, eventType, string(payload))
	if err != nil {
		return fmt.Errorf("outbox enqueue: %w", err)
	}
	return nil
}

// 并发安全的 dequeue：原子置 processing。
const sqlDequeue = `
UPDATE outbox_events
SET status = 'processing', updated_at = NOW()
WHERE id IN (
  SELECT id FROM outbox_events
  WHERE status IN ('pending','failed')
  ORDER BY created_at ASC
  LIMIT $1
  FOR UPDATE SKIP LOCKED
)
RETURNING id, event_type, payload::text, status, retry_count, COALESCE(last_error, ''), created_at
`

func (r *outboxRepoPG) Dequeue(ctx context.Context, limit int) ([]OutboxEvent, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.pool.Query(ctx, sqlDequeue, limit)
	if err != nil {
		return nil, fmt.Errorf("outbox dequeue: %w", err)
	}
	defer rows.Close()
	out := make([]OutboxEvent, 0)
	for rows.Next() {
		var ev OutboxEvent
		var payload string
		if err := rows.Scan(&ev.ID, &ev.EventType, &payload, &ev.Status,
			&ev.RetryCount, &ev.LastError, &ev.CreatedAt); err != nil {
			return nil, err
		}
		ev.Payload = []byte(payload)
		out = append(out, ev)
	}
	return out, rows.Err()
}

const sqlMarkDone = `
UPDATE outbox_events SET status = 'done', processed_at = NOW() WHERE id = $1
`

func (r *outboxRepoPG) MarkDone(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, sqlMarkDone, id)
	if err != nil {
		return fmt.Errorf("outbox mark done: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

const sqlMarkFailed = `
UPDATE outbox_events
SET status = 'failed', retry_count = retry_count + 1, last_error = $2, updated_at = NOW()
WHERE id = $1
`

func (r *outboxRepoPG) MarkFailed(ctx context.Context, id int64, reason string) error {
	_, err := r.pool.Exec(ctx, sqlMarkFailed, id, reason)
	if err != nil {
		return fmt.Errorf("outbox mark failed: %w", err)
	}
	return nil
}

// ErrOutboxNotPending 指示处理时状态已不是 pending。
var ErrOutboxNotPending = errors.New("outbox event not pending")
