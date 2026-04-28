package store

import (
	"context"
	"fmt"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// LakeRepository Lake 实体在 Neo4j 的读写。
type LakeRepository interface {
	Create(ctx context.Context, l *domain.Lake) error
	GetByID(ctx context.Context, id string) (*domain.Lake, error)
	// GetManyByIDs 批量查询，返回按输入顺序（缺失的 ID 跳过，不报错）。
	// 用于 ListMine 等需要一次性取多个湖的场景，避免 N+1。
	GetManyByIDs(ctx context.Context, ids []string) ([]domain.Lake, error)
	// UpdateSpaceID 修改湖归属的 space_id（''=移到个人湖）。返回更新后的 Lake。
	UpdateSpaceID(ctx context.Context, id, spaceID string) (*domain.Lake, error)
	// UpdateOrgID 修改湖归属的组织（P13-A）。
	UpdateOrgID(ctx context.Context, id, orgID string) (*domain.Lake, error)
	// ListByOrg 查询某组织下所有湖（P13-A）。
	ListByOrg(ctx context.Context, orgID string) ([]domain.Lake, error)
}

type lakeRepoNeo struct {
	driver neo4j.DriverWithContext
	dbName string
}

// NewLakeRepository 构造 Neo4j 实现。
func NewLakeRepository(driver neo4j.DriverWithContext, dbName string) LakeRepository {
	return &lakeRepoNeo{driver: driver, dbName: dbName}
}

const cypherCreateLake = `
CREATE (l:Lake {
  id: $id, name: $name, description: $desc, is_public: $is_public,
  owner_id: $owner_id, space_id: $space_id, org_id: $org_id, created_at: $created_at, updated_at: $updated_at
})
RETURN l.id AS id
`

func (r *lakeRepoNeo) Create(ctx context.Context, l *domain.Lake) error {
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()

	_, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, cypherCreateLake, map[string]any{
			"id":         l.ID,
			"name":       l.Name,
			"desc":       l.Description,
			"is_public":  l.IsPublic,
			"owner_id":   l.OwnerID,
			"space_id":   l.SpaceID,
			"org_id":     l.OrgID,
			"created_at": l.CreatedAt.UTC().Format(time.RFC3339Nano),
			"updated_at": l.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("lake create: %w", err)
	}
	return nil
}

const cypherGetLake = `
MATCH (l:Lake {id: $id})
RETURN l.id, l.name, l.description, l.is_public, l.owner_id, coalesce(l.space_id, '') AS space_id, coalesce(l.org_id, '') AS org_id, l.created_at, l.updated_at
`

func (r *lakeRepoNeo) GetByID(ctx context.Context, id string) (*domain.Lake, error) {
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()

	out, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		rec, err := tx.Run(ctx, cypherGetLake, map[string]any{"id": id})
		if err != nil {
			return nil, err
		}
		if !rec.Next(ctx) {
			return nil, domain.ErrNotFound
		}
		vals := rec.Record().Values
		l := &domain.Lake{
			ID:          asString(vals[0]),
			Name:        asString(vals[1]),
			Description: asString(vals[2]),
			IsPublic:    asBool(vals[3]),
			OwnerID:     asString(vals[4]),
			SpaceID:     asString(vals[5]),
			OrgID:       asString(vals[6]),
			CreatedAt:   parseTime(asString(vals[7])),
			UpdatedAt:   parseTime(asString(vals[8])),
		}
		return l, nil
	})
	if err != nil {
		return nil, err
	}
	return out.(*domain.Lake), nil
}

const cypherGetManyLakes = `
MATCH (l:Lake) WHERE l.id IN $ids
RETURN l.id, l.name, l.description, l.is_public, l.owner_id, coalesce(l.space_id, '') AS space_id, coalesce(l.org_id, '') AS org_id, l.created_at, l.updated_at
`

// GetManyByIDs 单次 Cypher 查询。结果顺序由 Neo4j 决定；调用方若需固定顺序可自行排序。
func (r *lakeRepoNeo) GetManyByIDs(ctx context.Context, ids []string) ([]domain.Lake, error) {
	if len(ids) == 0 {
		return []domain.Lake{}, nil
	}
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()

	out, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		rec, err := tx.Run(ctx, cypherGetManyLakes, map[string]any{"ids": ids})
		if err != nil {
			return nil, err
		}
		list := make([]domain.Lake, 0, len(ids))
		for rec.Next(ctx) {
			vals := rec.Record().Values
			list = append(list, domain.Lake{
				ID:          asString(vals[0]),
				Name:        asString(vals[1]),
				Description: asString(vals[2]),
				IsPublic:    asBool(vals[3]),
				OwnerID:     asString(vals[4]),
				SpaceID:     asString(vals[5]),
				OrgID:       asString(vals[6]),
				CreatedAt:   parseTime(asString(vals[7])),
				UpdatedAt:   parseTime(asString(vals[8])),
			})
		}
		return list, rec.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("lake get many: %w", err)
	}
	return out.([]domain.Lake), nil
}

// cypherUpdateLakeSpace 更新 space_id；空字符串等价于 REMOVE l.space_id 但为兼容简化我们仍 SET 为 ''。
const cypherUpdateLakeSpace = `
MATCH (l:Lake {id: $id})
SET l.space_id = $space_id, l.updated_at = $updated_at
RETURN l.id, l.name, l.description, l.is_public, l.owner_id, coalesce(l.space_id, '') AS space_id, coalesce(l.org_id, '') AS org_id, l.created_at, l.updated_at
`

func (r *lakeRepoNeo) UpdateSpaceID(ctx context.Context, id, spaceID string) (*domain.Lake, error) {
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()

	out, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		rec, err := tx.Run(ctx, cypherUpdateLakeSpace, map[string]any{
			"id": id, "space_id": spaceID, "updated_at": time.Now().UTC().Format(time.RFC3339Nano),
		})
		if err != nil {
			return nil, err
		}
		if !rec.Next(ctx) {
			return nil, domain.ErrNotFound
		}
		vals := rec.Record().Values
		l := &domain.Lake{
			ID:          asString(vals[0]),
			Name:        asString(vals[1]),
			Description: asString(vals[2]),
			IsPublic:    asBool(vals[3]),
			OwnerID:     asString(vals[4]),
			SpaceID:     asString(vals[5]),
			OrgID:       asString(vals[6]),
			CreatedAt:   parseTime(asString(vals[7])),
			UpdatedAt:   parseTime(asString(vals[8])),
		}
		return l, nil
	})
	if err != nil {
		return nil, err
	}
	return out.(*domain.Lake), nil
}

// cypherUpdateLakeOrg P13-A
const cypherUpdateLakeOrg = `
MATCH (l:Lake {id: $id})
SET l.org_id = $org_id, l.updated_at = $updated_at
RETURN l.id, l.name, l.description, l.is_public, l.owner_id, coalesce(l.space_id, '') AS space_id, coalesce(l.org_id, '') AS org_id, l.created_at, l.updated_at
`

func (r *lakeRepoNeo) UpdateOrgID(ctx context.Context, id, orgID string) (*domain.Lake, error) {
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()

	out, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		rec, err := tx.Run(ctx, cypherUpdateLakeOrg, map[string]any{
			"id": id, "org_id": orgID, "updated_at": time.Now().UTC().Format(time.RFC3339Nano),
		})
		if err != nil {
			return nil, err
		}
		if !rec.Next(ctx) {
			return nil, domain.ErrNotFound
		}
		vals := rec.Record().Values
		l := &domain.Lake{
			ID:          asString(vals[0]),
			Name:        asString(vals[1]),
			Description: asString(vals[2]),
			IsPublic:    asBool(vals[3]),
			OwnerID:     asString(vals[4]),
			SpaceID:     asString(vals[5]),
			OrgID:       asString(vals[6]),
			CreatedAt:   parseTime(asString(vals[7])),
			UpdatedAt:   parseTime(asString(vals[8])),
		}
		return l, nil
	})
	if err != nil {
		return nil, err
	}
	return out.(*domain.Lake), nil
}

const cypherListLakesByOrg = `
MATCH (l:Lake {org_id: $org_id})
RETURN l.id, l.name, l.description, l.is_public, l.owner_id, coalesce(l.space_id, '') AS space_id, coalesce(l.org_id, '') AS org_id, l.created_at, l.updated_at
ORDER BY l.created_at DESC
`

func (r *lakeRepoNeo) ListByOrg(ctx context.Context, orgID string) ([]domain.Lake, error) {
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()

	out, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		rec, err := tx.Run(ctx, cypherListLakesByOrg, map[string]any{"org_id": orgID})
		if err != nil {
			return nil, err
		}
		list := make([]domain.Lake, 0)
		for rec.Next(ctx) {
			vals := rec.Record().Values
			list = append(list, domain.Lake{
				ID:          asString(vals[0]),
				Name:        asString(vals[1]),
				Description: asString(vals[2]),
				IsPublic:    asBool(vals[3]),
				OwnerID:     asString(vals[4]),
				SpaceID:     asString(vals[5]),
				OrgID:       asString(vals[6]),
				CreatedAt:   parseTime(asString(vals[7])),
				UpdatedAt:   parseTime(asString(vals[8])),
			})
		}
		return list, rec.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("list lakes by org: %w", err)
	}
	return out.([]domain.Lake), nil
}

const cypherCountLakesByOrgIDs = `
MATCH (l:Lake)
WHERE l.org_id IN $org_ids
RETURN l.org_id AS org_id, count(l) AS count
`

func (r *lakeRepoNeo) CountLakesByOrgIDs(ctx context.Context, orgIDs []string) (map[string]int64, error) {
	out := make(map[string]int64, len(orgIDs))
	if len(orgIDs) == 0 {
		return out, nil
	}
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()

	res, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		rec, err := tx.Run(ctx, cypherCountLakesByOrgIDs, map[string]any{"org_ids": orgIDs})
		if err != nil {
			return nil, err
		}
		counts := make(map[string]int64, len(orgIDs))
		for rec.Next(ctx) {
			vals := rec.Record().Values
			if len(vals) < 2 {
				continue
			}
			counts[asString(vals[0])] = asInt64(vals[1])
		}
		return counts, rec.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("count lakes by org ids: %w", err)
	}
	return res.(map[string]int64), nil
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func asBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func asFloat(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
