package store

import (
	"context"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TagRepository P13-C：节点标签持久层。
type TagRepository interface {
	// SetTags 覆盖写 nodeID 在 lakeID 下的标签列表（先删再插）。
// 注：node_id 无 FK 约束（节点在 Neo4j），由调用方保证 nodeID 存在。
	SetTags(ctx context.Context, nodeID, lakeID string, tags []string) error
	// GetTags 返回单个节点的标签列表。
	GetTags(ctx context.Context, nodeID string) ([]string, error)
	// ListLakeTags 返回湖内所有已使用标签（去重，按字母排序）。
	ListLakeTags(ctx context.Context, lakeID string) ([]string, error)
	// ListNodesByTag 返回湖内带有指定标签的节点 ID 列表。
	ListNodesByTag(ctx context.Context, lakeID, tag string) ([]string, error)
}

// tagValidRe 只允许字母、数字、下划线、中划线、中文。
var tagValidRe = regexp.MustCompile(`^[\w\-\x{4e00}-\x{9fa5}]+$`)

// ValidTag 校验单个标签格式：非空，长度 ≤64，字符合法。
func ValidTag(t string) bool {
	return len(t) > 0 && len(t) <= 64 && tagValidRe.MatchString(t)
}

type pgTagRepository struct{ db *pgxpool.Pool }

// NewTagRepository 构造 PG 实现。
func NewTagRepository(db *pgxpool.Pool) TagRepository {
	return &pgTagRepository{db: db}
}

func (r *pgTagRepository) SetTags(ctx context.Context, nodeID, lakeID string, tags []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 删除旧标签
	_, err = tx.Exec(ctx, `DELETE FROM node_tags WHERE node_id = $1`, nodeID)
	if err != nil {
		return err
	}
	// 插入新标签
	for _, tag := range tags {
		if !ValidTag(tag) {
			continue // 忽略非法标签（由 service 层预校验）
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO node_tags (node_id, lake_id, tag) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			nodeID, lakeID, tag,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *pgTagRepository) GetTags(ctx context.Context, nodeID string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT tag FROM node_tags WHERE node_id = $1 ORDER BY tag`,
		nodeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := make([]string, 0)
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *pgTagRepository) ListLakeTags(ctx context.Context, lakeID string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT tag FROM node_tags WHERE lake_id = $1 ORDER BY tag`,
		lakeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := make([]string, 0)
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *pgTagRepository) ListNodesByTag(ctx context.Context, lakeID, tag string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT node_id FROM node_tags WHERE lake_id = $1 AND tag = $2`,
		lakeID, tag,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
