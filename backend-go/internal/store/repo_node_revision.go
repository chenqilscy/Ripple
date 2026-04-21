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

// NodeRevisionRepository 节点编辑历史（存于 PG）。
//
// 约束：
//   - (node_id, rev_number) UNIQUE；rev_number 在单节点内单调递增（从 1 开始）。
//   - InsertNext 通过 "SELECT COALESCE(MAX(rev_number),0)+1" 计算下一个号；
//     在并发场景由 UNIQUE 约束兜底，冲突时重试一次（Service 层处理）。
//   - List 为时间倒序（最新在前）。
type NodeRevisionRepository interface {
	// InsertNext 追加下一条 revision；rev_number 由实现自动分配；返回落库后的完整实体。
	InsertNext(ctx context.Context, rev *domain.NodeRevision) error
	// GetByNodeAndRev 取单条 revision。找不到返回 ErrNotFound。
	GetByNodeAndRev(ctx context.Context, nodeID string, revNumber int) (*domain.NodeRevision, error)
	// ListByNode 按时间倒序列出 revisions。limit<=0 用默认值 50；limit>200 截断到 200。
	ListByNode(ctx context.Context, nodeID string, limit int) ([]domain.NodeRevision, error)
	// LatestRevNumber 返回节点最新 rev_number；无记录返回 0（不报错）。
	LatestRevNumber(ctx context.Context, nodeID string) (int, error)
}

type nodeRevRepoPG struct{ pool *pgxpool.Pool }

// NewNodeRevisionRepository 构造。
func NewNodeRevisionRepository(pool *pgxpool.Pool) NodeRevisionRepository {
	return &nodeRevRepoPG{pool: pool}
}

const sqlInsertNodeRevision = `
INSERT INTO node_revisions (id, node_id, rev_number, content, title, editor_id, edit_reason, created_at)
SELECT $1, $2,
       COALESCE((SELECT MAX(rev_number) FROM node_revisions WHERE node_id = $2), 0) + 1,
       $3, $4, $5, $6, $7
RETURNING rev_number
`

func (r *nodeRevRepoPG) InsertNext(ctx context.Context, rev *domain.NodeRevision) error {
	if rev.CreatedAt.IsZero() {
		rev.CreatedAt = time.Now().UTC()
	}
	row := r.pool.QueryRow(ctx, sqlInsertNodeRevision,
		rev.ID, rev.NodeID, rev.Content, rev.Title, rev.EditorID, rev.EditReason, rev.CreatedAt,
	)
	if err := row.Scan(&rev.RevNumber); err != nil {
		return fmt.Errorf("insert node revision: %w", err)
	}
	return nil
}

const sqlGetNodeRevision = `
SELECT id, node_id, rev_number, content, title, editor_id, edit_reason, created_at
FROM node_revisions
WHERE node_id = $1 AND rev_number = $2
`

func (r *nodeRevRepoPG) GetByNodeAndRev(ctx context.Context, nodeID string, revNumber int) (*domain.NodeRevision, error) {
	row := r.pool.QueryRow(ctx, sqlGetNodeRevision, nodeID, revNumber)
	rev := &domain.NodeRevision{}
	if err := row.Scan(&rev.ID, &rev.NodeID, &rev.RevNumber, &rev.Content, &rev.Title, &rev.EditorID, &rev.EditReason, &rev.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get node revision: %w", err)
	}
	return rev, nil
}

const sqlListNodeRevisions = `
SELECT id, node_id, rev_number, content, title, editor_id, edit_reason, created_at
FROM node_revisions
WHERE node_id = $1
ORDER BY rev_number DESC
LIMIT $2
`

func (r *nodeRevRepoPG) ListByNode(ctx context.Context, nodeID string, limit int) ([]domain.NodeRevision, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := r.pool.Query(ctx, sqlListNodeRevisions, nodeID, limit)
	if err != nil {
		return nil, fmt.Errorf("list node revisions: %w", err)
	}
	defer rows.Close()
	out := make([]domain.NodeRevision, 0)
	for rows.Next() {
		var rev domain.NodeRevision
		if err := rows.Scan(&rev.ID, &rev.NodeID, &rev.RevNumber, &rev.Content, &rev.Title, &rev.EditorID, &rev.EditReason, &rev.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan node revision: %w", err)
		}
		out = append(out, rev)
	}
	return out, rows.Err()
}

const sqlLatestRevNumber = `SELECT COALESCE(MAX(rev_number), 0) FROM node_revisions WHERE node_id = $1`

func (r *nodeRevRepoPG) LatestRevNumber(ctx context.Context, nodeID string) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, sqlLatestRevNumber, nodeID).Scan(&n); err != nil {
		return 0, fmt.Errorf("latest rev_number: %w", err)
	}
	return n, nil
}
