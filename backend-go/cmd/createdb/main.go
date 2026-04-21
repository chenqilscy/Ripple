// Command createdb 创建数据库（连接到 postgres 库执行 CREATE DATABASE）。
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	url := os.Getenv("RIPPLE_PG_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "RIPPLE_PG_URL not set")
		os.Exit(1)
	}
	// 截 dbname
	idx := strings.LastIndex(url, "/")
	q := strings.Index(url[idx:], "?")
	end := len(url)
	if q >= 0 {
		end = idx + q
	}
	dbname := url[idx+1 : end]
	adminURL := url[:idx+1] + "postgres"
	if q >= 0 {
		adminURL += url[idx+q:]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect admin:", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, "CREATE DATABASE \""+dbname+"\"")
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			fmt.Println("OK · database already exists:", dbname)
			return
		}
		fmt.Fprintln(os.Stderr, "create db:", err)
		os.Exit(1)
	}
	fmt.Println("OK · created database:", dbname)
}
