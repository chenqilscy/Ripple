package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PlatformAdminRepository interface {
	IsActive(ctx context.Context, userID string) (bool, error)
	GetActive(ctx context.Context, userID string) (*domain.PlatformAdmin, error)
	ListActive(ctx context.Context, limit int) ([]domain.PlatformAdmin, error)
	Grant(ctx context.Context, admin *domain.PlatformAdmin) error
	Revoke(ctx context.Context, userID string, revokedAt time.Time) error
}

type platformAdminRepoPG struct{ pool *pgxpool.Pool }

func NewPlatformAdminRepository(pool *pgxpool.Pool) PlatformAdminRepository {
	return &platformAdminRepoPG{pool: pool}
}

const sqlPlatformAdminIsActive = `
SELECT 1 FROM platform_admins WHERE user_id = $1::uuid AND revoked_at IS NULL
`

func (r *platformAdminRepoPG) IsActive(ctx context.Context, userID string) (bool, error) {
	if strings.TrimSpace(userID) == "" {
		return false, nil
	}
	var one int
	err := r.pool.QueryRow(ctx, sqlPlatformAdminIsActive, userID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("platform_admin active: %w", err)
	}
	return true, nil
}

const sqlPlatformAdminGetActive = `
SELECT user_id::text, role, note, coalesce(created_by::text, ''), created_at, revoked_at
FROM platform_admins
WHERE user_id = $1::uuid AND revoked_at IS NULL
`

func (r *platformAdminRepoPG) GetActive(ctx context.Context, userID string) (*domain.PlatformAdmin, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, domain.ErrNotFound
	}
	row := r.pool.QueryRow(ctx, sqlPlatformAdminGetActive, userID)
	return scanPlatformAdmin(row)
}

const sqlPlatformAdminListActive = `
SELECT user_id::text, role, note, coalesce(created_by::text, ''), created_at, revoked_at
FROM platform_admins
WHERE revoked_at IS NULL
ORDER BY created_at DESC
LIMIT $1
`

func (r *platformAdminRepoPG) ListActive(ctx context.Context, limit int) ([]domain.PlatformAdmin, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, sqlPlatformAdminListActive, limit)
	if err != nil {
		return nil, fmt.Errorf("platform_admin list active: %w", err)
	}
	defer rows.Close()
	out := make([]domain.PlatformAdmin, 0)
	for rows.Next() {
		admin, err := scanPlatformAdmin(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *admin)
	}
	return out, rows.Err()
}

const sqlPlatformAdminGrant = `
INSERT INTO platform_admins (user_id, role, note, created_by, created_at, revoked_at)
VALUES ($1::uuid, $2, $3, nullif($4, '')::uuid, $5, NULL)
ON CONFLICT (user_id) DO UPDATE SET
    role = EXCLUDED.role,
    note = EXCLUDED.note,
    created_by = EXCLUDED.created_by,
    created_at = EXCLUDED.created_at,
    revoked_at = NULL
`

func (r *platformAdminRepoPG) Grant(ctx context.Context, admin *domain.PlatformAdmin) error {
	if admin == nil || strings.TrimSpace(admin.UserID) == "" {
		return domain.ErrInvalidInput
	}
	role := admin.Role
	if role == "" {
		role = domain.PlatformAdminRoleAdmin
	}
	if !role.IsValid() {
		return domain.ErrInvalidInput
	}
	if admin.CreatedAt.IsZero() {
		admin.CreatedAt = time.Now().UTC()
	}
	if _, err := r.pool.Exec(ctx, sqlPlatformAdminGrant, admin.UserID, string(role), admin.Note, admin.CreatedBy, admin.CreatedAt); err != nil {
		return fmt.Errorf("platform_admin grant: %w", err)
	}
	return nil
}

const sqlPlatformAdminRevoke = `
UPDATE platform_admins SET revoked_at = $2 WHERE user_id = $1::uuid AND revoked_at IS NULL
`

func (r *platformAdminRepoPG) Revoke(ctx context.Context, userID string, revokedAt time.Time) error {
	if strings.TrimSpace(userID) == "" {
		return domain.ErrInvalidInput
	}
	if revokedAt.IsZero() {
		revokedAt = time.Now().UTC()
	}
	tag, err := r.pool.Exec(ctx, sqlPlatformAdminRevoke, userID, revokedAt)
	if err != nil {
		return fmt.Errorf("platform_admin revoke: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanPlatformAdmin(row pgx.Row) (*domain.PlatformAdmin, error) {
	var admin domain.PlatformAdmin
	var role string
	if err := row.Scan(&admin.UserID, &role, &admin.Note, &admin.CreatedBy, &admin.CreatedAt, &admin.RevokedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan platform_admin: %w", err)
	}
	admin.Role = domain.PlatformAdminRole(role)
	return &admin, nil
}
