package store

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	// ListExpiringActive 返回在 [now, now+warnDays] 窗口内到期的 active 订阅（Phase 17）。
	ListExpiringActive(ctx context.Context, warnDays int) ([]domain.OrgSubscription, error)
	// ListExpiredActive 返回已逾期但仍标记为 active 的订阅（Phase 17）。
	ListExpiredActive(ctx context.Context) ([]domain.OrgSubscription, error)
	// UpdateStatusByID 更新订阅状态（Phase 17）。
	UpdateStatusByID(ctx context.Context, id string, status domain.SubscriptionStatus) error
	// ExtendExpiry 延长订阅到期时间（Phase 17 stub 续期）。
	ExtendExpiry(ctx context.Context, id string, newExpiry time.Time) error
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

// --- Phase 17 方法 ---

const sqlListExpiringActive = `
SELECT id::text, org_id, plan_id, status, billing_cycle, stub, started_at, expires_at, created_at
FROM org_subscriptions
WHERE status = 'active'
  AND expires_at IS NOT NULL
  AND expires_at > NOW()
  AND expires_at <= NOW() + ($1 || ' days')::INTERVAL
ORDER BY expires_at ASC
`

func (r *subscriptionRepoPG) ListExpiringActive(ctx context.Context, warnDays int) ([]domain.OrgSubscription, error) {
	rows, err := r.pool.Query(ctx, sqlListExpiringActive, fmt.Sprintf("%d", warnDays))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubscriptions(rows)
}

const sqlListExpiredActive = `
SELECT id::text, org_id, plan_id, status, billing_cycle, stub, started_at, expires_at, created_at
FROM org_subscriptions
WHERE status = 'active'
  AND expires_at IS NOT NULL
  AND expires_at < NOW()
ORDER BY expires_at ASC
`

func (r *subscriptionRepoPG) ListExpiredActive(ctx context.Context) ([]domain.OrgSubscription, error) {
	rows, err := r.pool.Query(ctx, sqlListExpiredActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubscriptions(rows)
}

func (r *subscriptionRepoPG) UpdateStatusByID(ctx context.Context, id string, status domain.SubscriptionStatus) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE org_subscriptions SET status = $1 WHERE id = $2::uuid`,
		string(status), id,
	)
	return err
}

func (r *subscriptionRepoPG) ExtendExpiry(ctx context.Context, id string, newExpiry time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE org_subscriptions SET expires_at = $1, started_at = NOW() WHERE id = $2::uuid AND status = 'active'`,
		newExpiry, id,
	)
	return err
}

func scanSubscriptions(rows pgx.Rows) ([]domain.OrgSubscription, error) {
	var out []domain.OrgSubscription
	for rows.Next() {
		var s domain.OrgSubscription
		var status, billingCycle string
		if err := rows.Scan(&s.ID, &s.OrgID, &s.PlanID, &status, &billingCycle,
			&s.Stub, &s.StartedAt, &s.ExpiresAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		s.Status = domain.SubscriptionStatus(status)
		s.BillingCycle = domain.BillingCycle(billingCycle)
		out = append(out, s)
	}
	return out, rows.Err()
}
