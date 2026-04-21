package store

import (
	"context"
	"fmt"

	"github.com/chenqilscy/ripple/backend-go/internal/config"
	"github.com/redis/go-redis/v9"
)

// NewRedis 初始化并 Ping Redis 客户端。
func NewRedis(ctx context.Context, c *config.Config) (*redis.Client, error) {
	cli := redis.NewClient(&redis.Options{
		Addr:     c.RedisAddr,
		Password: c.RedisPass,
		DB:       c.RedisDB,
	})
	if err := cli.Ping(ctx).Err(); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("redis: ping: %w", err)
	}
	return cli, nil
}
