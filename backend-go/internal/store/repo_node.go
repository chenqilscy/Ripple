package store

import (
	"context"
	"fmt"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// NodeRepository 节点在 Neo4j 的读写。
type NodeRepository interface {
	Create(ctx context.Context, n *domain.Node) error
	GetByID(ctx context.Context, id string) (*domain.Node, error)
	ListByLake(ctx context.Context, lakeID string, includeVapor bool) ([]domain.Node, error)
	UpdateState(ctx context.Context, n *domain.Node) error
	// UpdateContent 更新节点 content 与 updated_at。不改状态、不动软删字段。
	UpdateContent(ctx context.Context, n *domain.Node) error
	// Search 在指定湖内全文搜索节点（P12-D）。
	Search(ctx context.Context, lakeID, q string, limit int) ([]domain.NodeSearchResult, error)
	// BatchCreate 批量创建节点（单事务 UNWIND），P12-A。
	BatchCreate(ctx context.Context, nodes []*domain.Node) error
	// FindRelated P18-A：在同湖内找与指定节点内容相近的节点（全文搜索）。
	FindRelated(ctx context.Context, nodeID, lakeID, keyword string, limit int) ([]domain.NodeSearchResult, error)
}

type nodeRepoNeo struct {
	driver neo4j.DriverWithContext
	dbName string
}

// NewNodeRepository 构造。
func NewNodeRepository(driver neo4j.DriverWithContext, dbName string) NodeRepository {
	return &nodeRepoNeo{driver: driver, dbName: dbName}
}

// 创建节点。若 lakeID 非空，自动建立 (:Lake)-[:CONTAINS]->(:Node) 关系。
const cypherCreateNode = `
CREATE (n:Node {
  id: $id, lake_id: $lake_id, owner_id: $owner_id,
  content: $content, type: $type, state: $state,
  x: $x, y: $y, z: $z,
  created_at: $created_at, updated_at: $updated_at
})
WITH n
OPTIONAL MATCH (l:Lake {id: $lake_id})
FOREACH (_ IN CASE WHEN l IS NOT NULL THEN [1] ELSE [] END |
  MERGE (l)-[:CONTAINS]->(n)
)
RETURN n.id
`

func (r *nodeRepoNeo) Create(ctx context.Context, n *domain.Node) error {
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()

	x, y, z := 0.0, 0.0, 0.0
	if n.Position != nil {
		x, y, z = n.Position.X, n.Position.Y, n.Position.Z
	}

	_, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, cypherCreateNode, map[string]any{
			"id":         n.ID,
			"lake_id":    n.LakeID,
			"owner_id":   n.OwnerID,
			"content":    n.Content,
			"type":       string(n.Type),
			"state":      string(n.State),
			"x":          x,
			"y":          y,
			"z":          z,
			"created_at": n.CreatedAt.UTC().Format(time.RFC3339Nano),
			"updated_at": n.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("node create: %w", err)
	}
	return nil
}

const cypherGetNode = `
MATCH (n:Node {id: $id})
RETURN n.id, n.lake_id, n.owner_id, n.content, n.type, n.state,
       n.x, n.y, n.z, n.created_at, n.updated_at, n.deleted_at, n.ttl_at
`

func (r *nodeRepoNeo) GetByID(ctx context.Context, id string) (*domain.Node, error) {
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()

	out, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		rec, err := tx.Run(ctx, cypherGetNode, map[string]any{"id": id})
		if err != nil {
			return nil, err
		}
		if !rec.Next(ctx) {
			return nil, domain.ErrNotFound
		}
		return scanNode(rec.Record().Values), nil
	})
	if err != nil {
		return nil, err
	}
	return out.(*domain.Node), nil
}

const cypherListByLake = `
MATCH (:Lake {id: $lake_id})-[:CONTAINS]->(n:Node)
WHERE $include_vapor OR n.state <> 'VAPOR'
RETURN n.id, n.lake_id, n.owner_id, n.content, n.type, n.state,
       n.x, n.y, n.z, n.created_at, n.updated_at, n.deleted_at, n.ttl_at
ORDER BY n.created_at DESC
`

func (r *nodeRepoNeo) ListByLake(ctx context.Context, lakeID string, includeVapor bool) ([]domain.Node, error) {
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()

	out, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		rec, err := tx.Run(ctx, cypherListByLake, map[string]any{
			"lake_id":       lakeID,
			"include_vapor": includeVapor,
		})
		if err != nil {
			return nil, err
		}
		nodes := make([]domain.Node, 0)
		for rec.Next(ctx) {
			nodes = append(nodes, *scanNode(rec.Record().Values))
		}
		return nodes, rec.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("node list: %w", err)
	}
	return out.([]domain.Node), nil
}

const cypherUpdateNodeState = `
MATCH (n:Node {id: $id})
SET n.state = $state,
    n.lake_id = $lake_id,
    n.updated_at = $updated_at,
    n.deleted_at = $deleted_at,
    n.ttl_at = $ttl_at
WITH n
OPTIONAL MATCH (l:Lake {id: $lake_id})
FOREACH (_ IN CASE WHEN l IS NOT NULL THEN [1] ELSE [] END |
  MERGE (l)-[:CONTAINS]->(n)
)
RETURN n.id
`

func (r *nodeRepoNeo) UpdateState(ctx context.Context, n *domain.Node) error {
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()

	var deletedAt, ttlAt any
	if n.DeletedAt != nil {
		deletedAt = n.DeletedAt.UTC().Format(time.RFC3339Nano)
	}
	if n.TTLAt != nil {
		ttlAt = n.TTLAt.UTC().Format(time.RFC3339Nano)
	}

	_, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, cypherUpdateNodeState, map[string]any{
			"id":         n.ID,
			"state":      string(n.State),
			"lake_id":    n.LakeID,
			"updated_at": n.UpdatedAt.UTC().Format(time.RFC3339Nano),
			"deleted_at": deletedAt,
			"ttl_at":     ttlAt,
		})
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("node update state: %w", err)
	}
	return nil
}

const cypherUpdateNodeContent = `
MATCH (n:Node {id: $id})
SET n.content = $content, n.updated_at = $updated_at
RETURN n.id
`

// UpdateContent 更新节点 content 与 updated_at。若节点不存在返回 ErrNotFound。
func (r *nodeRepoNeo) UpdateContent(ctx context.Context, n *domain.Node) error {
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()

	_, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		rec, err := tx.Run(ctx, cypherUpdateNodeContent, map[string]any{
			"id":         n.ID,
			"content":    n.Content,
			"updated_at": n.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
		if err != nil {
			return nil, err
		}
		if !rec.Next(ctx) {
			return nil, domain.ErrNotFound
		}
		return nil, nil
	})
	if err != nil {
		return fmt.Errorf("node update content: %w", err)
	}
	return nil
}

// scanNode 把单条 Neo4j 记录转 domain.Node。
func scanNode(v []any) *domain.Node {
	n := &domain.Node{
		ID:        asString(v[0]),
		LakeID:    asString(v[1]),
		OwnerID:   asString(v[2]),
		Content:   asString(v[3]),
		Type:      domain.NodeType(asString(v[4])),
		State:     domain.NodeState(asString(v[5])),
		Position:  &domain.Position{X: asFloat(v[6]), Y: asFloat(v[7]), Z: asFloat(v[8])},
		CreatedAt: parseTime(asString(v[9])),
		UpdatedAt: parseTime(asString(v[10])),
	}
	if s := asString(v[11]); s != "" {
		t := parseTime(s)
		n.DeletedAt = &t
	}
	if s := asString(v[12]); s != "" {
		t := parseTime(s)
		n.TTLAt = &t
	}
	return n
}

// Search 全文搜索（P12-D）。
// 先用 Lucene FTS 索引命中，再按 lake_id 和 state 过滤。limit 最大 50。
const cypherSearchNodes = `
CALL db.index.fulltext.queryNodes("node_content_fts", $q)
YIELD node AS n, score
WHERE n.lake_id = $lake_id AND n.state <> 'ERASED'
RETURN n.id, n.lake_id, n.content, score
ORDER BY score DESC
LIMIT $limit
`

func (r *nodeRepoNeo) Search(ctx context.Context, lakeID, q string, limit int) ([]domain.NodeSearchResult, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()

	out, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		rec, err := tx.Run(ctx, cypherSearchNodes, map[string]any{
			"q":       q,
			"lake_id": lakeID,
			"limit":   int64(limit),
		})
		if err != nil {
			return nil, err
		}
		results := make([]domain.NodeSearchResult, 0)
		for rec.Next(ctx) {
			v := rec.Record().Values
			content := asString(v[2])
			snippet := content
			if len([]rune(snippet)) > 150 {
				runes := []rune(snippet)
				snippet = string(runes[:150]) + "…"
			}
			results = append(results, domain.NodeSearchResult{
				NodeID:  asString(v[0]),
				LakeID:  asString(v[1]),
				Snippet: snippet,
				Score:   asFloat(v[3]),
			})
		}
		return results, rec.Err()
	})
	if err != nil {
		return nil, err
	}
	return out.([]domain.NodeSearchResult), nil
}

// cypherBatchCreateNodes 单事务批量创建节点（P12-A）。
const cypherBatchCreateNodes = `
UNWIND $nodes AS item
CREATE (n:Node {
  id: item.id, lake_id: item.lake_id, owner_id: item.owner_id,
  content: item.content, type: item.type, state: item.state,
  x: 0.0, y: 0.0, z: 0.0,
  created_at: item.created_at, updated_at: item.updated_at
})
WITH n
OPTIONAL MATCH (l:Lake {id: n.lake_id})
FOREACH (_ IN CASE WHEN l IS NOT NULL THEN [1] ELSE [] END |
  MERGE (l)-[:CONTAINS]->(n)
)
RETURN n.id
`

// BatchCreate 批量创建节点，单事务 UNWIND（P12-A）。
func (r *nodeRepoNeo) BatchCreate(ctx context.Context, nodes []*domain.Node) error {
	if len(nodes) == 0 {
		return nil
	}
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()

	items := make([]map[string]any, len(nodes))
	for i, n := range nodes {
		items[i] = map[string]any{
			"id":         n.ID,
			"lake_id":    n.LakeID,
			"owner_id":   n.OwnerID,
			"content":    n.Content,
			"type":       string(n.Type),
			"state":      string(n.State),
			"created_at": n.CreatedAt,
			"updated_at": n.UpdatedAt,
		}
	}

	_, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, cypherBatchCreateNodes, map[string]any{"nodes": items})
		return nil, err
	})
	return err
}

// cypherFindRelated P18-A：在同湖内利用全文索引查找相关节点（排除自身）。
// 依赖 node_content_fts（与 cypherSearchNodes 相同的索引）。
const cypherFindRelated = `
CALL db.index.fulltext.queryNodes("node_content_fts", $q)
YIELD node AS n, score
WHERE n.lake_id = $lake_id AND n.id <> $node_id
  AND n.state IN ["MIST","DROP","FROZEN"]
RETURN n.id, n.lake_id, n.content, score
ORDER BY score DESC
LIMIT $limit
`

// FindRelated P18-A：在同湖内找与关键词相关的节点（排除 nodeID 自身）。
func (r *nodeRepoNeo) FindRelated(ctx context.Context, nodeID, lakeID, keyword string, limit int) ([]domain.NodeSearchResult, error) {
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	sess := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.dbName})
	defer func() { _ = sess.Close(ctx) }()

	out, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		rec, err := tx.Run(ctx, cypherFindRelated, map[string]any{
			"q":       keyword,
			"lake_id": lakeID,
			"node_id": nodeID,
			"limit":   int64(limit),
		})
		if err != nil {
			return nil, err
		}
		var results []domain.NodeSearchResult
		for rec.Next(ctx) {
			v := rec.Record().Values
			content := asString(v[2])
			snippet := content
			if len([]rune(snippet)) > 150 {
				runes := []rune(snippet)
				snippet = string(runes[:150]) + "…"
			}
			results = append(results, domain.NodeSearchResult{
				NodeID:  asString(v[0]),
				LakeID:  asString(v[1]),
				Snippet: snippet,
				Score:   asFloat(v[3]),
			})
		}
		return results, rec.Err()
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		return []domain.NodeSearchResult{}, nil
	}
	return out.([]domain.NodeSearchResult), nil
}

