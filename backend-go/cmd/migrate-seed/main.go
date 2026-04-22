// Command migrate-seed 把指定 versions 标记为"已应用"（不执行 SQL），用于把现有数据库纳入新 migrate 跟踪。
//
// 用法：
//   go run ./cmd/migrate-seed 0001_init 0002_cloud 0003_llm_calls 0004_lake_invites 0005_node_revisions 0006_spaces
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: migrate-seed <version> [<version>...]")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.PGURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pg connect:", err)
		os.Exit(1)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW());`); err != nil {
		fmt.Fprintln(os.Stderr, "ensure tracker:", err)
		os.Exit(1)
	}
	for _, v := range os.Args[1:] {
		if _, err := pool.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES ($1) ON CONFLICT DO NOTHING", v); err != nil {
			fmt.Fprintln(os.Stderr, "insert", v, ":", err)
			os.Exit(1)
		}
		fmt.Println("seeded", v)
	}
	fmt.Println("OK")
}
