package domain

import "time"

// SubscriptionStatus 是订阅状态。
type SubscriptionStatus string

const (
	SubStatusActive    SubscriptionStatus = "active"
	SubStatusExpired   SubscriptionStatus = "expired"
	SubStatusCancelled SubscriptionStatus = "cancelled"
)

// BillingCycle 是计费周期。
type BillingCycle string

const (
	BillingMonthly BillingCycle = "monthly"
	BillingAnnual  BillingCycle = "annual"
)

// OrgSubscription 是组织套餐订阅记录（Phase 15-D）。
type OrgSubscription struct {
	ID           string
	OrgID        string
	PlanID       string
	Status       SubscriptionStatus
	BillingCycle BillingCycle
	Stub         bool
	StartedAt    time.Time
	ExpiresAt    *time.Time
	CreatedAt    time.Time
}

// Plan 是内置套餐定义。
type Plan struct {
	ID              string
	NameZH          string
	PriceCNYMonthly int
	Quotas          PlanQuota
}

// PlanQuota 套餐配额约束（对应 OrgQuota 字段）。
type PlanQuota struct {
	MaxMembers   int64
	MaxLakes     int64
	MaxNodes     int64
	MaxStorageMB int64
}

// BuiltinPlans 是 Phase 15 内置套餐（Phase 16 改为管理后台配置）。
var BuiltinPlans = map[string]Plan{
	"free": {
		ID:              "free",
		NameZH:          "免费版",
		PriceCNYMonthly: 0,
		Quotas: PlanQuota{
			MaxMembers:   3,
			MaxLakes:     50,
			MaxNodes:     10000,
			MaxStorageMB: 1024,
		},
	},
	"pro": {
		ID:              "pro",
		NameZH:          "专业版",
		PriceCNYMonthly: 29,
		Quotas: PlanQuota{
			MaxMembers:   20,
			MaxLakes:     500,
			MaxNodes:     100000,
			MaxStorageMB: 10240,
		},
	},
	"team": {
		ID:              "team",
		NameZH:          "团队版",
		PriceCNYMonthly: 99,
		Quotas: PlanQuota{
			MaxMembers:   100,
			MaxLakes:     5000,
			MaxNodes:     1000000,
			MaxStorageMB: 102400,
		},
	},
}

// ProviderUsage 单 provider 的 LLM 用量汇总（Phase 15-D）。
type ProviderUsage struct {
	Provider         string  `json:"provider"`
	Calls            int64   `json:"calls"`
	AvgDurationMS    int64   `json:"avg_duration_ms"`
	EstimatedCostCNY float64 `json:"estimated_cost_cny"`
}

// DayUsage 单天 LLM 用量汇总（Phase 15-D）。
type DayUsage struct {
	Date             string  `json:"date"`
	Calls            int64   `json:"calls"`
	EstimatedCostCNY float64 `json:"estimated_cost_cny"`
}
