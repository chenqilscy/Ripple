package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

// NodeTemplateHandlers P18-C：节点模板库。
type NodeTemplateHandlers struct {
	Repo  store.NodeTemplateRepository
	Nodes *service.NodeService
}

type templateResp struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags"`
	IsSystem    bool      `json:"is_system"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

func toTemplateResp(t domain.NodeTemplate) templateResp {
	tags := t.Tags
	if tags == nil {
		tags = []string{}
	}
	return templateResp{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Content:     t.Content,
		Tags:        tags,
		IsSystem:    t.IsSystem,
		CreatedBy:   t.CreatedBy,
		CreatedAt:   t.CreatedAt,
	}
}

// ListTemplates GET /api/v1/templates
func (h *NodeTemplateHandlers) ListTemplates(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	templates, err := h.Repo.List(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]templateResp, len(templates))
	for i, t := range templates {
		resp[i] = toTemplateResp(t)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"templates": resp})
}

// CreateTemplate POST /api/v1/templates
func (h *NodeTemplateHandlers) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // 64KB 上限
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Content     string   `json:"content"`
		Tags        []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content required")
		return
	}
	if len(req.Name) > 100 {
		writeError(w, http.StatusBadRequest, "name too long (max 100)")
		return
	}
	if len([]rune(req.Content)) > 10000 {
		writeError(w, http.StatusBadRequest, "content too long (max 10000 chars)")
		return
	}
	if len([]rune(req.Description)) > 500 {
		writeError(w, http.StatusBadRequest, "description too long (max 500 chars)")
		return
	}
	if len(req.Tags) > 20 {
		writeError(w, http.StatusBadRequest, "too many tags (max 20)")
		return
	}
	for _, tag := range req.Tags {
		if len(tag) > 50 {
			writeError(w, http.StatusBadRequest, "tag too long (max 50 chars each)")
			return
		}
	}

	now := time.Now().UTC()
	t := &domain.NodeTemplate{
		ID:          platform.NewID(),
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		Tags:        req.Tags,
		CreatedBy:   u.ID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.Repo.Create(r.Context(), t); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toTemplateResp(*t))
}

// DeleteTemplate DELETE /api/v1/templates/{id}
func (h *NodeTemplateHandlers) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	tplID := chi.URLParam(r, "id")
	if err := h.Repo.Delete(r.Context(), tplID, u.ID); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateNodeFromTemplate POST /api/v1/lakes/{id}/nodes/from_template
func (h *NodeTemplateHandlers) CreateNodeFromTemplate(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	lakeID := chi.URLParam(r, "id")

	var req struct {
		TemplateID string `json:"template_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.TemplateID == "" {
		writeError(w, http.StatusBadRequest, "template_id required")
		return
	}

	tpl, err := h.Repo.Get(r.Context(), req.TemplateID, u.ID)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}

	node, err := h.Nodes.Create(r.Context(), u, service.CreateNodeInput{
		LakeID:  lakeID,
		Content: tpl.Content,
		Type:    domain.NodeTypeText,
	})
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, toNodeResp(node))
}
