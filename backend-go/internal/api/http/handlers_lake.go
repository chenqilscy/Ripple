package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// LakeHandlers 湖泊 HTTP 处理器。
type LakeHandlers struct {
	Lakes  *service.LakeService
	Spaces *service.SpaceService
	Orgs   *service.OrgService // P13-A：组织归属校验
	Nodes  *service.NodeService // P13-D：内容导出
	Edges  *service.EdgeService // P13-D：内容导出（边，可选）
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
	OrgID       string `json:"org_id,omitempty"`
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
		IsPublic: l.IsPublic, OwnerID: l.OwnerID, SpaceID: l.SpaceID, OrgID: l.OrgID, Role: "OWNER",
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
		IsPublic: l.IsPublic, OwnerID: l.OwnerID, SpaceID: l.SpaceID, OrgID: l.OrgID, Role: string(role),
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
			IsPublic: it.Lake.IsPublic, OwnerID: it.Lake.OwnerID, SpaceID: it.Lake.SpaceID, OrgID: it.Lake.OrgID, Role: string(it.Role),
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
		IsPublic: l.IsPublic, OwnerID: l.OwnerID, SpaceID: l.SpaceID, OrgID: l.OrgID,
	})
}

// SetLakeOrg PATCH /api/v1/lakes/{id}/org  P13-A：设置/清除湖的组织归属。
// body: {"org_id": "" | "<uuid>"}
func (h *LakeHandlers) SetLakeOrg(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "lake id required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var in struct {
		OrgID string `json:"org_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	l, err := h.Lakes.SetLakeOrg(r.Context(), u, id, in.OrgID, h.Orgs)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, lakeResp{
		ID: l.ID, Name: l.Name, Description: l.Description,
		IsPublic: l.IsPublic, OwnerID: l.OwnerID, SpaceID: l.SpaceID, OrgID: l.OrgID,
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

// Export GET /api/v1/lakes/{id}/export?format=json|markdown  P13-D：内容导出。
// 调用方须至少是 OBSERVER；最多导出 10000 节点。
const exportMaxNodes = 10_000

func (h *LakeHandlers) Export(w http.ResponseWriter, r *http.Request) {
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
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "markdown" {
		writeError(w, http.StatusBadRequest, "format must be json or markdown")
		return
	}

	l, _, err := h.Lakes.Get(r.Context(), actor, lakeID)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}

	nodes, err := h.Nodes.ListByLake(r.Context(), actor, lakeID, false)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	if len(nodes) > exportMaxNodes {
		writeError(w, http.StatusInsufficientStorage,
			fmt.Sprintf("lake has too many nodes (%d), export limit is %d", len(nodes), exportMaxNodes))
		return
	}

	var edges []domain.Edge
	if h.Edges != nil {
		edges, err = h.Edges.ListByLake(r.Context(), actor, lakeID, false)
		if err != nil {
			writeError(w, mapDomainError(err), err.Error())
			return
		}
	} else {
		edges = []domain.Edge{}
	}

	switch format {
	case "json":
		type nodeExport struct {
			ID        string  `json:"id"`
			Content   string  `json:"content"`
			Type      string  `json:"type"`
			State     string  `json:"state"`
			OwnerID   string  `json:"owner_id"`
			CreatedAt string  `json:"created_at"`
			UpdatedAt string  `json:"updated_at"`
		}
		type edgeExport struct {
			ID        string `json:"id"`
			SrcNodeID string `json:"src_node_id"`
			DstNodeID string `json:"dst_node_id"`
			Kind      string `json:"kind"`
			Label     string `json:"label,omitempty"`
		}
		type lakeExport struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			IsPublic    bool   `json:"is_public"`
			OwnerID     string `json:"owner_id"`
		}
		nodeOut := make([]nodeExport, 0, len(nodes))
		for _, n := range nodes {
			nodeOut = append(nodeOut, nodeExport{
				ID: n.ID, Content: n.Content, Type: string(n.Type),
				State: string(n.State), OwnerID: n.OwnerID,
				CreatedAt: n.CreatedAt.Format("2006-01-02T15:04:05Z"),
				UpdatedAt: n.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			})
		}
		edgeOut := make([]edgeExport, 0, len(edges))
		for _, e := range edges {
			edgeOut = append(edgeOut, edgeExport{
				ID: e.ID, SrcNodeID: e.SrcNodeID, DstNodeID: e.DstNodeID,
				Kind: string(e.Kind), Label: e.Label,
			})
		}
		payload := map[string]any{
			"lake":  lakeExport{ID: l.ID, Name: l.Name, Description: l.Description, IsPublic: l.IsPublic, OwnerID: l.OwnerID},
			"nodes": nodeOut,
			"edges": edgeOut,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "export failed")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="lake-%s.json"`, lakeID))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)

	case "markdown":
		var sb strings.Builder
		sb.WriteString("# ")
		sb.WriteString(l.Name)
		sb.WriteString("\n\n")
		if l.Description != "" {
			sb.WriteString("> ")
			sb.WriteString(l.Description)
			sb.WriteString("\n\n")
		}
		for _, n := range nodes {
			if n.State == domain.StateVapor || n.State == domain.StateErased {
				continue
			}
			sb.WriteString("## ")
			sb.WriteString(fmt.Sprintf("[%s · %s]", string(n.Type), string(n.State)))
			sb.WriteString("\n\n")
			sb.WriteString(n.Content)
			sb.WriteString("\n\n---\n\n")
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="lake-%s.md"`, lakeID))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sb.String()))
	}
}