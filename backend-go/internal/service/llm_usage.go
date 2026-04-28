package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// OrgLLMUsage 组织 LLM 用量完整汇总。
type OrgLLMUsage struct {
	OrgID                 string                 `json:"org_id"`
	PeriodDays            int                    `json:"period_days"`
	TotalCalls            int64                  `json:"total_calls"`
	TotalEstimatedCostCNY float64                `json:"total_estimated_cost_cny"`
	ByProvider            []domain.ProviderUsage `json:"by_provider"`
	ByDay                 []domain.DayUsage      `json:"by_day"`
}

// LLMUsageService 提供组织 LLM 用量聚合（Phase 15-D）。
type LLMUsageService struct {
	analytics store.LLMCallsAnalyticsRepository
}

// NewLLMUsageService 构造。
func NewLLMUsageService(analytics store.LLMCallsAnalyticsRepository) *LLMUsageService {
	return &LLMUsageService{analytics: analytics}
}

// GetOrgUsage 获取组织最近 days 天的 LLM 用量。
func (s *LLMUsageService) GetOrgUsage(ctx context.Context, orgID string, days int) (*OrgLLMUsage, error) {
	if days < 1 || days > 90 {
		return nil, fmt.Errorf("days must be between 1 and 90")
	}
	since := time.Now().UTC().AddDate(0, 0, -days)

	// 并发执行两个 DB 查询以减少响应时间
	var (
		byProvider []domain.ProviderUsage
		byDay      []domain.DayUsage
		provErr    error
		dayErr     error
		wg         sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		byProvider, provErr = s.analytics.SumByProvider(ctx, orgID, since)
	}()
	go func() {
		defer wg.Done()
		byDay, dayErr = s.analytics.SumByDay(ctx, orgID, since)
	}()
	wg.Wait()

	if provErr != nil {
		return nil, fmt.Errorf("llm usage by provider: %w", provErr)
	}
	if dayErr != nil {
		return nil, fmt.Errorf("llm usage by day: %w", dayErr)
	}

	var totalCalls int64
	var totalCost float64
	for i := range byProvider {
		byProvider[i].EstimatedCostCNY = estimateCost(byProvider[i].Provider, byProvider[i].Calls)
		totalCalls += byProvider[i].Calls
		totalCost += byProvider[i].EstimatedCostCNY
	}

	return &OrgLLMUsage{
		OrgID:                 orgID,
		PeriodDays:            days,
		TotalCalls:            totalCalls,
		TotalEstimatedCostCNY: totalCost,
		ByProvider:            byProvider,
		ByDay:                 byDay,
	}, nil
}

// estimateCost 按 provider 估算每次调用费用（仅供参考，非合同价）。
func estimateCost(provider string, calls int64) float64 {
	rates := map[string]float64{
		"zhipu":    0.01,
		"deepseek": 0.008,
		"openai":   0.02,
		"volc":     0.012,
		"minimax":  0.015,
	}
	rate, ok := rates[provider]
	if !ok {
		rate = 0.01
	}
	return float64(calls) * rate / 1000
}
