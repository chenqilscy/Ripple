// Package store · SpaceRepository（M3-S1）。
//
// Space 与 SpaceMember 都存 PG，以保证强一致性（配额 + 成员变更需事务）。
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

// SpaceRepository 工作空间仓储。
type SpaceRepository interface {
	Create(ctx context.Context, s *domain.Space) error
	GetByID(ctx context.Context, id string) (*domain.Space, error)
	UpdateMeta(ctx context.Context, id, name, description string) error
	Delete(ctx context.Context, id string) error

	// 成员
	UpsertMember(ctx context.Context, m *domain.SpaceMember) error
	RemoveMember(ctx context.Context, spaceID, userID string) error
	GetMemberRole(ctx context.Context, spaceID, userID string) (domain.SpaceRole, error)
	ListMembers(ctx context.Context, spaceID string) ([]domain.SpaceMember, error)
	ListSpacesByUser(ctx context.Context, userID string) ([]domain.Space, error)
}

type spaceRepoPG struct{ pool *pgxpool.Pool }

// NewSpaceRepository 构造 PG 实现。
func NewSpaceRepository(pool *pgxpool.Pool) SpaceRepository {
	return &spaceRepoPG{pool: pool}
}

const sqlInsertSpace = `
INSERT INTO spaces (id, owner_id, name, description, llm_quota_monthly, llm_used_current_month, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`

func (r *spaceRepoPG) Create(ctx context.Context, s *domain.Space) error {
	_, err := r.pool.Exec(ctx, sqlInsertSpace,
		s.ID, s.OwnerID, s.Name, s.Description,
		s.LLMQuotaMonthly, s.LLMUsedCurrentMonth,
		s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("%w: owner not found", domain.ErrInvalidInput)
		}
		return fmt.Errorf("space create: %w", err)
	}
	return nil
}

const sqlGetSpace = `
SELECT id, owner_id, name, description, llm_quota_monthly, llm_used_current_month, created_at, updated_at
FROM spaces WHERE id = $1
`

func (r *spaceRepoPG) GetByID(ctx context.Context, id string) (*domain.Space, error) {
	s := &domain.Space{}
	err := r.pool.QueryRow(ctx, sqlGetSpace, id).Scan(
		&s.ID, &s.OwnerID, &s.Name, &s.Description,
		&s.LLMQuotaMonthly, &s.LLMUsedCurrentMonth,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("space get: %w", err)
	}
	return s, nil
}

const sqlUpdateSpaceMeta = `
UPDATE spaces SET name = $2, description = $3 WHERE id = $1
`

func (r *spaceRepoPG) UpdateMeta(ctx context.Context, id, name, description string) error {
	tag, err := r.pool.Exec(ctx, sqlUpdateSpaceMeta, id, name, description)
	if err != nil {
		return fmt.Errorf("space update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

const sqlDeleteSpace = `DELETE FROM spaces WHERE id = $1`

func (r *spaceRepoPG) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, sqlDeleteSpace, id)
	if err != nil {
		return fmt.Errorf("space delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

const sqlUpsertSpaceMember = `
INSERT INTO space_members (space_id, user_id, role, joined_at, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (space_id, user_id) DO UPDATE
  SET role = EXCLUDED.role, updated_at = EXCLUDED.updated_at
`

func (r *spaceRepoPG) UpsertMember(ctx context.Context, m *domain.SpaceMember) error {
	_, err := r.pool.Exec(ctx, sqlUpsertSpaceMember,
		m.SpaceID, m.UserID, string(m.Role), m.JoinedAt, m.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("%w: space or user not found", domain.ErrInvalidInput)
		}
		return fmt.Errorf("space member upsert: %w", err)
	}
	return nil
}

const sqlDeleteSpaceMember = `DELETE FROM space_members WHERE space_id = $1 AND user_id = $2`

func (r *spaceRepoPG) RemoveMember(ctx context.Context, spaceID, userID string) error {
	tag, err := r.pool.Exec(ctx, sqlDeleteSpaceMember, spaceID, userID)
	if err != nil {
		return fmt.Errorf("space member delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

const sqlGetSpaceMemberRole = `
SELECT role FROM space_members WHERE space_id = $1 AND user_id = $2
`

func (r *spaceRepoPG) GetMemberRole(ctx context.Context, spaceID, userID string) (domain.SpaceRole, error) {
	var role string
	err := r.pool.QueryRow(ctx, sqlGetSpaceMemberRole, spaceID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("space member role: %w", err)
	}
	return domain.SpaceRole(role), nil
}

const sqlListSpaceMembers = `
SELECT space_id, user_id, role, joined_at, updated_at
FROM space_members WHERE space_id = $1 ORDER BY joined_at ASC
`

func (r *spaceRepoPG) ListMembers(ctx context.Context, spaceID string) ([]domain.SpaceMember, error) {
	rows, err := r.pool.Query(ctx, sqlListSpaceMembers, spaceID)
	if err != nil {
		return nil, fmt.Errorf("space list members: %w", err)
	}
	defer rows.Close()
	out := make([]domain.SpaceMember, 0)
	for rows.Next() {
		var m domain.SpaceMember
		var role string
		if err := rows.Scan(&m.SpaceID, &m.UserID, &role, &m.JoinedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.Role = domain.SpaceRole(role)
		out = append(out, m)
	}
	return out, rows.Err()
}

const sqlListSpacesByUser = `
SELECT s.id, s.owner_id, s.name, s.description, s.llm_quota_monthly, s.llm_used_current_month, s.created_at, s.updated_at
FROM spaces s
JOIN space_members m ON m.space_id = s.id
WHERE m.user_id = $1
ORDER BY s.updated_at DESC
`

func (r *spaceRepoPG) ListSpacesByUser(ctx context.Context, userID string) ([]domain.Space, error) {
	rows, err := r.pool.Query(ctx, sqlListSpacesByUser, userID)
	if err != nil {
		return nil, fmt.Errorf("space list by user: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Space, 0)
	for rows.Next() {
		var s domain.Space
		if err := rows.Scan(&s.ID, &s.OwnerID, &s.Name, &s.Description,
			&s.LLMQuotaMonthly, &s.LLMUsedCurrentMonth,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
