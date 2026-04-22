package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// TagHandlers P13-C：节点标签 HTTP 处理器。
type TagHandlers struct {
	Svc *service.TagService
}

// ListLakeTags GET /api/v1/lakes/{id}/tags
func (h *TagHandlers) ListLakeTags(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	lakeID := chi.URLParam(r, "id")
	tags, err := h.Svc.ListLakeTags(r.Context(), actor, lakeID)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": tags})
}

// GetNodeTags GET /api/v1/nodes/{id}/tags
func (h *TagHandlers) GetNodeTags(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	nodeID := chi.URLParam(r, "id")
	tags, err := h.Svc.GetNodeTags(r.Context(), actor, nodeID)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": tags})
}

type setTagsReq struct {
	Tags []string `json:"tags"`
}

// SetNodeTags PUT /api/v1/nodes/{id}/tags
func (h *TagHandlers) SetNodeTags(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	nodeID := chi.URLParam(r, "id")
	var in setTagsReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	if err := h.Svc.SetNodeTags(r.Context(), actor, nodeID, in.Tags); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": in.Tags})
}

// ListNodesByTag GET /api/v1/lakes/{id}/nodes/by_tag?tag=xxx
func (h *TagHandlers) ListNodesByTag(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	lakeID := chi.URLParam(r, "id")
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		writeError(w, http.StatusBadRequest, "tag param required")
		return
	}
	nodeIDs, err := h.Svc.ListNodesByTag(r.Context(), actor, lakeID, tag)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"node_ids": nodeIDs})
}
