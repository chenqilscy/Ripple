// Command migrate 把 migrations/*.up.sql 顺次应用到 PG。
// 用法：
//   go run ./cmd/migrate           # 执行 up
//   go run ./cmd/migrate down      # 执行 down（危险）
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}
	if direction != "up" && direction != "down" {
		fmt.Fprintln(os.Stderr, "direction must be 'up' or 'down'")
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.PGURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pg connect:", err)
		os.Exit(1)
	}
	defer pool.Close()

	files, err := filepath.Glob("migrations/*." + direction + ".sql")
	if err != nil {
		fmt.Fprintln(os.Stderr, "glob:", err)
		os.Exit(1)
	}
	if direction == "down" {
		sort.Sort(sort.Reverse(sort.StringSlice(files)))
	} else {
		sort.Strings(files)
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no migration files found")
		os.Exit(1)
	}

	for _, f := range files {
		fmt.Println("==>", f)
		raw, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read:", err)
			os.Exit(1)
		}
		sql := strings.TrimSpace(string(raw))
		if sql == "" {
			continue
		}
		if _, err := pool.Exec(ctx, sql); err != nil {
			fmt.Fprintln(os.Stderr, "exec:", err)
			os.Exit(1)
		}
	}
	fmt.Println("OK · migrations applied (" + direction + ")")
}
