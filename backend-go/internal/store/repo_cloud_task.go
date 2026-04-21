package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CloudTaskRepository 造云任务持久化。
type CloudTaskRepository interface {
	Create(ctx context.Context, t *domain.CloudTask) error
	GetByID(ctx context.Context, id string) (*domain.CloudTask, error)
	ListByOwner(ctx context.Context, ownerID string, limit int) ([]domain.CloudTask, error)
	// ClaimNext 原子取一条 queued 任务并置 running，返回任务；
	// 没有任务返回 (nil, ErrNotFound)。Worker 循环用。
	ClaimNext(ctx context.Context) (*domain.CloudTask, error)
	MarkDone(ctx context.Context, id string, nodeIDs []string) error
	MarkFailed(ctx context.Context, id string, reason string) error
	// RecoverRunning 进程启动时把"卡住的 running"重置为 queued，避免任务丢失。
	RecoverRunning(ctx context.Context) (int64, error)
}

type cloudTaskRepoPG struct{ pool *pgxpool.Pool }

// NewCloudTaskRepository 构造。
func NewCloudTaskRepository(pool *pgxpool.Pool) CloudTaskRepository {
	return &cloudTaskRepoPG{pool: pool}
}

const sqlInsertCloudTask = `
INSERT INTO cloud_tasks (id, owner_id, lake_id, prompt, n, node_type, status, created_at)
VALUES ($1, $2, NULLIF($3,'')::uuid, $4, $5, $6, 'queued', $7)
`

func (r *cloudTaskRepoPG) Create(ctx context.Context, t *domain.CloudTask) error {
	_, err := r.pool.Exec(ctx, sqlInsertCloudTask,
		t.ID, t.OwnerID, t.LakeID, t.Prompt, t.N, string(t.NodeType), t.CreatedAt)
	if err != nil {
		return fmt.Errorf("cloud task insert: %w", err)
	}
	return nil
}

const sqlGetCloudTask = `
SELECT id, owner_id, COALESCE(lake_id::text,''), prompt, n, node_type, status,
       retry_count, COALESCE(last_error,''), result_node_ids::text,
       created_at, started_at, completed_at
FROM cloud_tasks WHERE id = $1
`

func (r *cloudTaskRepoPG) GetByID(ctx context.Context, id string) (*domain.CloudTask, error) {
	var t domain.CloudTask
	var nodeType, statusStr, resultJSON string
	row := r.pool.QueryRow(ctx, sqlGetCloudTask, id)
	if err := row.Scan(&t.ID, &t.OwnerID, &t.LakeID, &t.Prompt, &t.N, &nodeType, &statusStr,
		&t.RetryCount, &t.LastError, &resultJSON, &t.CreatedAt, &t.StartedAt, &t.CompletedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("cloud task get: %w", err)
	}
	t.NodeType = domain.NodeType(nodeType)
	t.Status = domain.CloudTaskStatus(statusStr)
	_ = json.Unmarshal([]byte(resultJSON), &t.ResultNodeIDs)
	if t.ResultNodeIDs == nil {
		t.ResultNodeIDs = []string{}
	}
	return &t, nil
}

const sqlListCloudByOwner = `
SELECT id, owner_id, COALESCE(lake_id::text,''), prompt, n, node_type, status,
       retry_count, COALESCE(last_error,''), result_node_ids::text,
       created_at, started_at, completed_at
FROM cloud_tasks WHERE owner_id = $1
ORDER BY created_at DESC LIMIT $2
`

func (r *cloudTaskRepoPG) ListByOwner(ctx context.Context, ownerID string, limit int) ([]domain.CloudTask, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, sqlListCloudByOwner, ownerID, limit)
	if err != nil {
		return nil, fmt.Errorf("cloud task list: %w", err)
	}
	defer rows.Close()
	out := []domain.CloudTask{}
	for rows.Next() {
		var t domain.CloudTask
		var nodeType, statusStr, resultJSON string
		if err := rows.Scan(&t.ID, &t.OwnerID, &t.LakeID, &t.Prompt, &t.N, &nodeType, &statusStr,
			&t.RetryCount, &t.LastError, &resultJSON, &t.CreatedAt, &t.StartedAt, &t.CompletedAt); err != nil {
			return nil, err
		}
		t.NodeType = domain.NodeType(nodeType)
		t.Status = domain.CloudTaskStatus(statusStr)
		_ = json.Unmarshal([]byte(resultJSON), &t.ResultNodeIDs)
		if t.ResultNodeIDs == nil {
			t.ResultNodeIDs = []string{}
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ClaimNext 用 SELECT … FOR UPDATE SKIP LOCKED 拿一条 queued。
const sqlClaimNext = `
UPDATE cloud_tasks
SET status = 'running', started_at = NOW()
WHERE id = (
  SELECT id FROM cloud_tasks
  WHERE status = 'queued'
  ORDER BY created_at ASC
  LIMIT 1
  FOR UPDATE SKIP LOCKED
)
RETURNING id, owner_id, COALESCE(lake_id::text,''), prompt, n, node_type, status,
          retry_count, COALESCE(last_error,''), result_node_ids::text,
          created_at, started_at, completed_at
`

func (r *cloudTaskRepoPG) ClaimNext(ctx context.Context) (*domain.CloudTask, error) {
	var t domain.CloudTask
	var nodeType, statusStr, resultJSON string
	row := r.pool.QueryRow(ctx, sqlClaimNext)
	if err := row.Scan(&t.ID, &t.OwnerID, &t.LakeID, &t.Prompt, &t.N, &nodeType, &statusStr,
		&t.RetryCount, &t.LastError, &resultJSON, &t.CreatedAt, &t.StartedAt, &t.CompletedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("cloud task claim: %w", err)
	}
	t.NodeType = domain.NodeType(nodeType)
	t.Status = domain.CloudTaskStatus(statusStr)
	_ = json.Unmarshal([]byte(resultJSON), &t.ResultNodeIDs)
	if t.ResultNodeIDs == nil {
		t.ResultNodeIDs = []string{}
	}
	return &t, nil
}

const sqlMarkCloudDone = `
UPDATE cloud_tasks
SET status='done', completed_at=NOW(), result_node_ids=$2::jsonb
WHERE id=$1
`

func (r *cloudTaskRepoPG) MarkDone(ctx context.Context, id string, nodeIDs []string) error {
	if nodeIDs == nil {
		nodeIDs = []string{}
	}
	b, _ := json.Marshal(nodeIDs)
	_, err := r.pool.Exec(ctx, sqlMarkCloudDone, id, string(b))
	if err != nil {
		return fmt.Errorf("cloud task done: %w", err)
	}
	return nil
}

const sqlMarkCloudFailed = `
UPDATE cloud_tasks
SET status='failed', last_error=$2, retry_count=retry_count+1, completed_at=NOW()
WHERE id=$1
`

func (r *cloudTaskRepoPG) MarkFailed(ctx context.Context, id string, reason string) error {
	_, err := r.pool.Exec(ctx, sqlMarkCloudFailed, id, reason)
	if err != nil {
		return fmt.Errorf("cloud task fail: %w", err)
	}
	return nil
}

const sqlRecoverRunning = `
UPDATE cloud_tasks SET status='queued', started_at=NULL
WHERE status='running'
`

func (r *cloudTaskRepoPG) RecoverRunning(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, sqlRecoverRunning)
	if err != nil {
		return 0, fmt.Errorf("cloud task recover: %w", err)
	}
	return tag.RowsAffected(), nil
}
