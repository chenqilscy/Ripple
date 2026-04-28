package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SubscriptionRepository 是组织套餐订阅持久化接口（Phase 15-D）。
type SubscriptionRepository interface {
	// UpsertActive 新增 active 订阅；若已有 active 订阅先将其状态改为 cancelled。
	UpsertActive(ctx context.Context, sub domain.OrgSubscription) (*domain.OrgSubscription, error)
	// GetActiveByOrgID 获取当前 active 订阅，无则返回 ErrNotFound。
	GetActiveByOrgID(ctx context.Context, orgID string) (*domain.OrgSubscription, error)
}

type subscriptionRepoPG struct{ pool *pgxpool.Pool }

// NewSubscriptionRepository 构造。
func NewSubscriptionRepository(pool *pgxpool.Pool) SubscriptionRepository {
	return &subscriptionRepoPG{pool: pool}
}

const sqlCancelActiveSubscription = `
UPDATE org_subscriptions
SET status = 'cancelled'
WHERE org_id = $1 AND status = 'active'
`

const sqlInsertSubscription = `
INSERT INTO org_subscriptions (id, org_id, plan_id, status, billing_cycle, stub, started_at, expires_at, created_at)
VALUES ($1::uuid, $2, $3, 'active', $4, $5, $6, $7, $6)
RETURNING id::text, org_id, plan_id, status, billing_cycle, stub, started_at, expires_at, created_at
`

func (r *subscriptionRepoPG) UpsertActive(ctx context.Context, sub domain.OrgSubscription) (*domain.OrgSubscription, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("subscription upsert begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 先取消现有 active（若有）
	if _, err := tx.Exec(ctx, sqlCancelActiveSubscription, sub.OrgID); err != nil {
		return nil, fmt.Errorf("subscription cancel existing: %w", err)
	}

	row := tx.QueryRow(ctx, sqlInsertSubscription,
		sub.ID, sub.OrgID, sub.PlanID, string(sub.BillingCycle), sub.Stub,
		sub.StartedAt, sub.ExpiresAt,
	)
	result, err := scanSubscription(row)
	if err != nil {
		return nil, fmt.Errorf("subscription insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("subscription commit: %w", err)
	}
	return result, nil
}

const sqlGetActiveSubscription = `
SELECT id::text, org_id, plan_id, status, billing_cycle, stub, started_at, expires_at, created_at
FROM org_subscriptions
WHERE org_id = $1 AND status = 'active'
LIMIT 1
`

func (r *subscriptionRepoPG) GetActiveByOrgID(ctx context.Context, orgID string) (*domain.OrgSubscription, error) {
	row := r.pool.QueryRow(ctx, sqlGetActiveSubscription, orgID)
	result, err := scanSubscription(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return result, err
}

func scanSubscription(row pgx.Row) (*domain.OrgSubscription, error) {
	var s domain.OrgSubscription
	var status, billingCycle string
	err := row.Scan(&s.ID, &s.OrgID, &s.PlanID, &status, &billingCycle,
		&s.Stub, &s.StartedAt, &s.ExpiresAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	s.Status = domain.SubscriptionStatus(status)
	s.BillingCycle = domain.BillingCycle(billingCycle)
	return &s, nil
}
