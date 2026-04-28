package store

import (
	"context"
	"errors"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LakeSnapshotRepository 图谱布局快照持久化（P18-D）。
type LakeSnapshotRepository interface {
	Create(ctx context.Context, s *domain.LakeSnapshot) error
	List(ctx context.Context, lakeID string, limit int) ([]domain.LakeSnapshot, error)
	Get(ctx context.Context, id string) (*domain.LakeSnapshot, error)
	Delete(ctx context.Context, id, userID string) error
}

type snapshotRepoPG struct{ pool *pgxpool.Pool }

func NewLakeSnapshotRepository(pool *pgxpool.Pool) LakeSnapshotRepository {
	return &snapshotRepoPG{pool: pool}
}

const sqlInsertSnapshot = `
INSERT INTO lake_snapshots (id, lake_id, name, layout, graph_state, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

func (r *snapshotRepoPG) Create(ctx context.Context, s *domain.LakeSnapshot) error {
	_, err := r.pool.Exec(ctx, sqlInsertSnapshot,
		s.ID, s.LakeID, s.Name, s.Layout, s.GraphState, s.CreatedBy, s.CreatedAt)
	return err
}

const sqlListSnapshots = `
SELECT id, lake_id, name, layout, created_by, created_at
FROM lake_snapshots
WHERE lake_id = $1
ORDER BY created_at DESC
LIMIT $2
`

func (r *snapshotRepoPG) List(ctx context.Context, lakeID string, limit int) ([]domain.LakeSnapshot, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, sqlListSnapshots, lakeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.LakeSnapshot
	for rows.Next() {
		var s domain.LakeSnapshot
		if err := rows.Scan(&s.ID, &s.LakeID, &s.Name, &s.Layout, &s.CreatedBy, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

const sqlGetSnapshotFull = `
SELECT id, lake_id, name, layout, graph_state, created_by, created_at
FROM lake_snapshots WHERE id = $1
`

const sqlGetSnapshot = `
SELECT id, lake_id, name, layout, created_by, created_at
FROM lake_snapshots WHERE id = $1
`

func (r *snapshotRepoPG) Get(ctx context.Context, id string) (*domain.LakeSnapshot, error) {
	var s domain.LakeSnapshot
	err := r.pool.QueryRow(ctx, sqlGetSnapshotFull, id).Scan(
		&s.ID, &s.LakeID, &s.Name, &s.Layout, &s.GraphState, &s.CreatedBy, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return &s, err
}

const sqlDeleteSnapshot = `
DELETE FROM lake_snapshots WHERE id = $1 AND created_by = $2
`

func (r *snapshotRepoPG) Delete(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx, sqlDeleteSnapshot, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
