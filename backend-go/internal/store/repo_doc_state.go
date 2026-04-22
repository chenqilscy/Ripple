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

// NodeDocStateRepository 节点 Y.Doc 快照存取（P8-A）。
type NodeDocStateRepository interface {
	// Get 返回节点快照；若无记录返回 nil, nil（而非 error）。
	Get(ctx context.Context, nodeID string) (*domain.NodeDocState, error)
	// Put 幂等写入或更新快照（乐观锁 version++）。
	// 大小限制由调用方在入口层（handler）校验，仓库不做 BYTEA 校验。
	Put(ctx context.Context, nodeID string, state []byte) error
}

type docStateRepoPG struct{ pool *pgxpool.Pool }

// NewNodeDocStateRepository 构造 PG 实现。
func NewNodeDocStateRepository(pool *pgxpool.Pool) NodeDocStateRepository {
	return &docStateRepoPG{pool: pool}
}

const sqlGetDocState = `
SELECT node_id, state, version, updated_at
FROM node_doc_states
WHERE node_id = $1
`

func (r *docStateRepoPG) Get(ctx context.Context, nodeID string) (*domain.NodeDocState, error) {
	row := r.pool.QueryRow(ctx, sqlGetDocState, nodeID)
	var d domain.NodeDocState
	if err := row.Scan(&d.NodeID, &d.State, &d.Version, &d.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // 正常：节点首次协作时无快照
		}
		return nil, fmt.Errorf("doc_state get %s: %w", nodeID, err)
	}
	return &d, nil
}

// sqlUpsertDocState 使用 UPSERT，version++ 保证乐观递增。
const sqlUpsertDocState = `
INSERT INTO node_doc_states (node_id, state, version, updated_at)
VALUES ($1, $2, 1, $3)
ON CONFLICT (node_id) DO UPDATE
  SET state      = EXCLUDED.state,
      version    = node_doc_states.version + 1,
      updated_at = EXCLUDED.updated_at
`

func (r *docStateRepoPG) Put(ctx context.Context, nodeID string, state []byte) error {
	_, err := r.pool.Exec(ctx, sqlUpsertDocState, nodeID, state, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("doc_state put %s: %w", nodeID, err)
	}
	return nil
}
