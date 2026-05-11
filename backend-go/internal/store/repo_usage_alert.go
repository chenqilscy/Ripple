package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UsageAlertRepository interface {
	GetByOrgID(ctx context.Context, orgID string) (*UsageAlert, error)
	Upsert(ctx context.Context, orgID string, threshold int, enabled bool) (*UsageAlert, error)
}

type UsageAlert struct {
	ID               string
	OrgID            string
	ThresholdPercent int
	Enabled          bool
	LastTriggeredAt  *string
	CreatedAt        string
	UpdatedAt        string
}

type usageAlertRepository struct {
	db *pgxpool.Pool
}

func NewUsageAlertRepository(db *pgxpool.Pool) UsageAlertRepository {
	return &usageAlertRepository{db: db}
}

func (r *usageAlertRepository) GetByOrgID(ctx context.Context, orgID string) (*UsageAlert, error) {
	query := `SELECT id, org_id, threshold_percent, enabled, last_triggered_at, created_at, updated_at
              FROM org_usage_alerts WHERE org_id = $1`
	var a UsageAlert
	err := r.db.QueryRow(ctx, query, orgID).Scan(
		&a.ID, &a.OrgID, &a.ThresholdPercent, &a.Enabled, &a.LastTriggeredAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &a, err
}

func (r *usageAlertRepository) Upsert(ctx context.Context, orgID string, threshold int, enabled bool) (*UsageAlert, error) {
	query := `
        INSERT INTO org_usage_alerts (org_id, threshold_percent, enabled)
        VALUES ($1, $2, $3)
        ON CONFLICT (org_id) DO UPDATE SET
            threshold_percent = EXCLUDED.threshold_percent,
            enabled = EXCLUDED.enabled,
            updated_at = NOW()
        RETURNING id, org_id, threshold_percent, enabled, last_triggered_at, created_at, updated_at`
	var a UsageAlert
	err := r.db.QueryRow(ctx, query, orgID, threshold, enabled).Scan(
		&a.ID, &a.OrgID, &a.ThresholdPercent, &a.Enabled, &a.LastTriggeredAt, &a.CreatedAt, &a.UpdatedAt,
	)
	return &a, err
}
