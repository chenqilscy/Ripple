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
  owner_id: $owner_id, created_at: $created_at, updated_at: $updated_at
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
RETURN l.id, l.name, l.description, l.is_public, l.owner_id, l.created_at, l.updated_at
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
			CreatedAt:   parseTime(asString(vals[5])),
			UpdatedAt:   parseTime(asString(vals[6])),
		}
		return l, nil
	})
	if err != nil {
		return nil, err
	}
	return out.(*domain.Lake), nil
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
