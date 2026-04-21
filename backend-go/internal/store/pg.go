// Package store 提供数据库客户端与仓库实现。
//
// 客户端：pg (pgxpool) / neo4j / redis 三个 Connect/Close 函数。
// 仓库接口与实现按资源拆分文件。
package store

import (
	"context"
	"fmt"

	"github.com/chenqilscy/ripple/backend-go/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPGPool 初始化并 Ping PG 连接池。
func NewPGPool(ctx context.Context, c *config.Config) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(c.PGURL)
	if err != nil {
		return nil, fmt.Errorf("pg: parse url: %w", err)
	}
	cfg.MaxConns = c.PGMaxConns
	cfg.MinConns = c.PGMinConns
	cfg.ConnConfig.ConnectTimeout = c.PGConnTimeout

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pg: new pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pg: ping: %w", err)
	}
	return pool, nil
}
