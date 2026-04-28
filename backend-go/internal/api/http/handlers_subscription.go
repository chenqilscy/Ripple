package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// SubscriptionHandlers Phase 15-D：套餐订阅端点。
type SubscriptionHandlers struct {
	Svc                *service.SubscriptionService
	StubPaymentEnabled bool
}

type planResp struct {
	ID              string `json:"id"`
	NameZH          string `json:"name_zh"`
	PriceCNYMonthly int    `json:"price_cny_monthly"`
	Quotas          struct {
		MaxMembers   int64 `json:"max_members"`
		MaxLakes     int64 `json:"max_lakes"`
		MaxNodes     int64 `json:"max_nodes"`
		MaxStorageMB int64 `json:"max_storage_mb"`
	} `json:"quotas"`
}

func toPlanResp(p domain.Plan) planResp {
	r := planResp{
		ID:              p.ID,
		NameZH:          p.NameZH,
		PriceCNYMonthly: p.PriceCNYMonthly,
	}
	r.Quotas.MaxMembers = p.Quotas.MaxMembers
	r.Quotas.MaxLakes = p.Quotas.MaxLakes
	r.Quotas.MaxNodes = p.Quotas.MaxNodes
	r.Quotas.MaxStorageMB = p.Quotas.MaxStorageMB
	return r
}

// GetPlans GET /api/v1/subscriptions/plans
func (h *SubscriptionHandlers) GetPlans(w http.ResponseWriter, r *http.Request) {
	plans := h.Svc.GetPlans()
	resp := make([]planResp, len(plans))
	for i, p := range plans {
		resp[i] = toPlanResp(p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": resp})
}

type subscriptionResp struct {
	ID           string     `json:"id"`
	OrgID        string     `json:"org_id"`
	PlanID       string     `json:"plan_id"`
	Status       string     `json:"status"`
	BillingCycle string     `json:"billing_cycle"`
	Stub         bool       `json:"stub"`
	StartedAt    time.Time  `json:"started_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

func toSubResp(s *domain.OrgSubscription) subscriptionResp {
	return subscriptionResp{
		ID:           s.ID,
		OrgID:        s.OrgID,
		PlanID:       s.PlanID,
		Status:       string(s.Status),
		BillingCycle: string(s.BillingCycle),
		Stub:         s.Stub,
		StartedAt:    s.StartedAt,
		ExpiresAt:    s.ExpiresAt,
		CreatedAt:    s.CreatedAt,
	}
}

// GetSubscription GET /api/v1/organizations/{id}/subscription
func (h *SubscriptionHandlers) GetSubscription(w http.ResponseWriter, r *http.Request) {
	_, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	orgID := chi.URLParam(r, "id")

	sub, err := h.Svc.GetActive(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get subscription")
		return
	}
	if sub == nil {
		writeJSON(w, http.StatusOK, map[string]any{"subscription": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subscription": toSubResp(sub)})
}

type createSubReq struct {
	PlanID       string `json:"plan_id"`
	BillingCycle string `json:"billing_cycle"` // "monthly" | "annual"
	StubConfirm  bool   `json:"stub_confirm"`
}

// CreateSubscription POST /api/v1/organizations/{id}/subscription
func (h *SubscriptionHandlers) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	orgID := chi.URLParam(r, "id")

	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var in createSubReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.PlanID == "" {
		writeError(w, http.StatusBadRequest, "plan_id is required")
		return
	}
	cycle := domain.BillingCycle(in.BillingCycle)
	if cycle != domain.BillingMonthly && cycle != domain.BillingAnnual {
		cycle = domain.BillingMonthly
	}

	sub, err := h.Svc.Subscribe(r.Context(), service.SubscribeInput{
		OrgID:        orgID,
		PlanID:       in.PlanID,
		BillingCycle: cycle,
		StubConfirm:  in.StubConfirm,
		ActorID:      u.ID,
	}, h.StubPaymentEnabled)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrStubPaymentDisabled):
			writeError(w, http.StatusPaymentRequired, err.Error())
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusBadRequest, "unknown plan_id")
		default:
			var downErr *service.ErrDowngradeBlocked
			if errors.As(err, &downErr) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to create subscription")
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"subscription": toSubResp(sub)})
}
