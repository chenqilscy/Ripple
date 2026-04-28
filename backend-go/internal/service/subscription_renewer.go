package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/rs/zerolog"
)

// SubscriptionRenewer 定期扫描订阅状态：
//   - 即将到期（≤ WarnDays 天）→ 向 org OWNER 发送 subscription_expiring 通知（每订阅每天最多 1 次）
//   - 已到期且 stub=true   → 自动续期（monthly+30d, annual+365d）
//   - 已到期且 stub=false  → 标记为 expired，发送 subscription_expired 通知
//
// （Phase 17）
type SubscriptionRenewer struct {
	subs     store.SubscriptionRepository
	orgs     store.OrgRepository
	notifs   store.NotificationRepository
	log      zerolog.Logger
	interval time.Duration
	warnDays int
	// notifiedToday 去重集合：key=subID+"@"+date(YYYY-MM-DD)；重启后重置（可接受）
	notifiedToday map[string]struct{}
}

// NewSubscriptionRenewer 构造。interval=0 时默认 1 小时。
func NewSubscriptionRenewer(
	subs store.SubscriptionRepository,
	orgs store.OrgRepository,
	notifs store.NotificationRepository,
	log zerolog.Logger,
	interval time.Duration,
	warnDays int,
) *SubscriptionRenewer {
	if interval <= 0 {
		interval = time.Hour
	}
	if warnDays <= 0 {
		warnDays = 7
	}
	return &SubscriptionRenewer{
		subs:          subs,
		orgs:          orgs,
		notifs:        notifs,
		log:           log,
		interval:      interval,
		warnDays:      warnDays,
		notifiedToday: make(map[string]struct{}),
	}
}

// Run 在 ctx 取消前持续运行（应由 goroutine 调用）。
func (r *SubscriptionRenewer) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	// 启动时立刻跑一次
	r.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}

func (r *SubscriptionRenewer) tick(ctx context.Context) {
	r.processExpiring(ctx)
	r.processExpired(ctx)
}

// processExpiring 发出即将到期通知（每订阅每日最多 1 次，UTC 日期去重）。
// 注意：notifiedToday 仅进程内去重，多实例部署时以 notifications 表重复行为代价换简单性。
func (r *SubscriptionRenewer) processExpiring(ctx context.Context) {
	today := time.Now().UTC().Format("2006-01-02")
	subs, err := r.subs.ListExpiringActive(ctx, r.warnDays)
	if err != nil {
		r.log.Error().Err(err).Msg("renewer: list expiring")
		return
	}
	for _, s := range subs {
		dedupeKey := s.ID + "@" + today
		if _, already := r.notifiedToday[dedupeKey]; already {
			continue
		}
		ownerID, err := r.getOrgOwner(ctx, s.OrgID)
		if err != nil {
			r.log.Warn().Str("org_id", s.OrgID).Err(err).Msg("renewer: get org owner for expiring")
			continue
		}
		payload := r.buildPayload(s, "expiring")
		if _, err := r.notifs.Create(ctx, ownerID, "subscription_expiring", payload); err != nil {
			r.log.Warn().Str("org_id", s.OrgID).Err(err).Msg("renewer: create expiring notification")
		} else {
			r.notifiedToday[dedupeKey] = struct{}{}
		}
	}
	// 清除昨天的记录（内存节约）
	for k := range r.notifiedToday {
		if len(k) >= 10 && k[len(k)-10:] != today {
			delete(r.notifiedToday, k)
		}
	}
}

// processExpired 处理已到期订阅：stub→续期，否则→expired + 通知。
func (r *SubscriptionRenewer) processExpired(ctx context.Context) {
	subs, err := r.subs.ListExpiredActive(ctx)
	if err != nil {
		r.log.Error().Err(err).Msg("renewer: list expired")
		return
	}
	for _, s := range subs {
		if s.Stub {
			r.renewStub(ctx, s)
		} else {
			r.expireSub(ctx, s)
		}
	}
}

// renewStub 为 stub 订阅自动续期。
func (r *SubscriptionRenewer) renewStub(ctx context.Context, s domain.OrgSubscription) {
	var next time.Time
	base := time.Now()
	switch s.BillingCycle {
	case domain.BillingAnnual:
		next = base.AddDate(1, 0, 0)
	default: // monthly
		next = base.AddDate(0, 1, 0)
	}
	if err := r.subs.ExtendExpiry(ctx, s.ID, next); err != nil {
		r.log.Warn().Str("sub_id", s.ID).Err(err).Msg("renewer: extend stub expiry")
	} else {
		r.log.Info().Str("sub_id", s.ID).Time("next_expiry", next).Msg("renewer: stub subscription renewed")
	}
}

// expireSub 将真实订阅标记为 expired 并通知 OWNER。
func (r *SubscriptionRenewer) expireSub(ctx context.Context, s domain.OrgSubscription) {
	if err := r.subs.UpdateStatusByID(ctx, s.ID, domain.SubStatusExpired); err != nil {
		r.log.Warn().Str("sub_id", s.ID).Err(err).Msg("renewer: update status expired")
		return
	}
	ownerID, err := r.getOrgOwner(ctx, s.OrgID)
	if err != nil {
		r.log.Warn().Str("org_id", s.OrgID).Err(err).Msg("renewer: get org owner for expired")
		return
	}
	payload := r.buildPayload(s, "expired")
	if _, err := r.notifs.Create(ctx, ownerID, "subscription_expired", payload); err != nil {
		r.log.Warn().Str("org_id", s.OrgID).Err(err).Msg("renewer: create expired notification")
	}
}

func (r *SubscriptionRenewer) getOrgOwner(ctx context.Context, orgID string) (string, error) {
	org, err := r.orgs.GetByID(ctx, orgID)
	if err != nil {
		return "", err
	}
	return org.OwnerID, nil
}

func (r *SubscriptionRenewer) buildPayload(s domain.OrgSubscription, event string) json.RawMessage {
	m := map[string]any{
		"org_id":       s.OrgID,
		"plan_id":      s.PlanID,
		"billing_cycle": string(s.BillingCycle),
		"event":        event,
	}
	if s.ExpiresAt != nil {
		m["expires_at"] = s.ExpiresAt.Format(time.RFC3339)
	}
	b, _ := json.Marshal(m)
	return b
}
