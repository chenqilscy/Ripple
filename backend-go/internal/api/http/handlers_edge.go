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

// EdgeHandlers 边 HTTP 处理器。
type EdgeHandlers struct {
	Edges *service.EdgeService
}

type createEdgeReq struct {
	SrcNodeID string `json:"src_node_id"`
	DstNodeID string `json:"dst_node_id"`
	Kind      string `json:"kind"`
	Label     string `json:"label,omitempty"`
}

type edgeResp struct {
	ID        string     `json:"id"`
	LakeID    string     `json:"lake_id"`
	SrcNodeID string     `json:"src_node_id"`
	DstNodeID string     `json:"dst_node_id"`
	Kind      string     `json:"kind"`
	Label     string     `json:"label,omitempty"`
	OwnerID   string     `json:"owner_id"`
	Strength  float64    `json:"strength,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

func toEdgeResp(e *domain.Edge) edgeResp {
	return edgeResp{
		ID:        e.ID,
		LakeID:    e.LakeID,
		SrcNodeID: e.SrcNodeID,
		DstNodeID: e.DstNodeID,
		Kind:      string(e.Kind),
		Label:     e.Label,
		OwnerID:   e.OwnerID,
		Strength:  e.Strength,
		CreatedAt: e.CreatedAt,
		DeletedAt: e.DeletedAt,
	}
}

// Create POST /api/v1/edges
func (h *EdgeHandlers) Create(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	var in createEdgeReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	e, err := h.Edges.Create(r.Context(), u, service.CreateEdgeInput{
		SrcNodeID: in.SrcNodeID,
		DstNodeID: in.DstNodeID,
		Kind:      domain.EdgeKind(in.Kind),
		Label:     in.Label,
	})
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toEdgeResp(e))
}

// ListByLake GET /api/v1/lakes/{id}/edges?include_deleted=false
func (h *EdgeHandlers) ListByLake(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	lakeID := chi.URLParam(r, "id")
	includeDeleted, _ := strconv.ParseBool(r.URL.Query().Get("include_deleted"))
	edges, err := h.Edges.ListByLake(r.Context(), u, lakeID, includeDeleted)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	out := make([]edgeResp, 0, len(edges))
	for i := range edges {
		out = append(out, toEdgeResp(&edges[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"edges": out})
}

// Delete DELETE /api/v1/edges/{id}
func (h *EdgeHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.Edges.Delete(r.Context(), u, id); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
