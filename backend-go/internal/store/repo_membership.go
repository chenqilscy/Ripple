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

// MembershipRepository 湖泊成员关系（存于 PG，权威源）。
type MembershipRepository interface {
	Upsert(ctx context.Context, m *domain.LakeMembership) error
	UpsertInTx(ctx context.Context, tx pgx.Tx, m *domain.LakeMembership) error
	GetRole(ctx context.Context, userID, lakeID string) (domain.Role, error)
	ListLakesByUser(ctx context.Context, userID string) ([]string, error)
	ListMembers(ctx context.Context, lakeID string) ([]domain.LakeMembership, error)
}

type membershipRepoPG struct{ pool *pgxpool.Pool }

// NewMembershipRepository 构造 PG 实现。
func NewMembershipRepository(pool *pgxpool.Pool) MembershipRepository {
	return &membershipRepoPG{pool: pool}
}

const sqlUpsertMembership = `
INSERT INTO lake_memberships (user_id, lake_id, role, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, lake_id) DO UPDATE
  SET role = EXCLUDED.role, updated_at = EXCLUDED.updated_at
`

func (r *membershipRepoPG) Upsert(ctx context.Context, m *domain.LakeMembership) error {
	_, err := r.pool.Exec(ctx, sqlUpsertMembership,
		m.UserID, m.LakeID, string(m.Role), m.CreatedAt, m.UpdatedAt)
	return mapMembershipErr(err)
}

// UpsertInTx 在调用方事务内执行 upsert（saga 用）。
func (r *membershipRepoPG) UpsertInTx(ctx context.Context, tx pgx.Tx, m *domain.LakeMembership) error {
	_, err := tx.Exec(ctx, sqlUpsertMembership,
		m.UserID, m.LakeID, string(m.Role), m.CreatedAt, m.UpdatedAt)
	return mapMembershipErr(err)
}

func mapMembershipErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // FK 缺失
		return fmt.Errorf("%w: user not found", domain.ErrInvalidInput)
	}
	return fmt.Errorf("membership upsert: %w", err)
}

const sqlSelectRole = `
SELECT role FROM lake_memberships WHERE user_id = $1 AND lake_id = $2
`

func (r *membershipRepoPG) GetRole(ctx context.Context, userID, lakeID string) (domain.Role, error) {
	var role string
	err := r.pool.QueryRow(ctx, sqlSelectRole, userID, lakeID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("membership get: %w", err)
	}
	return domain.Role(role), nil
}

const sqlListLakesByUser = `
SELECT lake_id FROM lake_memberships WHERE user_id = $1 ORDER BY updated_at DESC
`

func (r *membershipRepoPG) ListLakesByUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, sqlListLakesByUser, userID)
	if err != nil {
		return nil, fmt.Errorf("membership list lakes: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

const sqlListMembers = `
SELECT user_id, lake_id, role, created_at, updated_at
FROM lake_memberships WHERE lake_id = $1 ORDER BY created_at ASC
`

func (r *membershipRepoPG) ListMembers(ctx context.Context, lakeID string) ([]domain.LakeMembership, error) {
	rows, err := r.pool.Query(ctx, sqlListMembers, lakeID)
	if err != nil {
		return nil, fmt.Errorf("membership list members: %w", err)
	}
	defer rows.Close()
	out := make([]domain.LakeMembership, 0)
	for rows.Next() {
		var m domain.LakeMembership
		var role string
		if err := rows.Scan(&m.UserID, &m.LakeID, &role, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.Role = domain.Role(role)
		out = append(out, m)
	}
	return out, rows.Err()
}
