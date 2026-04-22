package httpapi

import (
	"encoding/json"
	"net/http"

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
