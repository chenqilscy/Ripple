package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AiJobRepository 是 AI 节点任务持久化接口（Phase 15-C）。
type AiJobRepository interface {
	// CreateWithConflictCheck 插入一个 pending 任务。
	// 若 (node_id) 上已有 pending/processing 任务（部分唯一索引冲突），
	// 返回 (nil, false, nil)；其他错误返回 (nil, false, err)。
	CreateWithConflictCheck(ctx context.Context, job domain.AiJob) (*domain.AiJob, bool, error)

	// GetByNodeID 获取该节点最新一条任务（按 created_at DESC）。
	GetByNodeID(ctx context.Context, nodeID string) (*domain.AiJob, error)

	// GetByID 按 id 精确查询。
	GetByID(ctx context.Context, id string) (*domain.AiJob, error)

	// ListPending 取 limit 条 pending 任务（FOR UPDATE SKIP LOCKED），供 worker 消费。
	ListPending(ctx context.Context, limit int) ([]domain.AiJob, error)

	// UpdateStatus 更新 status + 相关时间戳字段。
	UpdateStatus(ctx context.Context, id string, status domain.AiJobStatus, progressPct int, errMsg string) error

	// RecoverProcessing 启动时把"卡住的 processing"重置为 pending。
	RecoverProcessing(ctx context.Context) (int64, error)
}

type aiJobRepoPG struct{ pool *pgxpool.Pool }

// NewAiJobRepository 构造。
func NewAiJobRepository(pool *pgxpool.Pool) AiJobRepository {
	return &aiJobRepoPG{pool: pool}
}

const sqlInsertAiJob = `
INSERT INTO ai_jobs (id, node_id, lake_id, prompt_template_id, status, progress_pct,
                     input_node_ids, override_vars, created_by, created_at)
VALUES ($1, $2, $3, NULLIF($4,'')::uuid, 'pending', 0, $5, $6, $7::uuid, $8)
ON CONFLICT (node_id) WHERE status IN ('pending', 'processing')
DO NOTHING
RETURNING id, node_id, lake_id, COALESCE(prompt_template_id::text,''), status,
          progress_pct, input_node_ids, override_vars,
          started_at, finished_at, error, created_by::text, created_at
`

func (r *aiJobRepoPG) CreateWithConflictCheck(ctx context.Context, job domain.AiJob) (*domain.AiJob, bool, error) {
	varsJSON, err := json.Marshal(job.OverrideVars)
	if err != nil {
		return nil, false, fmt.Errorf("ai_jobs: marshal override_vars: %w", err)
	}
	row := r.pool.QueryRow(ctx, sqlInsertAiJob,
		job.ID, job.NodeID, job.LakeID, job.PromptTemplateID,
		job.InputNodeIDs, varsJSON,
		job.CreatedBy, job.CreatedAt,
	)
	result, err := scanAiJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		// ON CONFLICT DO NOTHING → 已存在活跃任务
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("ai_jobs insert: %w", err)
	}
	return result, true, nil
}

const sqlGetAiJobByNode = `
SELECT id, node_id, lake_id, COALESCE(prompt_template_id::text,''), status,
       progress_pct, input_node_ids, override_vars,
       started_at, finished_at, error, created_by::text, created_at
FROM ai_jobs
WHERE node_id = $1
ORDER BY created_at DESC
LIMIT 1
`

func (r *aiJobRepoPG) GetByNodeID(ctx context.Context, nodeID string) (*domain.AiJob, error) {
	row := r.pool.QueryRow(ctx, sqlGetAiJobByNode, nodeID)
	job, err := scanAiJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return job, err
}

const sqlGetAiJobByID = `
SELECT id, node_id, lake_id, COALESCE(prompt_template_id::text,''), status,
       progress_pct, input_node_ids, override_vars,
       started_at, finished_at, error, created_by::text, created_at
FROM ai_jobs
WHERE id = $1::uuid
`

func (r *aiJobRepoPG) GetByID(ctx context.Context, id string) (*domain.AiJob, error) {
	row := r.pool.QueryRow(ctx, sqlGetAiJobByID, id)
	job, err := scanAiJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return job, err
}

const sqlListPendingAiJobs = `
SELECT id, node_id, lake_id, COALESCE(prompt_template_id::text,''), status,
       progress_pct, input_node_ids, override_vars,
       started_at, finished_at, error, created_by::text, created_at
FROM ai_jobs
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED
`

func (r *aiJobRepoPG) ListPending(ctx context.Context, limit int) ([]domain.AiJob, error) {
	rows, err := r.pool.Query(ctx, sqlListPendingAiJobs, limit)
	if err != nil {
		return nil, fmt.Errorf("ai_jobs list pending: %w", err)
	}
	defer rows.Close()

	var jobs []domain.AiJob
	for rows.Next() {
		job, err := scanAiJobRow(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}
	return jobs, rows.Err()
}

const sqlUpdateAiJobStatus = `
UPDATE ai_jobs
SET status       = $2,
    progress_pct = $3,
    error        = $4,
    started_at   = CASE WHEN $2 = 'processing' AND started_at IS NULL THEN NOW() ELSE started_at END,
    finished_at  = CASE WHEN $2 IN ('done','failed') THEN NOW() ELSE finished_at END
WHERE id = $1::uuid
`

func (r *aiJobRepoPG) UpdateStatus(ctx context.Context, id string, status domain.AiJobStatus, progressPct int, errMsg string) error {
	_, err := r.pool.Exec(ctx, sqlUpdateAiJobStatus, id, string(status), progressPct, errMsg)
	if err != nil {
		return fmt.Errorf("ai_jobs update status: %w", err)
	}
	return nil
}

const sqlRecoverProcessingAiJobs = `
UPDATE ai_jobs
SET status = 'pending', started_at = NULL
WHERE status = 'processing'
`

func (r *aiJobRepoPG) RecoverProcessing(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, sqlRecoverProcessingAiJobs)
	if err != nil {
		return 0, fmt.Errorf("ai_jobs recover: %w", err)
	}
	return tag.RowsAffected(), nil
}

// scanAiJob 扫描 QueryRow 结果。
func scanAiJob(row pgx.Row) (*domain.AiJob, error) {
	var job domain.AiJob
	var statusStr string
	var inputNodeIDsArr []string
	var varsJSON []byte

	err := row.Scan(
		&job.ID, &job.NodeID, &job.LakeID, &job.PromptTemplateID, &statusStr,
		&job.ProgressPct, &inputNodeIDsArr, &varsJSON,
		&job.StartedAt, &job.FinishedAt, &job.Error, &job.CreatedBy, &job.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	job.Status = domain.AiJobStatus(statusStr)
	job.InputNodeIDs = inputNodeIDsArr
	if len(varsJSON) > 0 {
		_ = json.Unmarshal(varsJSON, &job.OverrideVars)
	}
	if job.OverrideVars == nil {
		job.OverrideVars = map[string]string{}
	}
	return &job, nil
}

// scanAiJobRow 扫描 Rows 结果（字段顺序与 scanAiJob 相同）。
func scanAiJobRow(rows pgx.Rows) (*domain.AiJob, error) {
	var job domain.AiJob
	var statusStr string
	var inputNodeIDsArr []string
	var varsJSON []byte

	err := rows.Scan(
		&job.ID, &job.NodeID, &job.LakeID, &job.PromptTemplateID, &statusStr,
		&job.ProgressPct, &inputNodeIDsArr, &varsJSON,
		&job.StartedAt, &job.FinishedAt, &job.Error, &job.CreatedBy, &job.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	job.Status = domain.AiJobStatus(statusStr)
	job.InputNodeIDs = inputNodeIDsArr
	if len(varsJSON) > 0 {
		_ = json.Unmarshal(varsJSON, &job.OverrideVars)
	}
	if job.OverrideVars == nil {
		job.OverrideVars = map[string]string{}
	}
	return &job, nil
}

// ensure time.Time is used
var _ = time.Time{}
