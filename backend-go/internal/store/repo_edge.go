package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// EdgeRepository 边在 Neo4j 的读写。
//
// 设计：
//   - 边存为 (:Node)-[:EDGE {...}]->(:Node)
//   - lake_id 冗余在 EDGE 属性里，便于按湖列表
//   - 软删：DELETED_AT 属性置为 ISO 时间字符串
//
// 注意：Neo4j 不强制关系唯一性，重复（src,dst,kind）alive 在 service 层应用层校验。
type EdgeRepository interface {
	Create(ctx context.Context, e *domain.Edge) error
	GetByID(ctx context.Context, id string) (*domain.Edge, error)
	ListByLake(ctx context.Context, lakeID string, includeDeleted bool) ([]domain.Edge, error)
	ExistsAlive(ctx context.Context, src, dst string, kind domain.EdgeKind) (bool, error)
	SoftDelete(ctx context.Context, id string, when time.Time) error
}

type edgeRepoNeo struct {
	driver neo4j.DriverWithContext
	dbName string
}

// NewEdgeRepository 装配。
func NewEdgeRepository(driver neo4j.DriverWithContext, dbName string) EdgeRepository {
	return &edgeRepoNeo{driver: driver, dbName: dbName}
}

const cypherCreateEdge = `
MATCH (s:Node {id: $src}), (d:Node {id: $dst})
WHERE s.lake_id = $lake_id AND d.lake_id = $lake_id
OPTIONAL MATCH (s)-[dup:EDGE {kind: $kind}]->(d) WHERE dup.deleted_at IS NULL
WITH s, d, dup
WHERE dup IS NULL
CREATE (s)-[e:EDGE {
  id: $id, lake_id: $lake_id,
  kind: $kind, label: $label, owner_id: $owner_id,
  created_at: $created_at
}]->(d)
RETURN e.id
`

func (r *edgeRepoNeo) Create(ctx context.Context, e *domain.Edge) error {
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()
	_, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypherCreateEdge, map[string]any{
			"id":         e.ID,
			"src":        e.SrcNodeID,
			"dst":        e.DstNodeID,
			"lake_id":    e.LakeID,
			"kind":       string(e.Kind),
			"label":      e.Label,
			"owner_id":   e.OwnerID,
			"created_at": e.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
		if err != nil {
			return nil, err
		}
		if !res.Next(ctx) {
			// 三种可能：
			// 1. src/dst 节点不存在
			// 2. 不同湖
			// 3. 已存在同 kind 的 alive 边（TOCTOU 兜底）
			// 上层 service 已先做精确报错；这里返回通用错误。
			return nil, fmt.Errorf("%w: src/dst not found, cross-lake, or duplicate edge", domain.ErrInvalidInput)
		}
		return nil, nil
	})
	if err != nil {
		return fmt.Errorf("edge create: %w", err)
	}
	return nil
}

const cypherGetEdge = `
MATCH ()-[e:EDGE {id: $id}]->()
RETURN e.id, e.lake_id, startNode(e).id, endNode(e).id,
       e.kind, e.label, e.owner_id, e.created_at, e.deleted_at
`

func (r *edgeRepoNeo) GetByID(ctx context.Context, id string) (*domain.Edge, error) {
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()
	out, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypherGetEdge, map[string]any{"id": id})
		if err != nil {
			return nil, err
		}
		if !res.Next(ctx) {
			return nil, domain.ErrNotFound
		}
		return scanEdge(res.Record().Values), nil
	})
	if err != nil {
		return nil, err
	}
	return out.(*domain.Edge), nil
}

const cypherListEdgesAlive = `
MATCH (s:Node)-[e:EDGE {lake_id: $lake_id}]->(d:Node)
WHERE e.deleted_at IS NULL
RETURN e.id, e.lake_id, s.id, d.id, e.kind, e.label, e.owner_id, e.created_at, e.deleted_at
ORDER BY e.created_at ASC
`

const cypherListEdgesAll = `
MATCH (s:Node)-[e:EDGE {lake_id: $lake_id}]->(d:Node)
RETURN e.id, e.lake_id, s.id, d.id, e.kind, e.label, e.owner_id, e.created_at, e.deleted_at
ORDER BY e.created_at ASC
`

func (r *edgeRepoNeo) ListByLake(ctx context.Context, lakeID string, includeDeleted bool) ([]domain.Edge, error) {
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()
	q := cypherListEdgesAlive
	if includeDeleted {
		q = cypherListEdgesAll
	}
	out, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, q, map[string]any{"lake_id": lakeID})
		if err != nil {
			return nil, err
		}
		var list []domain.Edge
		for res.Next(ctx) {
			list = append(list, *scanEdge(res.Record().Values))
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		return []domain.Edge{}, nil
	}
	return out.([]domain.Edge), nil
}

const cypherExistsEdgeAlive = `
MATCH ()-[e:EDGE]->()
WHERE startNode(e).id = $src AND endNode(e).id = $dst
  AND e.kind = $kind AND e.deleted_at IS NULL
RETURN COUNT(e) AS c
`

func (r *edgeRepoNeo) ExistsAlive(ctx context.Context, src, dst string, kind domain.EdgeKind) (bool, error) {
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()
	out, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypherExistsEdgeAlive, map[string]any{
			"src": src, "dst": dst, "kind": string(kind),
		})
		if err != nil {
			return nil, err
		}
		if !res.Next(ctx) {
			return false, nil
		}
		c, _ := res.Record().Values[0].(int64)
		return c > 0, nil
	})
	if err != nil {
		return false, err
	}
	return out.(bool), nil
}

const cypherSoftDeleteEdge = `
MATCH ()-[e:EDGE {id: $id}]->()
WHERE e.deleted_at IS NULL
SET e.deleted_at = $now
RETURN e.id
`

func (r *edgeRepoNeo) SoftDelete(ctx context.Context, id string, when time.Time) error {
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()
	_, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypherSoftDeleteEdge, map[string]any{
			"id": id, "now": when.UTC().Format(time.RFC3339Nano),
		})
		if err != nil {
			return nil, err
		}
		if !res.Next(ctx) {
			return nil, domain.ErrNotFound
		}
		return nil, nil
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return err
		}
		return fmt.Errorf("edge soft delete: %w", err)
	}
	return nil
}

// scanEdge 把 Neo4j Record.Values 解析成 domain.Edge。
// 字段顺序与 cypherGetEdge / cypherListEdges 保持一致。
func scanEdge(vals []any) *domain.Edge {
	e := &domain.Edge{}
	if len(vals) < 9 {
		return e
	}
	if v, ok := vals[0].(string); ok {
		e.ID = v
	}
	if v, ok := vals[1].(string); ok {
		e.LakeID = v
	}
	if v, ok := vals[2].(string); ok {
		e.SrcNodeID = v
	}
	if v, ok := vals[3].(string); ok {
		e.DstNodeID = v
	}
	if v, ok := vals[4].(string); ok {
		e.Kind = domain.EdgeKind(v)
	}
	if v, ok := vals[5].(string); ok {
		e.Label = v
	}
	if v, ok := vals[6].(string); ok {
		e.OwnerID = v
	}
	if v, ok := vals[7].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			e.CreatedAt = t
		}
	}
	if vals[8] != nil {
		if v, ok := vals[8].(string); ok {
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				e.DeletedAt = &t
			}
		}
	}
	return e
}
