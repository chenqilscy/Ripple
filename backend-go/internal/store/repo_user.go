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

// UserRepository 用户读写。
type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
}

// userRepoPG 是 PG 实现。
type userRepoPG struct {
	pool *pgxpool.Pool
}

// NewUserRepository 创建 PG 用户仓库。
func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userRepoPG{pool: pool}
}

const sqlInsertUser = `
INSERT INTO users (id, email, password_hash, display_name, is_active, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

func (r *userRepoPG) Create(ctx context.Context, u *domain.User) error {
	_, err := r.pool.Exec(ctx, sqlInsertUser,
		u.ID, u.Email, u.PasswordHash, u.DisplayName, u.IsActive, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("user create: %w", err)
	}
	return nil
}

const sqlSelectUserByID = `
SELECT id, email, password_hash, display_name, is_active, created_at, updated_at
FROM users WHERE id = $1
`

func (r *userRepoPG) GetByID(ctx context.Context, id string) (*domain.User, error) {
	row := r.pool.QueryRow(ctx, sqlSelectUserByID, id)
	return scanUser(row)
}

const sqlSelectUserByEmail = `
SELECT id, email, password_hash, display_name, is_active, created_at, updated_at
FROM users WHERE email = $1
`

func (r *userRepoPG) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.pool.QueryRow(ctx, sqlSelectUserByEmail, email)
	return scanUser(row)
}

// scanUser 把单行扫描成 domain.User，未找到返回 ErrNotFound。
func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("user scan: %w", err)
	}
	return &u, nil
}
