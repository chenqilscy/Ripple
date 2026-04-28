package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GraylistRepository 提供灰度准入邮箱名单读写。
type GraylistRepository interface {
	List(ctx context.Context, limit int) ([]domain.GraylistEntry, error)
	Upsert(ctx context.Context, entry *domain.GraylistEntry) (*domain.GraylistEntry, error)
	Delete(ctx context.Context, id string) error
	IsAllowedEmail(ctx context.Context, email string) (bool, error)
}

type graylistRepoPG struct{ pool *pgxpool.Pool }

// NewGraylistRepository 创建 PG 灰度名单仓库。
func NewGraylistRepository(pool *pgxpool.Pool) GraylistRepository {
	return &graylistRepoPG{pool: pool}
}

const sqlListGraylistEntries = `
SELECT id, email, note, created_by, created_at
FROM graylist_entries
ORDER BY created_at DESC
LIMIT $1
`

func (r *graylistRepoPG) List(ctx context.Context, limit int) ([]domain.GraylistEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, sqlListGraylistEntries, limit)
	if err != nil {
		return nil, fmt.Errorf("graylist list: %w", err)
	}
	defer rows.Close()
	out := make([]domain.GraylistEntry, 0)
	for rows.Next() {
		var entry domain.GraylistEntry
		if err := rows.Scan(&entry.ID, &entry.Email, &entry.Note, &entry.CreatedBy, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("graylist scan: %w", err)
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

const sqlUpsertGraylistEntry = `
INSERT INTO graylist_entries (id, email, note, created_by, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (email) DO UPDATE
SET note = EXCLUDED.note,
    created_by = EXCLUDED.created_by,
    created_at = EXCLUDED.created_at
RETURNING id, email, note, created_by, created_at
`

func (r *graylistRepoPG) Upsert(ctx context.Context, entry *domain.GraylistEntry) (*domain.GraylistEntry, error) {
	row := r.pool.QueryRow(ctx, sqlUpsertGraylistEntry,
		entry.ID,
		strings.ToLower(strings.TrimSpace(entry.Email)),
		entry.Note,
		entry.CreatedBy,
		entry.CreatedAt,
	)
	var out domain.GraylistEntry
	if err := row.Scan(&out.ID, &out.Email, &out.Note, &out.CreatedBy, &out.CreatedAt); err != nil {
		return nil, fmt.Errorf("graylist upsert: %w", err)
	}
	return &out, nil
}

const sqlDeleteGraylistEntry = `
DELETE FROM graylist_entries WHERE id = $1
`

func (r *graylistRepoPG) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, sqlDeleteGraylistEntry, id)
	if err != nil {
		return fmt.Errorf("graylist delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

const sqlGraylistAllowedEmail = `
SELECT 1 FROM graylist_entries WHERE email = $1
`

const sqlCountGraylistEntries = `
SELECT COUNT(*) FROM graylist_entries
`

func (r *graylistRepoPG) IsAllowedEmail(ctx context.Context, email string) (bool, error) {
	var matched int
	err := r.pool.QueryRow(ctx, sqlGraylistAllowedEmail, strings.ToLower(strings.TrimSpace(email))).Scan(&matched)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("graylist allowed email: %w", err)
	}
	return true, nil
}

func (r *graylistRepoPG) CountAll(ctx context.Context) (int64, error) {
	var n int64
	if err := r.pool.QueryRow(ctx, sqlCountGraylistEntries).Scan(&n); err != nil {
		return 0, fmt.Errorf("graylist count: %w", err)
	}
	return n, nil
}