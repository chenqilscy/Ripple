package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

// PlatformAdminHandlers 管理平台管理员 RBAC 授权。
type PlatformAdminHandlers struct {
	Repo        store.PlatformAdminRepository
	Users       store.UserRepository
	AuditLogs   store.AuditLogRepository
	AdminEmails map[string]struct{}
}

type platformAdminResp struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email,omitempty"`
	Role      string    `json:"role"`
	Note      string    `json:"note"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

func toPlatformAdminResp(ctx context.Context, users store.UserRepository, admin domain.PlatformAdmin) platformAdminResp {
	resp := platformAdminResp{
		UserID:    admin.UserID,
		Role:      string(admin.Role),
		Note:      admin.Note,
		CreatedBy: admin.CreatedBy,
		CreatedAt: admin.CreatedAt,
	}
	if users != nil {
		if user, err := users.GetByID(ctx, admin.UserID); err == nil && user != nil {
			resp.Email = user.Email
		}
	}
	return resp
}

// List GET /api/v1/admin/platform_admins
func (h *PlatformAdminHandlers) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if ok := h.requireOwner(w, r, actor); !ok {
		return
	}
	admins, err := h.Repo.ListActive(r.Context(), 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list platform admins failed")
		return
	}
	out := make([]platformAdminResp, 0, len(admins))
	for _, admin := range admins {
		out = append(out, toPlatformAdminResp(r.Context(), h.Users, admin))
	}
	writeJSON(w, http.StatusOK, map[string]any{"admins": out})
}

// Grant POST /api/v1/admin/platform_admins
func (h *PlatformAdminHandlers) Grant(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if ok := h.requireOwner(w, r, actor); !ok {
		return
	}
	var body struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
		Role   string `json:"role"`
		Note   string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	user, err := h.resolveTargetUser(r.Context(), strings.TrimSpace(body.UserID), body.Email)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	role := domain.PlatformAdminRoleAdmin
	if strings.TrimSpace(body.Role) != "" {
		role = domain.PlatformAdminRole(strings.ToUpper(strings.TrimSpace(body.Role)))
	}
	if !role.IsValid() {
		writeError(w, http.StatusBadRequest, "invalid role")
		return
	}
	admin := &domain.PlatformAdmin{
		UserID:    user.ID,
		Role:      role,
		Note:      strings.TrimSpace(body.Note),
		CreatedBy: actor.ID,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.Repo.Grant(r.Context(), admin); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	if err := h.writeAudit(r.Context(), actor.ID, domain.AuditPlatformAdminGrant, user.ID, map[string]any{
		"email": user.Email,
		"role":  string(role),
		"note":  admin.Note,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "platform admin granted but audit failed")
		return
	}
	writeJSON(w, http.StatusOK, toPlatformAdminResp(r.Context(), h.Users, *admin))
}

// Revoke DELETE /api/v1/admin/platform_admins/{user_id}
func (h *PlatformAdminHandlers) Revoke(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if ok := h.requireOwner(w, r, actor); !ok {
		return
	}
	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user_id required")
		return
	}
	if userID == actor.ID && !isPlatformAdminEmail(actor.Email, h.AdminEmails) {
		writeError(w, http.StatusBadRequest, "cannot revoke current owner")
		return
	}
	if err := h.Repo.Revoke(r.Context(), userID, time.Now().UTC()); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	if err := h.writeAudit(r.Context(), actor.ID, domain.AuditPlatformAdminRevoke, userID, map[string]any{}); err != nil {
		writeError(w, http.StatusInternalServerError, "platform admin revoked but audit failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PlatformAdminHandlers) requireOwner(w http.ResponseWriter, r *http.Request, actor *domain.User) bool {
	if h.Repo == nil {
		writeError(w, http.StatusServiceUnavailable, "platform admin repository not configured")
		return false
	}
	ok, err := isPlatformOwner(r.Context(), actor, h.AdminEmails, h.Repo)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "platform owner check failed")
		return false
	}
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden")
		return false
	}
	return true
}

func (h *PlatformAdminHandlers) resolveTargetUser(ctx context.Context, userID, email string) (*domain.User, error) {
	if h.Users == nil {
		return nil, domain.ErrInvalidInput
	}
	if userID != "" {
		return h.Users.GetByID(ctx, userID)
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, domain.ErrInvalidInput
	}
	return h.Users.GetByEmail(ctx, email)
}

func (h *PlatformAdminHandlers) writeAudit(ctx context.Context, actorID, action, resourceID string, detail map[string]any) error {
	if h.AuditLogs == nil {
		return nil
	}
	return h.AuditLogs.Write(ctx, &domain.AuditLog{
		ID:           platform.NewID(),
		ActorID:      actorID,
		Action:       action,
		ResourceType: "platform_admin",
		ResourceID:   resourceID,
		Detail:       detail,
		CreatedAt:    time.Now().UTC(),
	})
}
