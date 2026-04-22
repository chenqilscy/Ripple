package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PermaNodeRepository 索引表 CRUD（核心读路径）。
type PermaNodeRepository interface {
	Create(ctx context.Context, p *domain.PermaNode) error
	GetByID(ctx context.Context, id string) (*domain.PermaNode, error)
	ListByLake(ctx context.Context, lakeID string, limit int) ([]domain.PermaNode, error)
	// ListIDsByLakes 返回这些 lake 下最近的 perma_node id（推荐"同空间信号"用）。
	ListIDsByLakes(ctx context.Context, lakeIDs []string, limit int) ([]string, error)
}

type permaRepoPG struct{ pool *pgxpool.Pool }

// NewPermaNodeRepository 构造 PG 实现。
func NewPermaNodeRepository(pool *pgxpool.Pool) PermaNodeRepository {
	return &permaRepoPG{pool: pool}
}

const sqlInsertPerma = `
INSERT INTO perma_nodes (id, lake_id, owner_id, title, summary, source_node_ids, llm_provider, llm_cost_tokens, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
`

func (r *permaRepoPG) Create(ctx context.Context, p *domain.PermaNode) error {
	_, err := r.pool.Exec(ctx, sqlInsertPerma,
		p.ID, p.LakeID, p.OwnerID, p.Title, p.Summary,
		p.SourceNodeIDs, p.LLMProvider, p.LLMCostTokens,
		p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23503" || pgErr.Code == "23505" {
				return fmt.Errorf("%w: %s", domain.ErrInvalidInput, pgErr.Message)
			}
		}
		return fmt.Errorf("perma create: %w", err)
	}
	return nil
}

const sqlGetPerma = `
SELECT id, lake_id, owner_id, title, summary, source_node_ids, llm_provider, llm_cost_tokens, created_at, updated_at
FROM perma_nodes WHERE id = $1
`

func (r *permaRepoPG) GetByID(ctx context.Context, id string) (*domain.PermaNode, error) {
	row := r.pool.QueryRow(ctx, sqlGetPerma, id)
	var p domain.PermaNode
	if err := row.Scan(&p.ID, &p.LakeID, &p.OwnerID, &p.Title, &p.Summary,
		&p.SourceNodeIDs, &p.LLMProvider, &p.LLMCostTokens, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("perma get: %w", err)
	}
	return &p, nil
}

const sqlListPermaByLake = `
SELECT id, lake_id, owner_id, title, summary, source_node_ids, llm_provider, llm_cost_tokens, created_at, updated_at
FROM perma_nodes WHERE lake_id = $1
ORDER BY created_at DESC LIMIT $2
`

func (r *permaRepoPG) ListByLake(ctx context.Context, lakeID string, limit int) ([]domain.PermaNode, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, sqlListPermaByLake, lakeID, limit)
	if err != nil {
		return nil, fmt.Errorf("perma list: %w", err)
	}
	defer rows.Close()
	out := make([]domain.PermaNode, 0)
	for rows.Next() {
		var p domain.PermaNode
		if err := rows.Scan(&p.ID, &p.LakeID, &p.OwnerID, &p.Title, &p.Summary,
			&p.SourceNodeIDs, &p.LLMProvider, &p.LLMCostTokens, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("perma list scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

const sqlListPermaIDsByLakes = `
SELECT id FROM perma_nodes WHERE lake_id = ANY($1)
ORDER BY created_at DESC LIMIT $2
`

func (r *permaRepoPG) ListIDsByLakes(ctx context.Context, lakeIDs []string, limit int) ([]string, error) {
	if len(lakeIDs) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, sqlListPermaIDsByLakes, lakeIDs, limit)
	if err != nil {
		return nil, fmt.Errorf("perma list-ids-by-lakes: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
