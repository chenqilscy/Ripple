package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PromptTemplateRepository 是 Prompt 模板库的持久化接口（Phase 15-C）。
type PromptTemplateRepository interface {
	Create(ctx context.Context, t domain.PromptTemplate) (*domain.PromptTemplate, error)
	GetByID(ctx context.Context, id string) (*domain.PromptTemplate, error)
	List(ctx context.Context, createdBy string, limit, offset int) ([]domain.PromptTemplate, int, error)
	Update(ctx context.Context, id string, u domain.PromptTemplateUpdate) error
	Delete(ctx context.Context, id string) error
}

type promptTemplateRepoPG struct{ pool *pgxpool.Pool }

// NewPromptTemplateRepository 构造。
func NewPromptTemplateRepository(pool *pgxpool.Pool) PromptTemplateRepository {
	return &promptTemplateRepoPG{pool: pool}
}

const sqlInsertPromptTemplate = `
INSERT INTO prompt_templates (id, name, description, template, scope, org_id, created_by, created_at, updated_at)
VALUES ($1::uuid, $2, $3, $4, $5, NULLIF($6,''), $7::uuid, $8, $8)
RETURNING id::text, name, description, template, scope, COALESCE(org_id,''),
          created_by::text, created_at, updated_at
`

func (r *promptTemplateRepoPG) Create(ctx context.Context, t domain.PromptTemplate) (*domain.PromptTemplate, error) {
	row := r.pool.QueryRow(ctx, sqlInsertPromptTemplate,
		t.ID, t.Name, t.Description, t.Template, string(t.Scope),
		t.OrgID, t.CreatedBy, t.CreatedAt,
	)
	result, err := scanPromptTemplate(row)
	if err != nil {
		return nil, fmt.Errorf("prompt_templates insert: %w", err)
	}
	return result, nil
}

const sqlGetPromptTemplateByID = `
SELECT id::text, name, description, template, scope, COALESCE(org_id,''),
       created_by::text, created_at, updated_at
FROM prompt_templates
WHERE id = $1::uuid
`

func (r *promptTemplateRepoPG) GetByID(ctx context.Context, id string) (*domain.PromptTemplate, error) {
	row := r.pool.QueryRow(ctx, sqlGetPromptTemplateByID, id)
	result, err := scanPromptTemplate(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return result, err
}

const sqlListPromptTemplates = `
SELECT id::text, name, description, template, scope, COALESCE(org_id,''),
       created_by::text, created_at, updated_at
FROM prompt_templates
WHERE created_by = $1::uuid
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
`

const sqlCountPromptTemplates = `
SELECT COUNT(*) FROM prompt_templates WHERE created_by = $1::uuid
`

func (r *promptTemplateRepoPG) List(ctx context.Context, createdBy string, limit, offset int) ([]domain.PromptTemplate, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, sqlCountPromptTemplates, createdBy).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("prompt_templates count: %w", err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	rows, err := r.pool.Query(ctx, sqlListPromptTemplates, createdBy, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("prompt_templates list: %w", err)
	}
	defer rows.Close()

	var out []domain.PromptTemplate
	for rows.Next() {
		t, err := scanPromptTemplateRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *t)
	}
	return out, total, rows.Err()
}

const sqlUpdatePromptTemplate = `
UPDATE prompt_templates
SET name        = COALESCE($2, name),
    description = COALESCE($3, description),
    template    = COALESCE($4, template),
    scope       = COALESCE($5, scope),
    updated_at  = NOW()
WHERE id = $1::uuid
`

func (r *promptTemplateRepoPG) Update(ctx context.Context, id string, u domain.PromptTemplateUpdate) error {
	var name, desc, tmpl, scope *string
	if u.Name != nil {
		name = u.Name
	}
	if u.Description != nil {
		desc = u.Description
	}
	if u.Template != nil {
		tmpl = u.Template
	}
	if u.Scope != nil {
		s := string(*u.Scope)
		scope = &s
	}
	tag, err := r.pool.Exec(ctx, sqlUpdatePromptTemplate, id, name, desc, tmpl, scope)
	if err != nil {
		return fmt.Errorf("prompt_templates update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

const sqlDeletePromptTemplate = `DELETE FROM prompt_templates WHERE id = $1::uuid`

func (r *promptTemplateRepoPG) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, sqlDeletePromptTemplate, id)
	if err != nil {
		return fmt.Errorf("prompt_templates delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanPromptTemplate(row pgx.Row) (*domain.PromptTemplate, error) {
	var t domain.PromptTemplate
	var scope string
	err := row.Scan(&t.ID, &t.Name, &t.Description, &t.Template, &scope,
		&t.OrgID, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	t.Scope = domain.PromptTemplateScope(scope)
	return &t, nil
}

func scanPromptTemplateRow(rows pgx.Rows) (*domain.PromptTemplate, error) {
	var t domain.PromptTemplate
	var scope string
	err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Template, &scope,
		&t.OrgID, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	t.Scope = domain.PromptTemplateScope(scope)
	return &t, nil
}

// ensure time.Time is used
var _ = time.Time{}
