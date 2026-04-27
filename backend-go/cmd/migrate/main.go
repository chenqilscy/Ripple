// Command migrate 把 migrations/*.up.sql 顺次应用到 PG，并通过 schema_migrations 表跟踪已应用版本（幂等）。
//
// 用法：
//   go run ./cmd/migrate           # 执行 up（仅未应用的）
//   go run ./cmd/migrate down      # 回滚最近一个（危险）
//   go run ./cmd/migrate down all  # 全量回滚（极度危险）
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/config"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ensureTrackerSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

func main() {
	direction := "up"
	scope := "one"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}
	if len(os.Args) > 2 {
		scope = os.Args[2]
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
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.PGURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pg connect:", err)
		os.Exit(1)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, ensureTrackerSQL); err != nil {
		fmt.Fprintln(os.Stderr, "ensure schema_migrations:", err)
		os.Exit(1)
	}

	applied, err := loadApplied(ctx, pool)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load applied:", err)
		os.Exit(1)
	}

	files, err := filepath.Glob("migrations/*." + direction + ".sql")
	if err != nil {
		fmt.Fprintln(os.Stderr, "glob:", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no migration files found")
		os.Exit(1)
	}
	if direction == "down" {
		sort.Sort(sort.Reverse(sort.StringSlice(files)))
	} else {
		sort.Strings(files)
	}

	count := 0
	for _, f := range files {
		ver := versionOf(f, direction)
		isApplied := applied[ver]
		switch direction {
		case "up":
			if isApplied {
				fmt.Println("SKIP", f, "(already applied)")
				continue
			}
		case "down":
			if !isApplied {
				fmt.Println("SKIP", f, "(not applied)")
				continue
			}
		}
		fmt.Println("==>", f)
		raw, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read:", err)
			os.Exit(1)
		}
		sql := strings.TrimSpace(strings.TrimPrefix(string(raw), "\ufeff"))
		if sql == "" {
			continue
		}
		if err := execMigrationSQL(ctx, pool, sql); err != nil {
			fmt.Fprintln(os.Stderr, "exec:", err)
			os.Exit(1)
		}
		if direction == "up" {
			if _, err := pool.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES ($1) ON CONFLICT DO NOTHING", ver); err != nil {
				fmt.Fprintln(os.Stderr, "track up:", err)
				os.Exit(1)
			}
		} else {
			if _, err := pool.Exec(ctx, "DELETE FROM schema_migrations WHERE version=$1", ver); err != nil {
				fmt.Fprintln(os.Stderr, "track down:", err)
				os.Exit(1)
			}
		}
		count++
		if direction == "down" && scope != "all" {
			break
		}
	}
	fmt.Printf("OK · %s applied=%d\n", direction, count)
}

func execMigrationSQL(ctx context.Context, pool *pgxpool.Pool, sql string) error {
	statements, err := splitSQLStatements(sql)
	if err != nil {
		return err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	for _, stmt := range statements {
		stmt = strings.TrimSpace(strings.TrimPrefix(stripSQLComments(stmt), "\ufeff"))
		if stmt == "" {
			continue
		}
		if _, err := conn.Exec(ctx, stmt); err != nil {
			return formatExecError(err, stmt)
		}
	}
	return nil
}

func formatExecError(err error, stmt string) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	msg := fmt.Sprintf("%s (SQLSTATE %s)", pgErr.Message, pgErr.Code)
	if pgErr.Position == 0 {
		return fmt.Errorf("%s | stmt=%q", msg, stmt)
	}
	line, col := positionToLineCol(stmt, int(pgErr.Position))
	return fmt.Errorf("%s at line %d col %d | stmt=%q", msg, line, col, stmt)
}

func positionToLineCol(stmt string, pos int) (int, int) {
	if pos < 1 {
		return 1, 1
	}
	line := 1
	col := 1
	for i := 0; i < len(stmt) && i < pos-1; i++ {
		if stmt[i] == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return line, col
}

func splitSQLStatements(sql string) ([]string, error) {
	var (
		statements   []string
		current      strings.Builder
		inSingle     bool
		inDouble     bool
		inLineCmt    bool
		inBlockCmt   bool
		dollarTag    string
	)

	flush := func() {
		stmt := strings.TrimSpace(current.String())
		if stmt != "" {
			statements = append(statements, stmt)
		}
		current.Reset()
	}

	for i := 0; i < len(sql); i++ {
		if dollarTag != "" {
			if strings.HasPrefix(sql[i:], dollarTag) {
				current.WriteString(dollarTag)
				i += len(dollarTag) - 1
				dollarTag = ""
				continue
			}
			current.WriteByte(sql[i])
			continue
		}

		if inLineCmt {
			current.WriteByte(sql[i])
			if sql[i] == '\n' {
				inLineCmt = false
			}
			continue
		}

		if inBlockCmt {
			current.WriteByte(sql[i])
			if sql[i] == '*' && i+1 < len(sql) && sql[i+1] == '/' {
				current.WriteByte(sql[i+1])
				i++
				inBlockCmt = false
			}
			continue
		}

		if inSingle {
			current.WriteByte(sql[i])
			if sql[i] == '\'' {
				if i+1 < len(sql) && sql[i+1] == '\'' {
					current.WriteByte(sql[i+1])
					i++
				} else {
					inSingle = false
				}
			}
			continue
		}

		if inDouble {
			current.WriteByte(sql[i])
			if sql[i] == '"' {
				if i+1 < len(sql) && sql[i+1] == '"' {
					current.WriteByte(sql[i+1])
					i++
				} else {
					inDouble = false
				}
			}
			continue
		}

		if sql[i] == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			current.WriteString("--")
			i++
			inLineCmt = true
			continue
		}
		if sql[i] == '/' && i+1 < len(sql) && sql[i+1] == '*' {
			current.WriteString("/*")
			i++
			inBlockCmt = true
			continue
		}
		if sql[i] == '\'' {
			current.WriteByte(sql[i])
			inSingle = true
			continue
		}
		if sql[i] == '"' {
			current.WriteByte(sql[i])
			inDouble = true
			continue
		}
		if sql[i] == '$' {
			if tag, ok := readDollarTag(sql[i:]); ok {
				current.WriteString(tag)
				i += len(tag) - 1
				dollarTag = tag
				continue
			}
		}
		if sql[i] == ';' {
			flush()
			continue
		}
		current.WriteByte(sql[i])
	}

	if dollarTag != "" || inSingle || inDouble || inBlockCmt {
		return nil, errors.New("unterminated SQL literal or comment")
	}
	flush()
	return statements, nil
}

func readDollarTag(s string) (string, bool) {
	if len(s) < 2 || s[0] != '$' {
		return "", false
	}
	for i := 1; i < len(s); i++ {
		switch ch := s[i]; {
		case ch == '$':
			return s[:i+1], true
		case ch == '_' || ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z':
			continue
		default:
			return "", false
		}
	}
	return "", false
}

func stripSQLComments(sql string) string {
	var (
		out         strings.Builder
		inSingle    bool
		inDouble    bool
		inLineCmt   bool
		inBlockCmt  bool
		dollarTag   string
	)

	for i := 0; i < len(sql); i++ {
		if dollarTag != "" {
			if strings.HasPrefix(sql[i:], dollarTag) {
				out.WriteString(dollarTag)
				i += len(dollarTag) - 1
				dollarTag = ""
				continue
			}
			out.WriteByte(sql[i])
			continue
		}
		if inLineCmt {
			if sql[i] == '\n' {
				inLineCmt = false
				out.WriteByte('\n')
			}
			continue
		}
		if inBlockCmt {
			if sql[i] == '*' && i+1 < len(sql) && sql[i+1] == '/' {
				i++
				inBlockCmt = false
			}
			continue
		}
		if inSingle {
			out.WriteByte(sql[i])
			if sql[i] == '\'' {
				if i+1 < len(sql) && sql[i+1] == '\'' {
					out.WriteByte(sql[i+1])
					i++
				} else {
					inSingle = false
				}
			}
			continue
		}
		if inDouble {
			out.WriteByte(sql[i])
			if sql[i] == '"' {
				if i+1 < len(sql) && sql[i+1] == '"' {
					out.WriteByte(sql[i+1])
					i++
				} else {
					inDouble = false
				}
			}
			continue
		}
		if sql[i] == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			i++
			inLineCmt = true
			continue
		}
		if sql[i] == '/' && i+1 < len(sql) && sql[i+1] == '*' {
			i++
			inBlockCmt = true
			continue
		}
		if sql[i] == '\'' {
			out.WriteByte(sql[i])
			inSingle = true
			continue
		}
		if sql[i] == '"' {
			out.WriteByte(sql[i])
			inDouble = true
			continue
		}
		if sql[i] == '$' {
			if tag, ok := readDollarTag(sql[i:]); ok {
				out.WriteString(tag)
				i += len(tag) - 1
				dollarTag = tag
				continue
			}
		}
		out.WriteByte(sql[i])
	}
	return out.String()
}

func loadApplied(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

// versionOf 把 "migrations/0007_perma_nodes.up.sql" 转为 "0007_perma_nodes"。
func versionOf(path, direction string) string {
	base := filepath.Base(path)
	suffix := "." + direction + ".sql"
	return strings.TrimSuffix(base, suffix)
}
