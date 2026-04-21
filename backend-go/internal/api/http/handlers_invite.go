package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// InviteHandlers 邀请 HTTP 处理器。
type InviteHandlers struct {
	Invites *service.InviteService
}

type createInviteReq struct {
	Role       string `json:"role"`        // "NAVIGATOR" | "PASSENGER" | "OBSERVER"
	MaxUses    int    `json:"max_uses"`    // 1..10000
	TTLSeconds int    `json:"ttl_seconds"` // 1..365d
}

type inviteResp struct {
	ID        string     `json:"id"`
	LakeID    string     `json:"lake_id"`
	Token     string     `json:"token"`
	CreatedBy string     `json:"created_by"`
	Role      string     `json:"role"`
	MaxUses   int        `json:"max_uses"`
	UsedCount int        `json:"used_count"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func toInviteResp(inv *domain.Invite) inviteResp {
	return inviteResp{
		ID:        inv.ID,
		LakeID:    inv.LakeID,
		Token:     inv.Token,
		CreatedBy: inv.CreatedBy,
		Role:      string(inv.Role),
		MaxUses:   inv.MaxUses,
		UsedCount: inv.UsedCount,
		ExpiresAt: inv.ExpiresAt,
		RevokedAt: inv.RevokedAt,
		CreatedAt: inv.CreatedAt,
	}
}

// Create POST /api/v1/lakes/{id}/invites
func (h *InviteHandlers) Create(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	lakeID := chi.URLParam(r, "id")
	var in createInviteReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	inv, err := h.Invites.Create(r.Context(), u, service.CreateInviteInput{
		LakeID:  lakeID,
		Role:    domain.Role(in.Role),
		MaxUses: in.MaxUses,
		TTL:     time.Duration(in.TTLSeconds) * time.Second,
	})
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toInviteResp(inv))
}

// ListByLake GET /api/v1/lakes/{id}/invites?include_inactive=false
func (h *InviteHandlers) ListByLake(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	lakeID := chi.URLParam(r, "id")
	includeInactive, _ := strconv.ParseBool(r.URL.Query().Get("include_inactive"))
	list, err := h.Invites.ListByLake(r.Context(), u, lakeID, includeInactive)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	out := make([]inviteResp, 0, len(list))
	for i := range list {
		out = append(out, toInviteResp(&list[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"invites": out})
}

// Revoke DELETE /api/v1/lake-invites/{id}
func (h *InviteHandlers) Revoke(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.Invites.Revoke(r.Context(), u, id); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type previewResp struct {
	LakeID    string    `json:"lake_id"`
	LakeName  string    `json:"lake_name"`
	InviterID string    `json:"inviter_id"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
	UsedCount int       `json:"used_count"`
	MaxUses   int       `json:"max_uses"`
	Alive     bool      `json:"alive"`
}

// Preview GET /api/v1/invites/preview?token=xxx
func (h *InviteHandlers) Preview(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "token required")
		return
	}
	p, err := h.Invites.Preview(r.Context(), token)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, previewResp{
		LakeID:    p.LakeID,
		LakeName:  p.LakeName,
		InviterID: p.InviterID,
		Role:      string(p.Role),
		ExpiresAt: p.ExpiresAt,
		UsedCount: p.UsedCount,
		MaxUses:   p.MaxUses,
		Alive:     p.Alive,
	})
}

type acceptReq struct {
	Token string `json:"token"`
}

type acceptResp struct {
	LakeID        string `json:"lake_id"`
	Role          string `json:"role"`
	AlreadyMember bool   `json:"already_member"`
}

// Accept POST /api/v1/invites/accept
func (h *InviteHandlers) Accept(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	var in acceptReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	res, err := h.Invites.Accept(r.Context(), u, in.Token)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, acceptResp{
		LakeID:        res.LakeID,
		Role:          string(res.Role),
		AlreadyMember: res.AlreadyMember,
	})
}
