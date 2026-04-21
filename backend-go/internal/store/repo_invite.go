package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InviteRepository 邀请令牌（存于 PG）。
//
// 约束：
//   - token 列 UNIQUE；并发 Create 冲突时返回 ErrAlreadyExists（但实际上 crypto/rand 冲突概率 ~0）。
//   - ConsumeByToken 使用乐观更新 `WHERE used_count < max_uses AND revoked_at IS NULL AND expires_at > now`
//     保证不会超额、不过期、未撤销；单 SQL RETURNING 保证原子。
type InviteRepository interface {
	Create(ctx context.Context, inv *domain.Invite) error
	GetByID(ctx context.Context, id string) (*domain.Invite, error)
	GetByToken(ctx context.Context, token string) (*domain.Invite, error)
	ListByLake(ctx context.Context, lakeID string, includeInactive bool) ([]domain.Invite, error)
	// ConsumeByToken 原子地把 used_count +1（要求 alive），返回消费后的邀请；
	// 若条件不满足返回 ErrNotFound（token 不存在 / 已撤销 / 已过期 / 已用完）。
	ConsumeByToken(ctx context.Context, token string, now time.Time) (*domain.Invite, error)
	Revoke(ctx context.Context, id string, when time.Time) error
}

type inviteRepoPG struct{ pool *pgxpool.Pool }

// NewInviteRepository 构造 PG 实现。
func NewInviteRepository(pool *pgxpool.Pool) InviteRepository {
	return &inviteRepoPG{pool: pool}
}

const sqlInsertInvite = `
INSERT INTO lake_invites
  (id, lake_id, token, created_by, role, max_uses, used_count, expires_at, revoked_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULL, $9)
`

func (r *inviteRepoPG) Create(ctx context.Context, inv *domain.Invite) error {
	_, err := r.pool.Exec(ctx, sqlInsertInvite,
		inv.ID, inv.LakeID, inv.Token, inv.CreatedBy, string(inv.Role),
		inv.MaxUses, inv.UsedCount, inv.ExpiresAt, inv.CreatedAt,
	)
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique violation (token)
			return domain.ErrAlreadyExists
		case "23503": // FK violation
			return fmt.Errorf("%w: user not found", domain.ErrInvalidInput)
		}
	}
	return fmt.Errorf("invite create: %w", err)
}

const sqlSelectInviteByID = `
SELECT id, lake_id, token, created_by, role, max_uses, used_count,
       expires_at, revoked_at, created_at
FROM lake_invites WHERE id = $1
`

func (r *inviteRepoPG) GetByID(ctx context.Context, id string) (*domain.Invite, error) {
	row := r.pool.QueryRow(ctx, sqlSelectInviteByID, id)
	return scanInvite(row)
}

const sqlSelectInviteByToken = `
SELECT id, lake_id, token, created_by, role, max_uses, used_count,
       expires_at, revoked_at, created_at
FROM lake_invites WHERE token = $1
`

func (r *inviteRepoPG) GetByToken(ctx context.Context, token string) (*domain.Invite, error) {
	row := r.pool.QueryRow(ctx, sqlSelectInviteByToken, token)
	return scanInvite(row)
}

const sqlListInvitesByLakeActive = `
SELECT id, lake_id, token, created_by, role, max_uses, used_count,
       expires_at, revoked_at, created_at
FROM lake_invites
WHERE lake_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC
`

const sqlListInvitesByLakeAll = `
SELECT id, lake_id, token, created_by, role, max_uses, used_count,
       expires_at, revoked_at, created_at
FROM lake_invites
WHERE lake_id = $1
ORDER BY created_at DESC
`

func (r *inviteRepoPG) ListByLake(ctx context.Context, lakeID string, includeInactive bool) ([]domain.Invite, error) {
	sqlStr := sqlListInvitesByLakeActive
	if includeInactive {
		sqlStr = sqlListInvitesByLakeAll
	}
	rows, err := r.pool.Query(ctx, sqlStr, lakeID)
	if err != nil {
		return nil, fmt.Errorf("invite list: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Invite, 0)
	for rows.Next() {
		inv, err := scanInviteRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *inv)
	}
	return out, rows.Err()
}

// sqlConsumeInvite 原子消费：只要满足所有条件就 used_count+1 并返回整行；不满足则 0 行。
const sqlConsumeInvite = `
UPDATE lake_invites
   SET used_count = used_count + 1
 WHERE token = $1
   AND revoked_at IS NULL
   AND expires_at > $2
   AND used_count < max_uses
RETURNING id, lake_id, token, created_by, role, max_uses, used_count,
          expires_at, revoked_at, created_at
`

func (r *inviteRepoPG) ConsumeByToken(ctx context.Context, token string, now time.Time) (*domain.Invite, error) {
	row := r.pool.QueryRow(ctx, sqlConsumeInvite, token, now)
	inv, err := scanInvite(row)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.ErrNotFound
	}
	return inv, err
}

const sqlRevokeInvite = `
UPDATE lake_invites SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL
`

func (r *inviteRepoPG) Revoke(ctx context.Context, id string, when time.Time) error {
	ct, err := r.pool.Exec(ctx, sqlRevokeInvite, id, when)
	if err != nil {
		return fmt.Errorf("invite revoke: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// --- 扫描工具 ---

type scanner interface {
	Scan(dest ...any) error
}

func scanInvite(s scanner) (*domain.Invite, error) {
	var inv domain.Invite
	var role string
	var revokedAt *time.Time
	err := s.Scan(&inv.ID, &inv.LakeID, &inv.Token, &inv.CreatedBy, &role,
		&inv.MaxUses, &inv.UsedCount, &inv.ExpiresAt, &revokedAt, &inv.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("invite scan: %w", err)
	}
	inv.Role = domain.Role(role)
	inv.RevokedAt = revokedAt
	return &inv, nil
}

func scanInviteRows(rows pgx.Rows) (*domain.Invite, error) {
	var inv domain.Invite
	var role string
	var revokedAt *time.Time
	if err := rows.Scan(&inv.ID, &inv.LakeID, &inv.Token, &inv.CreatedBy, &role,
		&inv.MaxUses, &inv.UsedCount, &inv.ExpiresAt, &revokedAt, &inv.CreatedAt); err != nil {
		return nil, fmt.Errorf("invite scan: %w", err)
	}
	inv.Role = domain.Role(role)
	inv.RevokedAt = revokedAt
	return &inv, nil
}
