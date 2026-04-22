package store

import (
	"context"
	"fmt"

	"github.com/chenqilscy/ripple/backend-go/internal/config"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// NewNeo4jDriver 初始化并 VerifyConnectivity Neo4j。
func NewNeo4jDriver(ctx context.Context, c *config.Config) (neo4j.DriverWithContext, error) {
	driver, err := neo4j.NewDriverWithContext(
		c.Neo4jURI,
		neo4j.BasicAuth(c.Neo4jUser, c.Neo4jPass, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("neo4j: new driver: %w", err)
	}
	if err := driver.VerifyConnectivity(ctx); err != nil {
		_ = driver.Close(ctx)
		return nil, fmt.Errorf("neo4j: verify: %w", err)
	}
	if err := ensureNodeSearchIndex(ctx, driver, c.Neo4jDatabase); err != nil {
		_ = driver.Close(ctx)
		return nil, fmt.Errorf("neo4j: ensure search index: %w", err)
	}
	return driver, nil
}

// ensureNodeSearchIndex 创建节点内容全文索引（P12-D）。
// IF NOT EXISTS 保证幂等，多次启动安全。
const cypherCreateFTSIndex = `
CREATE FULLTEXT INDEX node_content_fts IF NOT EXISTS
FOR (n:Node) ON EACH [n.content]
`

func ensureNodeSearchIndex(ctx context.Context, driver neo4j.DriverWithContext, dbName string) error {
	sess := driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: dbName})
	defer func() { _ = sess.Close(ctx) }()
	_, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, cypherCreateFTSIndex, nil)
		return nil, err
	})
	return err
}

