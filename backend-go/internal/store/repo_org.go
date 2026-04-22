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

// OrgRepository 组织与组织成员仓储（P12-C）。
type OrgRepository interface {
	// 组织 CRUD
	Create(ctx context.Context, org *domain.Organization) error
	GetByID(ctx context.Context, id string) (*domain.Organization, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Organization, error)
	ListByUser(ctx context.Context, userID string) ([]domain.Organization, error)

	// 成员管理
	AddMember(ctx context.Context, m *domain.OrgMember) error
	GetMemberRole(ctx context.Context, orgID, userID string) (domain.OrgRole, error)
	ListMembers(ctx context.Context, orgID string) ([]domain.OrgMember, error)
	UpdateMemberRole(ctx context.Context, orgID, userID string, role domain.OrgRole) error
	RemoveMember(ctx context.Context, orgID, userID string) error
	CountOwners(ctx context.Context, orgID string) (int, error)
}

type orgRepoPG struct{ pool *pgxpool.Pool }

// NewOrgRepository 构造 PG 实现。
func NewOrgRepository(pool *pgxpool.Pool) OrgRepository {
	return &orgRepoPG{pool: pool}
}

// --- 组织 ---

const sqlInsertOrg = `
INSERT INTO organizations (id, name, slug, description, owner_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

const sqlInsertOrgMemberTx = `
INSERT INTO org_members (org_id, user_id, role, joined_at)
VALUES ($1, $2, $3, $4)
`

func (r *orgRepoPG) Create(ctx context.Context, org *domain.Organization) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("org create: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, sqlInsertOrg,
		org.ID, org.Name, org.Slug, org.Description, org.OwnerID, org.CreatedAt, org.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w: slug already taken", domain.ErrAlreadyExists)
		}
		return fmt.Errorf("org insert: %w", err)
	}

	// 自动插入创始人 OWNER 成员
	_, err = tx.Exec(ctx, sqlInsertOrgMemberTx, org.ID, org.OwnerID, string(domain.OrgRoleOwner), org.CreatedAt)
	if err != nil {
		return fmt.Errorf("org member insert: %w", err)
	}

	return tx.Commit(ctx)
}

const sqlGetOrgByID = `
SELECT id, name, slug, description, owner_id, created_at, updated_at
FROM organizations WHERE id = $1
`

func (r *orgRepoPG) GetByID(ctx context.Context, id string) (*domain.Organization, error) {
	row := r.pool.QueryRow(ctx, sqlGetOrgByID, id)
	return scanOrg(row)
}

const sqlGetOrgBySlug = `
SELECT id, name, slug, description, owner_id, created_at, updated_at
FROM organizations WHERE slug = $1
`

func (r *orgRepoPG) GetBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	row := r.pool.QueryRow(ctx, sqlGetOrgBySlug, slug)
	return scanOrg(row)
}

func scanOrg(row pgx.Row) (*domain.Organization, error) {
	var o domain.Organization
	if err := row.Scan(&o.ID, &o.Name, &o.Slug, &o.Description, &o.OwnerID, &o.CreatedAt, &o.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan org: %w", err)
	}
	return &o, nil
}

const sqlListOrgsByUser = `
SELECT o.id, o.name, o.slug, o.description, o.owner_id, o.created_at, o.updated_at
FROM organizations o
JOIN org_members m ON m.org_id = o.id
WHERE m.user_id = $1
ORDER BY o.created_at DESC
`

func (r *orgRepoPG) ListByUser(ctx context.Context, userID string) ([]domain.Organization, error) {
	rows, err := r.pool.Query(ctx, sqlListOrgsByUser, userID)
	if err != nil {
		return nil, fmt.Errorf("list orgs: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Organization, 0)
	for rows.Next() {
		var o domain.Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.Description, &o.OwnerID, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan org row: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// --- 成员 ---

const sqlInsertOrgMember = `
INSERT INTO org_members (org_id, user_id, role, joined_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (org_id, user_id) DO NOTHING
`

func (r *orgRepoPG) AddMember(ctx context.Context, m *domain.OrgMember) error {
	_, err := r.pool.Exec(ctx, sqlInsertOrgMember, m.OrgID, m.UserID, string(m.Role), m.JoinedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("%w: user not found", domain.ErrInvalidInput)
		}
		return fmt.Errorf("add org member: %w", err)
	}
	return nil
}

const sqlGetOrgMemberRole = `
SELECT role FROM org_members WHERE org_id = $1 AND user_id = $2
`

func (r *orgRepoPG) GetMemberRole(ctx context.Context, orgID, userID string) (domain.OrgRole, error) {
	var role string
	err := r.pool.QueryRow(ctx, sqlGetOrgMemberRole, orgID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrNotFound
		}
		return "", fmt.Errorf("get org member role: %w", err)
	}
	return domain.OrgRole(role), nil
}

const sqlListOrgMembers = `
SELECT org_id, user_id, role, joined_at
FROM org_members WHERE org_id = $1
ORDER BY joined_at ASC
`

func (r *orgRepoPG) ListMembers(ctx context.Context, orgID string) ([]domain.OrgMember, error) {
	rows, err := r.pool.Query(ctx, sqlListOrgMembers, orgID)
	if err != nil {
		return nil, fmt.Errorf("list org members: %w", err)
	}
	defer rows.Close()
	out := make([]domain.OrgMember, 0)
	for rows.Next() {
		var m domain.OrgMember
		var role string
		if err := rows.Scan(&m.OrgID, &m.UserID, &role, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan org member: %w", err)
		}
		m.Role = domain.OrgRole(role)
		out = append(out, m)
	}
	return out, rows.Err()
}

const sqlUpdateOrgMemberRole = `
UPDATE org_members SET role = $3 WHERE org_id = $1 AND user_id = $2
`

func (r *orgRepoPG) UpdateMemberRole(ctx context.Context, orgID, userID string, role domain.OrgRole) error {
	tag, err := r.pool.Exec(ctx, sqlUpdateOrgMemberRole, orgID, userID, string(role))
	if err != nil {
		return fmt.Errorf("update org member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

const sqlRemoveOrgMember = `
DELETE FROM org_members WHERE org_id = $1 AND user_id = $2
`

func (r *orgRepoPG) RemoveMember(ctx context.Context, orgID, userID string) error {
	tag, err := r.pool.Exec(ctx, sqlRemoveOrgMember, orgID, userID)
	if err != nil {
		return fmt.Errorf("remove org member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

const sqlCountOrgOwners = `
SELECT COUNT(*) FROM org_members WHERE org_id = $1 AND role = 'OWNER'
`

func (r *orgRepoPG) CountOwners(ctx context.Context, orgID string) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, sqlCountOrgOwners, orgID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count org owners: %w", err)
	}
	return n, nil
}

