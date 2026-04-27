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

// OrgQuotaRepository 组织配额仓储（P14-A）。
type OrgQuotaRepository interface {
	EnsureDefault(ctx context.Context, orgID string) (*domain.OrgQuota, error)
	GetByOrgID(ctx context.Context, orgID string) (*domain.OrgQuota, error)
	Update(ctx context.Context, quota *domain.OrgQuota) error
}

type orgQuotaRepoPG struct{ pool *pgxpool.Pool }

// NewOrgQuotaRepository 构造 PG 实现。
func NewOrgQuotaRepository(pool *pgxpool.Pool) OrgQuotaRepository {
	return &orgQuotaRepoPG{pool: pool}
}

const sqlEnsureDefaultOrgQuota = `
INSERT INTO org_quotas (org_id)
VALUES ($1)
ON CONFLICT (org_id) DO NOTHING
`

func (r *orgQuotaRepoPG) EnsureDefault(ctx context.Context, orgID string) (*domain.OrgQuota, error) {
	quota, err := r.GetByOrgID(ctx, orgID)
	if err == nil {
		return quota, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if _, err := r.pool.Exec(ctx, sqlEnsureDefaultOrgQuota, orgID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("org quota ensure default: %w", err)
	}
	return r.GetByOrgID(ctx, orgID)
}

const sqlGetOrgQuota = `
SELECT org_id, max_members, max_lakes, max_nodes, max_attachments, max_api_keys, max_storage_mb, created_at, updated_at
FROM org_quotas WHERE org_id = $1
`

func (r *orgQuotaRepoPG) GetByOrgID(ctx context.Context, orgID string) (*domain.OrgQuota, error) {
	row := r.pool.QueryRow(ctx, sqlGetOrgQuota, orgID)
	return scanOrgQuota(row)
}

func scanOrgQuota(row pgx.Row) (*domain.OrgQuota, error) {
	var q domain.OrgQuota
	if err := row.Scan(
		&q.OrgID,
		&q.MaxMembers,
		&q.MaxLakes,
		&q.MaxNodes,
		&q.MaxAttachments,
		&q.MaxAPIKeys,
		&q.MaxStorageMB,
		&q.CreatedAt,
		&q.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan org quota: %w", err)
	}
	return &q, nil
}

const sqlUpdateOrgQuota = `
INSERT INTO org_quotas (
    org_id, max_members, max_lakes, max_nodes, max_attachments, max_api_keys, max_storage_mb, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (org_id) DO UPDATE SET
    max_members = EXCLUDED.max_members,
    max_lakes = EXCLUDED.max_lakes,
    max_nodes = EXCLUDED.max_nodes,
    max_attachments = EXCLUDED.max_attachments,
    max_api_keys = EXCLUDED.max_api_keys,
    max_storage_mb = EXCLUDED.max_storage_mb,
    updated_at = EXCLUDED.updated_at
`

func (r *orgQuotaRepoPG) Update(ctx context.Context, quota *domain.OrgQuota) error {
	_, err := r.pool.Exec(ctx, sqlUpdateOrgQuota,
		quota.OrgID,
		quota.MaxMembers,
		quota.MaxLakes,
		quota.MaxNodes,
		quota.MaxAttachments,
		quota.MaxAPIKeys,
		quota.MaxStorageMB,
		quota.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return domain.ErrNotFound
		}
		return fmt.Errorf("org quota update: %w", err)
	}
	return nil
}
