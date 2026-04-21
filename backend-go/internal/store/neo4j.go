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
	return driver, nil
}
