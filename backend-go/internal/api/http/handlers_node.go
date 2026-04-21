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

// NodeHandlers 节点 HTTP 处理器。
type NodeHandlers struct {
	Nodes *service.NodeService
}

type createNodeReq struct {
	LakeID   string           `json:"lake_id"`
	Content  string           `json:"content"`
	Type     string           `json:"type"`
	Position *positionPayload `json:"position,omitempty"`
}

type positionPayload struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type nodeResp struct {
	ID        string           `json:"id"`
	LakeID    string           `json:"lake_id"`
	OwnerID   string           `json:"owner_id"`
	Content   string           `json:"content"`
	Type      string           `json:"type"`
	State     string           `json:"state"`
	Position  *positionPayload `json:"position,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	DeletedAt *time.Time       `json:"deleted_at,omitempty"`
	TTLAt     *time.Time       `json:"ttl_at,omitempty"`
}

func toNodeResp(n *domain.Node) nodeResp {
	resp := nodeResp{
		ID: n.ID, LakeID: n.LakeID, OwnerID: n.OwnerID,
		Content: n.Content, Type: string(n.Type), State: string(n.State),
		CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt,
		DeletedAt: n.DeletedAt, TTLAt: n.TTLAt,
	}
	if n.Position != nil {
		resp.Position = &positionPayload{X: n.Position.X, Y: n.Position.Y, Z: n.Position.Z}
	}
	return resp
}

// Create POST /api/v1/nodes
func (h *NodeHandlers) Create(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	var in createNodeReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	input := service.CreateNodeInput{
		LakeID:  in.LakeID,
		Content: in.Content,
		Type:    domain.NodeType(in.Type),
	}
	if in.Position != nil {
		input.Position = &domain.Position{X: in.Position.X, Y: in.Position.Y, Z: in.Position.Z}
	}
	n, err := h.Nodes.Create(r.Context(), u, input)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toNodeResp(n))
}

// Get GET /api/v1/nodes/{id}
func (h *NodeHandlers) Get(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	n, err := h.Nodes.Get(r.Context(), u, id)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toNodeResp(n))
}

// ListByLake GET /api/v1/lakes/{id}/nodes?include_vapor=false
func (h *NodeHandlers) ListByLake(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	lakeID := chi.URLParam(r, "id")
	includeVapor, _ := strconv.ParseBool(r.URL.Query().Get("include_vapor"))
	nodes, err := h.Nodes.ListByLake(r.Context(), u, lakeID, includeVapor)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	out := make([]nodeResp, 0, len(nodes))
	for i := range nodes {
		out = append(out, toNodeResp(&nodes[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": out})
}

// Evaporate POST /api/v1/nodes/{id}/evaporate
func (h *NodeHandlers) Evaporate(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	n, err := h.Nodes.Evaporate(r.Context(), u, id)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toNodeResp(n))
}

// Restore POST /api/v1/nodes/{id}/restore
func (h *NodeHandlers) Restore(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	n, err := h.Nodes.Restore(r.Context(), u, id)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toNodeResp(n))
}

// Condense POST /api/v1/nodes/{id}/condense
// Body 可选 {"lake_id":"..."}：未传则沿用节点当前 LakeID（造云时已写）。
func (h *NodeHandlers) Condense(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	var body struct {
		LakeID string `json:"lake_id"`
	}
	// Body 可选；解析失败也允许（沿用节点 LakeID）
	_ = json.NewDecoder(r.Body).Decode(&body)
	n, err := h.Nodes.Condense(r.Context(), u, id, body.LakeID)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toNodeResp(n))
}
