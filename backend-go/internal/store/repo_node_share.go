package store

import (
	"context"
	"errors"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NodeShareRepository 节点外链持久化（P18-B）。
type NodeShareRepository interface {
	Create(ctx context.Context, s *domain.NodeShare) error
	GetByToken(ctx context.Context, token string) (*domain.NodeShare, error)
	ListByNode(ctx context.Context, nodeID string) ([]domain.NodeShare, error)
	Revoke(ctx context.Context, id, userID string) error
}

type nodeShareRepoPG struct{ pool *pgxpool.Pool }

func NewNodeShareRepository(pool *pgxpool.Pool) NodeShareRepository {
	return &nodeShareRepoPG{pool: pool}
}

const sqlInsertNodeShare = `
INSERT INTO node_shares (id, node_id, token, expires_at, revoked, created_by, created_at)
VALUES ($1, $2, $3, $4, FALSE, $5, $6)
`

func (r *nodeShareRepoPG) Create(ctx context.Context, s *domain.NodeShare) error {
	var expiresAt interface{}
	if !s.ExpiresAt.IsZero() {
		expiresAt = s.ExpiresAt
	}
	_, err := r.pool.Exec(ctx, sqlInsertNodeShare,
		s.ID, s.NodeID, s.Token, expiresAt, s.CreatedBy, s.CreatedAt)
	return err
}

const sqlGetShareByToken = `
SELECT id, node_id, token, COALESCE(expires_at, '0001-01-01'::timestamptz), revoked, created_by, created_at
FROM node_shares WHERE token = $1
`

func (r *nodeShareRepoPG) GetByToken(ctx context.Context, token string) (*domain.NodeShare, error) {
	var s domain.NodeShare
	err := r.pool.QueryRow(ctx, sqlGetShareByToken, token).Scan(
		&s.ID, &s.NodeID, &s.Token, &s.ExpiresAt, &s.Revoked, &s.CreatedBy, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return &s, err
}

const sqlListSharesByNode = `
SELECT id, node_id, token, COALESCE(expires_at, '0001-01-01'::timestamptz), revoked, created_by, created_at
FROM node_shares WHERE node_id = $1
ORDER BY created_at DESC
`

func (r *nodeShareRepoPG) ListByNode(ctx context.Context, nodeID string) ([]domain.NodeShare, error) {
	rows, err := r.pool.Query(ctx, sqlListSharesByNode, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.NodeShare
	for rows.Next() {
		var s domain.NodeShare
		if err := rows.Scan(&s.ID, &s.NodeID, &s.Token, &s.ExpiresAt, &s.Revoked, &s.CreatedBy, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

const sqlRevokeShare = `
UPDATE node_shares SET revoked = TRUE WHERE id = $1 AND created_by = $2
`

func (r *nodeShareRepoPG) Revoke(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx, sqlRevokeShare, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
