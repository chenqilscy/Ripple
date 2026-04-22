package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// SpaceHandlers Space HTTP 处理器（M3-S1）。
type SpaceHandlers struct {
	Spaces *service.SpaceService
}

type spaceResp struct {
	ID                  string `json:"id"`
	OwnerID             string `json:"owner_id"`
	Name                string `json:"name"`
	Description         string `json:"description"`
	LLMQuotaMonthly     int    `json:"llm_quota_monthly"`
	LLMUsedCurrentMonth int    `json:"llm_used_current_month"`
	Role                string `json:"role,omitempty"`
}

func toSpaceResp(s *domain.Space, role domain.SpaceRole) spaceResp {
	return spaceResp{
		ID: s.ID, OwnerID: s.OwnerID, Name: s.Name, Description: s.Description,
		LLMQuotaMonthly: s.LLMQuotaMonthly, LLMUsedCurrentMonth: s.LLMUsedCurrentMonth,
		Role: string(role),
	}
}

type createSpaceReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Create POST /api/v1/spaces
func (h *SpaceHandlers) Create(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	var in createSpaceReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	sp, err := h.Spaces.Create(r.Context(), u, service.CreateSpaceInput{
		Name: in.Name, Description: in.Description,
	})
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toSpaceResp(sp, domain.SpaceRoleOwner))
}

// Get GET /api/v1/spaces/{id}
func (h *SpaceHandlers) Get(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	sp, role, err := h.Spaces.Get(r.Context(), u, id)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toSpaceResp(sp, role))
}

// ListMine GET /api/v1/spaces
func (h *SpaceHandlers) ListMine(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	items, err := h.Spaces.ListMine(r.Context(), u)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	out := make([]spaceResp, 0, len(items))
	for i := range items {
		out = append(out, toSpaceResp(&items[i], ""))
	}
	writeJSON(w, http.StatusOK, map[string]any{"spaces": out})
}

type updateSpaceReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Update PATCH /api/v1/spaces/{id}
func (h *SpaceHandlers) Update(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	var in updateSpaceReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.Spaces.UpdateMeta(r.Context(), u, id, service.UpdateMetaInput{
		Name: in.Name, Description: in.Description,
	}); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Delete DELETE /api/v1/spaces/{id}
func (h *SpaceHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.Spaces.Delete(r.Context(), u, id); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type addMemberReq struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// AddMember POST /api/v1/spaces/{id}/members
func (h *SpaceHandlers) AddMember(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	var in addMemberReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.Spaces.AddMember(r.Context(), u, id, in.UserID, domain.SpaceRole(in.Role)); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveMember DELETE /api/v1/spaces/{id}/members/{userID}
func (h *SpaceHandlers) RemoveMember(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	target := chi.URLParam(r, "userID")
	if err := h.Spaces.RemoveMember(r.Context(), u, id, target); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListMembers GET /api/v1/spaces/{id}/members
func (h *SpaceHandlers) ListMembers(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	members, err := h.Spaces.ListMembers(r.Context(), u, id)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	type mResp struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	out := make([]mResp, 0, len(members))
	for _, m := range members {
		out = append(out, mResp{UserID: m.UserID, Role: string(m.Role)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": out})
}
