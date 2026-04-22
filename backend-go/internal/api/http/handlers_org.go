package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// OrgHandlers 组织 HTTP 处理器（P12-C）。
type OrgHandlers struct {
	Orgs  *service.OrgService
	Lakes *service.LakeService // P13-A：列出组织湖
}

type orgResp struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	OwnerID     string    `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toOrgResp(o *domain.Organization) orgResp {
	return orgResp{
		ID: o.ID, Name: o.Name, Slug: o.Slug,
		Description: o.Description, OwnerID: o.OwnerID,
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
	}
}

type orgMemberResp struct {
	OrgID    string    `json:"org_id"`
	UserID   string    `json:"user_id"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// CreateOrg POST /api/v1/organizations
func (h *OrgHandlers) CreateOrg(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024)
	var body struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	org, err := h.Orgs.CreateOrg(r.Context(), u, service.CreateOrgInput{
		Name:        body.Name,
		Slug:        body.Slug,
		Description: body.Description,
	})
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toOrgResp(org))
}

// ListMyOrgs GET /api/v1/organizations
func (h *OrgHandlers) ListMyOrgs(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	orgs, err := h.Orgs.ListMyOrgs(r.Context(), u)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	out := make([]orgResp, 0, len(orgs))
	for i := range orgs {
		out = append(out, toOrgResp(&orgs[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"organizations": out})
}

// GetOrg GET /api/v1/organizations/{id}
func (h *OrgHandlers) GetOrg(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	org, err := h.Orgs.GetOrg(r.Context(), u, id)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toOrgResp(org))
}

// ListMembers GET /api/v1/organizations/{id}/members
func (h *OrgHandlers) ListMembers(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	members, err := h.Orgs.ListMembers(r.Context(), u, id)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	out := make([]orgMemberResp, 0, len(members))
	for _, m := range members {
		out = append(out, orgMemberResp{OrgID: m.OrgID, UserID: m.UserID, Role: string(m.Role), JoinedAt: m.JoinedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": out})
}

// AddMember POST /api/v1/organizations/{id}/members
// Body: {"user_id": "...", "role": "ADMIN|MEMBER"}
func (h *OrgHandlers) AddMember(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var body struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id required")
		return
	}
	if err := h.Orgs.AddMember(r.Context(), u, id, body.UserID, domain.OrgRole(body.Role)); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateMemberRole PATCH /api/v1/organizations/{id}/members/{userId}/role
// Body: {"role": "ADMIN|MEMBER"}
func (h *OrgHandlers) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	orgID := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")
	r.Body = http.MaxBytesReader(w, r.Body, 512)
	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.Orgs.UpdateMemberRole(r.Context(), u, orgID, userID, domain.OrgRole(body.Role)); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveMember DELETE /api/v1/organizations/{id}/members/{userId}
func (h *OrgHandlers) RemoveMember(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	orgID := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")
	if err := h.Orgs.RemoveMember(r.Context(), u, orgID, userID); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListOrgLakes GET /api/v1/organizations/{id}/lakes  P13-A：列出组织下的所有湖。
// 调用者需是该组织成员。
func (h *OrgHandlers) ListOrgLakes(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	orgID := chi.URLParam(r, "id")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "org id required")
		return
	}
	lakes, err := h.Lakes.ListLakesByOrg(r.Context(), u, orgID, h.Orgs)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	type lakeItem struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		IsPublic    bool   `json:"is_public"`
		OwnerID     string `json:"owner_id"`
		OrgID       string `json:"org_id,omitempty"`
	}
	out := make([]lakeItem, 0, len(lakes))
	for _, l := range lakes {
		out = append(out, lakeItem{
			ID: l.ID, Name: l.Name, Description: l.Description,
			IsPublic: l.IsPublic, OwnerID: l.OwnerID, OrgID: l.OrgID,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"lakes": out})
}
