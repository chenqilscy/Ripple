package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TxRunner 包装 pgx 事务执行。fn 返回错误则 Rollback，否则 Commit。
type TxRunner interface {
	RunInTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type txRunnerPG struct{ pool *pgxpool.Pool }

// NewTxRunner 创建。
func NewTxRunner(pool *pgxpool.Pool) TxRunner { return &txRunnerPG{pool: pool} }

func (r *txRunnerPG) RunInTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("tx begin: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("tx commit: %w", err)
	}
	return nil
}
