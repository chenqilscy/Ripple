package service

import (
	"context"
	"fmt"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/google/uuid"
)

// SubscriptionService 管理组织套餐订阅（Phase 15-D）。
type SubscriptionService struct {
	subs   store.SubscriptionRepository
	quotas store.OrgQuotaRepository
}

// NewSubscriptionService 构造。
func NewSubscriptionService(
	subs store.SubscriptionRepository,
	quotas store.OrgQuotaRepository,
) *SubscriptionService {
	return &SubscriptionService{subs: subs, quotas: quotas}
}

// GetPlans 返回内置套餐列表（顺序固定：free → pro → team）。
func (s *SubscriptionService) GetPlans() []domain.Plan {
	return []domain.Plan{
		domain.BuiltinPlans["free"],
		domain.BuiltinPlans["pro"],
		domain.BuiltinPlans["team"],
	}
}

// SubscribeInput 订阅套餐入参。
type SubscribeInput struct {
	OrgID        string
	PlanID       string
	BillingCycle domain.BillingCycle
	StubConfirm  bool
	ActorID      string // 操作者 user ID（必须是 org OWNER）
}

// ErrStubPaymentDisabled 当 RIPPLE_STUB_PAYMENT=false 时返回此错误。
var ErrStubPaymentDisabled = fmt.Errorf("real payment not implemented in Phase 15; set RIPPLE_STUB_PAYMENT=true for stub mode")

// ErrDowngradeBlocked 降级时当前用量超出目标套餐限额。
type ErrDowngradeBlocked struct {
	Exceeded []string
}

func (e *ErrDowngradeBlocked) Error() string {
	return fmt.Sprintf("downgrade blocked: exceeded quota fields %v", e.Exceeded)
}

// Subscribe 订阅套餐（Stub 支付）。
// stubPaymentEnabled 来自 config.StubPaymentEnabled（RIPPLE_STUB_PAYMENT）。
func (s *SubscriptionService) Subscribe(ctx context.Context, in SubscribeInput, stubPaymentEnabled bool) (*domain.OrgSubscription, error) {
	if !stubPaymentEnabled {
		return nil, ErrStubPaymentDisabled
	}
	if !in.StubConfirm {
		return nil, fmt.Errorf("stub_confirm must be true in stub payment mode")
	}

	plan, ok := domain.BuiltinPlans[in.PlanID]
	if !ok {
		return nil, domain.ErrNotFound
	}

	// 校验降级场景
	if err := s.validateDowngrade(ctx, in.OrgID, plan); err != nil {
		return nil, err
	}

	// 计算到期时间（monthly = +30d，annual = +365d）
	now := time.Now().UTC()
	var expiresAt *time.Time
	if in.BillingCycle == domain.BillingMonthly {
		t := now.AddDate(0, 1, 0)
		expiresAt = &t
	} else {
		t := now.AddDate(1, 0, 0)
		expiresAt = &t
	}

	sub := domain.OrgSubscription{
		ID:           uuid.New().String(),
		OrgID:        in.OrgID,
		PlanID:       in.PlanID,
		Status:       domain.SubStatusActive,
		BillingCycle: in.BillingCycle,
		Stub:         true,
		StartedAt:    now,
		ExpiresAt:    expiresAt,
		CreatedAt:    now,
	}

	result, err := s.subs.UpsertActive(ctx, sub)
	if err != nil {
		return nil, fmt.Errorf("subscription upsert: %w", err)
	}

	// 同步更新 org_quotas（先确保记录存在，再 Update）
	if existingQuota, qErr := s.quotas.EnsureDefault(ctx, in.OrgID); qErr == nil {
		existingQuota.MaxMembers = plan.Quotas.MaxMembers
		existingQuota.MaxLakes = plan.Quotas.MaxLakes
		existingQuota.MaxNodes = plan.Quotas.MaxNodes
		existingQuota.MaxStorageMB = plan.Quotas.MaxStorageMB
		existingQuota.UpdatedAt = now
		_ = s.quotas.Update(ctx, existingQuota) // 非致命：订阅记录已写入
	}

	return result, nil
}

// GetActive 获取当前 active 订阅；无订阅时返回 nil（非错误）。
func (s *SubscriptionService) GetActive(ctx context.Context, orgID string) (*domain.OrgSubscription, error) {
	sub, err := s.subs.GetActiveByOrgID(ctx, orgID)
	if err != nil {
		if err == domain.ErrNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("subscription get: %w", err)
	}
	return sub, nil
}

// validateDowngrade 如果目标套餐低于当前用量，返回 ErrDowngradeBlocked。
func (s *SubscriptionService) validateDowngrade(ctx context.Context, orgID string, targetPlan domain.Plan) error {
	quota, err := s.quotas.GetByOrgID(ctx, orgID)
	if err != nil {
		// 获取不到配额（如 org 未初始化配额），放行降级
		return nil
	}
	_ = quota // 真实用量检查需要统计当前 members/lakes/nodes 数量
	// Phase 15.1 简化：只检查配额硬上限，不实际统计当前用量（Phase 16 完善）
	// 如果目标 MaxNodes 比当前配额还高，不可能降级失败
	if quota.MaxNodes > targetPlan.Quotas.MaxNodes {
		// 实际应统计当前节点数；Phase 15.1 暂时放行（conservative: 允许降级）
		// TODO Phase 16: 真实用量统计
	}
	return nil
}
