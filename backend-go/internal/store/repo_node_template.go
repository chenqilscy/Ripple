package store

import (
	"context"
	"errors"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NodeTemplateRepository 节点模板持久化。
type NodeTemplateRepository interface {
	// List 返回内置模板 + 该用户创建的模板。
	List(ctx context.Context, userID string) ([]domain.NodeTemplate, error)
	// Get 返回单个模板（任何人可读内置模板；用户只能读自己的）。
	Get(ctx context.Context, id, userID string) (*domain.NodeTemplate, error)
	// Create 创建用户模板。
	Create(ctx context.Context, t *domain.NodeTemplate) error
	// Delete 删除用户模板（系统模板不可删）。
	Delete(ctx context.Context, id, userID string) error
}

type nodeTemplateRepoPG struct{ pool *pgxpool.Pool }

// NewNodeTemplateRepository 构造 PG 实现。
func NewNodeTemplateRepository(pool *pgxpool.Pool) NodeTemplateRepository {
	return &nodeTemplateRepoPG{pool: pool}
}

const sqlListTemplates = `
SELECT id, name, description, content, tags, created_by, is_system, created_at, updated_at
FROM node_templates
WHERE is_system = TRUE OR created_by = $1
ORDER BY is_system DESC, created_at DESC
`

func (r *nodeTemplateRepoPG) List(ctx context.Context, userID string) ([]domain.NodeTemplate, error) {
	rows, err := r.pool.Query(ctx, sqlListTemplates, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.NodeTemplate
	for rows.Next() {
		var t domain.NodeTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Content,
			&t.Tags, &t.CreatedBy, &t.IsSystem, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

const sqlGetTemplate = `
SELECT id, name, description, content, tags, created_by, is_system, created_at, updated_at
FROM node_templates
WHERE id = $1 AND (is_system = TRUE OR created_by = $2)
`

func (r *nodeTemplateRepoPG) Get(ctx context.Context, id, userID string) (*domain.NodeTemplate, error) {
	var t domain.NodeTemplate
	err := r.pool.QueryRow(ctx, sqlGetTemplate, id, userID).Scan(
		&t.ID, &t.Name, &t.Description, &t.Content,
		&t.Tags, &t.CreatedBy, &t.IsSystem, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return &t, err
}

const sqlInsertTemplate = `
INSERT INTO node_templates (id, name, description, content, tags, created_by, is_system, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, FALSE, $7, $8)
`

func (r *nodeTemplateRepoPG) Create(ctx context.Context, t *domain.NodeTemplate) error {
	_, err := r.pool.Exec(ctx, sqlInsertTemplate,
		t.ID, t.Name, t.Description, t.Content, t.Tags, t.CreatedBy, t.CreatedAt, t.UpdatedAt)
	return err
}

const sqlDeleteTemplate = `
DELETE FROM node_templates WHERE id = $1 AND created_by = $2 AND is_system = FALSE
`

func (r *nodeTemplateRepoPG) Delete(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx, sqlDeleteTemplate, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// EnsureSystemTemplates 检查并插入内置模板（幂等）。
// 在 main.go 启动时调用一次即可。
func EnsureSystemTemplates(ctx context.Context, pool *pgxpool.Pool) error {
	type tpl struct {
		id, name, desc, content string
		tags                    []string
	}
	builtins := []tpl{
		{
			id:      "tpl-meeting-notes",
			name:    "会议纪要",
			desc:    "结构化会议记录模板",
			content: "## 会议纪要\n\n**时间**：\n**参与人**：\n\n### 议题\n\n1. \n\n### 决议\n\n- \n\n### 行动项\n\n| 负责人 | 事项 | 截止日期 |\n|--------|------|----------|\n| | | |",
			tags:    []string{"会议", "纪要"},
		},
		{
			id:      "tpl-requirement",
			name:    "需求卡片",
			desc:    "功能需求描述模板",
			content: "## 需求：{标题}\n\n**优先级**：P0 / P1 / P2\n**状态**：待评审\n\n### 背景\n\n### 目标用户\n\n### 核心需求\n\n### 验收标准\n\n- [ ] \n",
			tags:    []string{"需求", "产品"},
		},
		{
			id:      "tpl-idea",
			name:    "灵感闪记",
			desc:    "快速记录一闪而过的想法",
			content: "💡 **想法**：\n\n**来源**：\n**潜在价值**：\n**下一步**：",
			tags:    []string{"灵感", "想法"},
		},
	}

	now := time.Now()
	for _, b := range builtins {
		_, err := pool.Exec(ctx, `
INSERT INTO node_templates (id, name, description, content, tags, created_by, is_system, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, '', TRUE, $6, $6)
ON CONFLICT (id) DO NOTHING
`, b.id, b.name, b.desc, b.content, b.tags, now)
		if err != nil {
			return err
		}
	}
	return nil
}
