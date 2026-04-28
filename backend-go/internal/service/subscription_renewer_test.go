package service

import (
	"context"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/rs/zerolog"
)

// memSubRepoR17 是 SubscriptionRepository 的内存测试桩（Phase 17）。
type memSubRepoR17 struct {
	subs         []*domain.OrgSubscription
	extendCalls  []extendCall17
	statusCalls  []statusCall17
}

type extendCall17 struct {
	id     string
	expiry time.Time
}

type statusCall17 struct {
	id     string
	status domain.SubscriptionStatus
}

func (r *memSubRepoR17) UpsertActive(_ context.Context, sub domain.OrgSubscription) (*domain.OrgSubscription, error) {
	cp := sub
	r.subs = append(r.subs, &cp)
	return &cp, nil
}

func (r *memSubRepoR17) GetActiveByOrgID(_ context.Context, orgID string) (*domain.OrgSubscription, error) {
	for _, s := range r.subs {
		if s.OrgID == orgID && s.Status == domain.SubStatusActive {
			cp := *s
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *memSubRepoR17) ListExpiringActive(_ context.Context, warnDays int) ([]domain.OrgSubscription, error) {
	now := time.Now()
	limit := now.AddDate(0, 0, warnDays)
	var out []domain.OrgSubscription
	for _, s := range r.subs {
		if s.Status == domain.SubStatusActive && s.ExpiresAt != nil {
			// expires_at > now（未过期）且 expires_at <= limit（在警告窗口内）
			if s.ExpiresAt.After(now) && !s.ExpiresAt.After(limit) {
				out = append(out, *s)
			}
		}
	}
	return out, nil
}

func (r *memSubRepoR17) ListExpiredActive(_ context.Context) ([]domain.OrgSubscription, error) {
	now := time.Now()
	var out []domain.OrgSubscription
	for _, s := range r.subs {
		if s.Status == domain.SubStatusActive && s.ExpiresAt != nil && s.ExpiresAt.Before(now) {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (r *memSubRepoR17) UpdateStatusByID(_ context.Context, id string, status domain.SubscriptionStatus) error {
	r.statusCalls = append(r.statusCalls, statusCall17{id: id, status: status})
	for _, s := range r.subs {
		if s.ID == id {
			s.Status = status
		}
	}
	return nil
}

func (r *memSubRepoR17) ExtendExpiry(_ context.Context, id string, newExpiry time.Time) error {
	r.extendCalls = append(r.extendCalls, extendCall17{id: id, expiry: newExpiry})
	for _, s := range r.subs {
		if s.ID == id {
			t := newExpiry
			s.ExpiresAt = &t
		}
	}
	return nil
}

// buildR17Sub 构造测试用订阅记录。
func buildR17Sub(id string, expiresAt time.Time, stub bool, cycle domain.BillingCycle) domain.OrgSubscription {
	return domain.OrgSubscription{
		ID:           id,
		OrgID:        "org-1",
		PlanID:       "pro",
		Status:       domain.SubStatusActive,
		BillingCycle: cycle,
		Stub:         stub,
		StartedAt:    time.Now().AddDate(0, -1, 0),
		ExpiresAt:    &expiresAt,
	}
}

// TestSubscriptionRenewerStubMonthlyRenewal 验证 monthly stub 到期 → 自动续期 +1 月。
func TestSubscriptionRenewerStubMonthlyRenewal(t *testing.T) {
	past := time.Now().Add(-time.Minute)
	sub := buildR17Sub("sub-stub-m", past, true, domain.BillingMonthly)
	subRepo := &memSubRepoR17{subs: []*domain.OrgSubscription{&sub}}
	notifRepo := newMemNotificationRepo()

	renewer := NewSubscriptionRenewer(subRepo, &stubOrgRepo{}, notifRepo, zerolog.Nop(), time.Hour, 7)
	renewer.tick(context.Background())

	if len(subRepo.extendCalls) != 1 {
		t.Fatalf("want 1 ExtendExpiry, got %d", len(subRepo.extendCalls))
	}
	if subRepo.extendCalls[0].id != "sub-stub-m" {
		t.Fatalf("ExtendExpiry wrong id: %s", subRepo.extendCalls[0].id)
	}
	// 新到期时间应在约 30 天后（允许 1min 误差）
	expectedExpiry := time.Now().AddDate(0, 1, 0)
	diff := subRepo.extendCalls[0].expiry.Sub(expectedExpiry)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Minute {
		t.Fatalf("new expiry not ~1 month: diff=%v", diff)
	}
	// stub 续期不触发通知
	if len(notifRepo.items) != 0 {
		t.Fatalf("want 0 notifications for stub renewal, got %d", len(notifRepo.items))
	}
	// UpdateStatusByID 不应被调用
	if len(subRepo.statusCalls) != 0 {
		t.Fatalf("want 0 status updates for stub renewal, got %d", len(subRepo.statusCalls))
	}
}

// TestSubscriptionRenewerStubAnnualRenewal 验证 annual stub 到期 → 续期 +1 年。
func TestSubscriptionRenewerStubAnnualRenewal(t *testing.T) {
	past := time.Now().Add(-time.Minute)
	sub := buildR17Sub("sub-stub-a", past, true, domain.BillingAnnual)
	subRepo := &memSubRepoR17{subs: []*domain.OrgSubscription{&sub}}
	notifRepo := newMemNotificationRepo()

	renewer := NewSubscriptionRenewer(subRepo, &stubOrgRepo{}, notifRepo, zerolog.Nop(), time.Hour, 7)
	renewer.tick(context.Background())

	if len(subRepo.extendCalls) != 1 {
		t.Fatalf("want 1 ExtendExpiry, got %d", len(subRepo.extendCalls))
	}
	expectedExpiry := time.Now().AddDate(1, 0, 0)
	diff := subRepo.extendCalls[0].expiry.Sub(expectedExpiry)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Minute {
		t.Fatalf("new expiry not ~1 year: diff=%v", diff)
	}
}

// TestSubscriptionRenewerRealSubExpired 验证真实订阅到期 → 状态改为 expired + 通知 OWNER。
func TestSubscriptionRenewerRealSubExpired(t *testing.T) {
	past := time.Now().Add(-time.Minute)
	sub := buildR17Sub("sub-real", past, false, domain.BillingMonthly)
	subRepo := &memSubRepoR17{subs: []*domain.OrgSubscription{&sub}}
	notifRepo := newMemNotificationRepo()

	renewer := NewSubscriptionRenewer(subRepo, &stubOrgRepo{}, notifRepo, zerolog.Nop(), time.Hour, 7)
	renewer.tick(context.Background())

	// 状态应被改为 expired
	if len(subRepo.statusCalls) != 1 {
		t.Fatalf("want 1 UpdateStatusByID, got %d", len(subRepo.statusCalls))
	}
	if subRepo.statusCalls[0].id != "sub-real" {
		t.Fatalf("UpdateStatusByID wrong id: %s", subRepo.statusCalls[0].id)
	}
	if subRepo.statusCalls[0].status != domain.SubStatusExpired {
		t.Fatalf("want expired, got %s", subRepo.statusCalls[0].status)
	}
	// 不触发 ExtendExpiry
	if len(subRepo.extendCalls) != 0 {
		t.Fatalf("want 0 ExtendExpiry for real sub, got %d", len(subRepo.extendCalls))
	}
	// 通知发给 OWNER（stubOrgRepo.GetByID → OwnerID="owner"）
	if len(notifRepo.items) != 1 {
		t.Fatalf("want 1 notification, got %d", len(notifRepo.items))
	}
	n := notifRepo.items[0]
	if n.Type != "subscription_expired" {
		t.Fatalf("want subscription_expired, got %s", n.Type)
	}
	if n.UserID != "owner" {
		t.Fatalf("want notification to owner, got %s", n.UserID)
	}
}

// TestSubscriptionRenewerExpiringNotification 验证即将到期 → 发送 subscription_expiring 通知。
func TestSubscriptionRenewerExpiringNotification(t *testing.T) {
	// 3 天后到期，在 7 天警告窗口内
	expiresAt := time.Now().Add(3 * 24 * time.Hour)
	sub := buildR17Sub("sub-expiring", expiresAt, false, domain.BillingMonthly)
	subRepo := &memSubRepoR17{subs: []*domain.OrgSubscription{&sub}}
	notifRepo := newMemNotificationRepo()

	renewer := NewSubscriptionRenewer(subRepo, &stubOrgRepo{}, notifRepo, zerolog.Nop(), time.Hour, 7)
	renewer.tick(context.Background())

	if len(notifRepo.items) != 1 {
		t.Fatalf("want 1 expiring notification, got %d", len(notifRepo.items))
	}
	if notifRepo.items[0].Type != "subscription_expiring" {
		t.Fatalf("want subscription_expiring, got %s", notifRepo.items[0].Type)
	}
	// 验证去重 key 已记录
	dedupeKey := "sub-expiring@" + time.Now().UTC().Format("2006-01-02")
	if _, ok := renewer.notifiedToday[dedupeKey]; !ok {
		t.Fatalf("dedup key not recorded after notification")
	}
	// 状态不应改变
	if len(subRepo.statusCalls) != 0 {
		t.Fatalf("expiring sub should not change status, got %d calls", len(subRepo.statusCalls))
	}
}

// TestSubscriptionRenewerExpiringNotificationDedup 验证同一天同一订阅只发 1 次通知。
func TestSubscriptionRenewerExpiringNotificationDedup(t *testing.T) {
	expiresAt := time.Now().Add(3 * 24 * time.Hour)
	sub := buildR17Sub("sub-dedup", expiresAt, false, domain.BillingMonthly)
	subRepo := &memSubRepoR17{subs: []*domain.OrgSubscription{&sub}}
	notifRepo := newMemNotificationRepo()

	renewer := NewSubscriptionRenewer(subRepo, &stubOrgRepo{}, notifRepo, zerolog.Nop(), time.Hour, 7)
	renewer.tick(context.Background()) // 第 1 次
	renewer.tick(context.Background()) // 第 2 次（同一天）

	if len(notifRepo.items) != 1 {
		t.Fatalf("want 1 notification (dedup), got %d", len(notifRepo.items))
	}
}

// TestSubscriptionRenewerNoExpiredNoExpiring 验证无到期/即将到期时无任何副作用。
func TestSubscriptionRenewerNoExpiredNoExpiring(t *testing.T) {
	// 订阅 30 天后才到期（不在 7 天窗口内）
	future := time.Now().Add(30 * 24 * time.Hour)
	sub := buildR17Sub("sub-future", future, false, domain.BillingMonthly)
	subRepo := &memSubRepoR17{subs: []*domain.OrgSubscription{&sub}}
	notifRepo := newMemNotificationRepo()

	renewer := NewSubscriptionRenewer(subRepo, &stubOrgRepo{}, notifRepo, zerolog.Nop(), time.Hour, 7)
	renewer.tick(context.Background())

	if len(subRepo.extendCalls) != 0 {
		t.Fatalf("want 0 ExtendExpiry, got %d", len(subRepo.extendCalls))
	}
	if len(subRepo.statusCalls) != 0 {
		t.Fatalf("want 0 status updates, got %d", len(subRepo.statusCalls))
	}
	if len(notifRepo.items) != 0 {
		t.Fatalf("want 0 notifications, got %d", len(notifRepo.items))
	}
}
