package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// LakeHandlers 湖泊 HTTP 处理器。
type LakeHandlers struct {
	Lakes  *service.LakeService
	Spaces *service.SpaceService
}

type createLakeReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
	SpaceID     string `json:"space_id,omitempty"`
}

type lakeResp struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
	OwnerID     string `json:"owner_id"`
	SpaceID     string `json:"space_id,omitempty"`
	Role        string `json:"role,omitempty"`
}

// Create POST /api/v1/lakes
func (h *LakeHandlers) Create(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	var in createLakeReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	// 若指定了 space_id，必须验证调用者是该 space 成员（OWNER/EDITOR/VIEWER 均可加湖）。
	if in.SpaceID != "" && h.Spaces != nil {
		if _, _, err := h.Spaces.Get(r.Context(), u, in.SpaceID); err != nil {
			writeError(w, mapDomainError(err), err.Error())
			return
		}
	}
	l, err := h.Lakes.Create(r.Context(), u, service.CreateLakeInput{
		Name: in.Name, Description: in.Description, IsPublic: in.IsPublic, SpaceID: in.SpaceID,
	})
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, lakeResp{
		ID: l.ID, Name: l.Name, Description: l.Description,
		IsPublic: l.IsPublic, OwnerID: l.OwnerID, SpaceID: l.SpaceID, Role: "OWNER",
	})
}

// Get GET /api/v1/lakes/{id}
func (h *LakeHandlers) Get(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	l, role, err := h.Lakes.Get(r.Context(), u, id)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, lakeResp{
		ID: l.ID, Name: l.Name, Description: l.Description,
		IsPublic: l.IsPublic, OwnerID: l.OwnerID, SpaceID: l.SpaceID, Role: string(role),
	})
}

// ListMine GET /api/v1/lakes  （可选 ?space_id=xxx 过滤）
func (h *LakeHandlers) ListMine(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	spaceID := r.URL.Query().Get("space_id")
	items, err := h.Lakes.ListMineBySpace(r.Context(), u, spaceID)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	out := make([]lakeResp, 0, len(items))
	ids := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, lakeResp{
			ID: it.Lake.ID, Name: it.Lake.Name, Description: it.Lake.Description,
			IsPublic: it.Lake.IsPublic, OwnerID: it.Lake.OwnerID, SpaceID: it.Lake.SpaceID, Role: string(it.Role),
		})
		ids = append(ids, it.Lake.ID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"lakes": out, "lake_ids": ids})
}

// Move PATCH /api/v1/lakes/{id}/space  body: {"space_id": "" | "<uuid>"}
// 仅 Owner；目标 space 非空时 actor 必须是该 space 的成员。
func (h *LakeHandlers) Move(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "lake id required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var in struct {
		SpaceID string `json:"space_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	l, err := h.Lakes.MoveToSpace(r.Context(), u, id, in.SpaceID, h.Spaces)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, lakeResp{
		ID: l.ID, Name: l.Name, Description: l.Description,
		IsPublic: l.IsPublic, OwnerID: l.OwnerID, SpaceID: l.SpaceID,
	})
}

// UpdateMemberRole PUT /api/v1/lakes/{id}/members/{userID}/role
// P10-C：变更湖成员角色（仅 OWNER 可操作；不能升级为 OWNER；不能修改自己）。
func (h *LakeHandlers) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	lakeID := chi.URLParam(r, "id")
	targetUserID := chi.URLParam(r, "userID")
	if lakeID == "" || targetUserID == "" {
		writeError(w, http.StatusBadRequest, "lake id and user id required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 512)
	var in struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Role == "" {
		writeError(w, http.StatusBadRequest, "role is required")
		return
	}
	if err := h.Lakes.UpdateMemberRole(r.Context(), actor, lakeID, targetUserID, domain.Role(in.Role)); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListMembers GET /api/v1/lakes/{id}/members
// P11-C：返回湖成员列表，调用方须至少是 OBSERVER。
func (h *LakeHandlers) ListMembers(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	lakeID := chi.URLParam(r, "id")
	if lakeID == "" {
		writeError(w, http.StatusBadRequest, "lake id required")
		return
	}
	members, err := h.Lakes.ListMembers(r.Context(), actor, lakeID)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	type memberView struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	out := make([]memberView, 0, len(members))
	for _, m := range members {
		out = append(out, memberView{UserID: m.UserID, Role: string(m.Role)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": out})
}