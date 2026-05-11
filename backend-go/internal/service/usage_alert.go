package service

import (
	"context"

	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// UsageAlert 用量告警配置（Phase 15.2）。
type UsageAlert struct {
	OrgID            string
	ThresholdPercent int
	Enabled          bool
	LastTriggeredAt  *string
}

// UsageAlertService 提供组织用量告警配置管理（Phase 15.2）。
type UsageAlertService struct {
	Repo store.UsageAlertRepository
}

// NewUsageAlertService 构造。
func NewUsageAlertService(repo store.UsageAlertRepository) *UsageAlertService {
	return &UsageAlertService{Repo: repo}
}

// GetAlert 获取组织用量告警配置。若未设置则返回 nil。
func (s *UsageAlertService) GetAlert(ctx context.Context, orgID string) (*UsageAlert, error) {
	a, err := s.Repo.GetByOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, nil // 未设置，使用默认值
	}
	return &UsageAlert{
		OrgID:            a.OrgID,
		ThresholdPercent: a.ThresholdPercent,
		Enabled:          a.Enabled,
		LastTriggeredAt:  a.LastTriggeredAt,
	}, nil
}

// UpdateAlert 更新组织用量告警配置。
func (s *UsageAlertService) UpdateAlert(ctx context.Context, orgID string, threshold int, enabled bool) (*UsageAlert, error) {
	a, err := s.Repo.Upsert(ctx, orgID, threshold, enabled)
	if err != nil {
		return nil, err
	}
	return &UsageAlert{
		OrgID:            a.OrgID,
		ThresholdPercent: a.ThresholdPercent,
		Enabled:          a.Enabled,
		LastTriggeredAt:  a.LastTriggeredAt,
	}, nil
}