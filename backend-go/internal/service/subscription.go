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
	orgs   store.OrgRepository   // Phase 16: 成员数真实用量
	lakes  store.LakeRepository  // Phase 16: 湖数真实用量
	nodes  store.NodeRepository  // Phase 16: 节点数真实用量
}

// NewSubscriptionService 构造。
func NewSubscriptionService(
	subs store.SubscriptionRepository,
	quotas store.OrgQuotaRepository,
) *SubscriptionService {
	return &SubscriptionService{subs: subs, quotas: quotas}
}

// WithUsageRepos 注入真实用量仓储（Phase 16）；返回 self 支持链式调用。
func (s *SubscriptionService) WithUsageRepos(orgs store.OrgRepository, lakes store.LakeRepository, nodes store.NodeRepository) *SubscriptionService {
	s.orgs = orgs
	s.lakes = lakes
	s.nodes = nodes
	return s
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

// GetRealUsage 查询组织真实资源用量（Phase 16）。
// 若任意仓储未注入（WithUsageRepos 未调用），对应字段返回 0。
func (s *SubscriptionService) GetRealUsage(ctx context.Context, orgID string) (*domain.OrgUsage, error) {
	usage := &domain.OrgUsage{}
	if s.orgs != nil {
		n, err := s.orgs.CountMembersByOrg(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("count members: %w", err)
		}
		usage.Members = n
	}
	if s.lakes != nil {
		n, err := s.lakes.CountByOrg(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("count lakes: %w", err)
		}
		usage.Lakes = n
	}
	if s.nodes != nil {
		n, err := s.nodes.CountByOrg(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("count nodes: %w", err)
		}
		usage.Nodes = n
	}
	return usage, nil
}

// validateDowngrade 如果目标套餐低于当前用量，返回 ErrDowngradeBlocked。
// Phase 16：使用真实用量（members/lakes/nodes count）而非配额配置值。
// 若用量仓储未注入（Phase 15 兼容模式），回退到与配额值的比较。
func (s *SubscriptionService) validateDowngrade(ctx context.Context, orgID string, targetPlan domain.Plan) error {
	// Phase 16：有真实用量仓储时，使用实际数据
	if s.orgs != nil || s.lakes != nil || s.nodes != nil {
		usage, err := s.GetRealUsage(ctx, orgID)
		if err != nil {
			// 查询失败：放行，由调用方记录日志
			return nil
		}
		var exceeded []string
		if targetPlan.Quotas.MaxMembers >= 0 && usage.Members > targetPlan.Quotas.MaxMembers {
			exceeded = append(exceeded, "max_members")
		}
		if targetPlan.Quotas.MaxLakes >= 0 && usage.Lakes > targetPlan.Quotas.MaxLakes {
			exceeded = append(exceeded, "max_lakes")
		}
		if targetPlan.Quotas.MaxNodes >= 0 && usage.Nodes > targetPlan.Quotas.MaxNodes {
			exceeded = append(exceeded, "max_nodes")
		}
		if len(exceeded) > 0 {
			return &ErrDowngradeBlocked{Exceeded: exceeded}
		}
		return nil
	}

	// Phase 15 兼容模式（无 usage repos）：与当前已分配配额比较
	quota, err := s.quotas.GetByOrgID(ctx, orgID)
	if err != nil {
		return nil
	}
	var exceeded []string
	if targetPlan.Quotas.MaxNodes < quota.MaxNodes {
		exceeded = append(exceeded, "max_nodes")
	}
	if targetPlan.Quotas.MaxLakes < quota.MaxLakes {
		exceeded = append(exceeded, "max_lakes")
	}
	if targetPlan.Quotas.MaxMembers < quota.MaxMembers {
		exceeded = append(exceeded, "max_members")
	}
	if len(exceeded) > 0 {
		return &ErrDowngradeBlocked{Exceeded: exceeded}
	}
	return nil
}
