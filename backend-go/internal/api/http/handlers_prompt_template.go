package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PromptTemplateHandlers Phase 15-C：Prompt 模板 CRUD。
type PromptTemplateHandlers struct {
	Repo store.PromptTemplateRepository
	Orgs *service.OrgService
}

type createPromptTplReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Template    string `json:"template"`
	Scope       string `json:"scope"` // "private" | "org"
	OrgID       string `json:"org_id,omitempty"`
}

type promptTplResp struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Template    string    `json:"template"`
	Scope       string    `json:"scope"`
	OrgID       string    `json:"org_id,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toPromptTplResp(t *domain.PromptTemplate) promptTplResp {
	return promptTplResp{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Template:    t.Template,
		Scope:       string(t.Scope),
		OrgID:       t.OrgID,
		CreatedBy:   t.CreatedBy,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

// Create POST /api/v1/prompt_templates
func (h *PromptTemplateHandlers) Create(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var in createPromptTplReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.Template = strings.TrimSpace(in.Template)
	in.Scope = strings.TrimSpace(in.Scope)
	in.OrgID = strings.TrimSpace(in.OrgID)
	if in.Name == "" || in.Template == "" {
		writeError(w, http.StatusBadRequest, "name and template are required")
		return
	}
	scope := domain.PromptTemplateScope(in.Scope)
	if scope != domain.PromptScopePrivate && scope != domain.PromptScopeOrg {
		scope = domain.PromptScopePrivate
	}
	if scope == domain.PromptScopeOrg {
		if in.OrgID == "" {
			writeError(w, http.StatusBadRequest, "org_id is required for org scope")
			return
		}
		if h.Orgs == nil {
			writeError(w, http.StatusBadRequest, "organization sharing unavailable")
			return
		}
		isMember, err := h.Orgs.IsMember(r.Context(), u.ID, in.OrgID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to validate org membership")
			return
		}
		if !isMember {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
	} else {
		in.OrgID = ""
	}

	now := time.Now().UTC()
	tpl := domain.PromptTemplate{
		ID:          uuid.New().String(),
		Name:        in.Name,
		Description: in.Description,
		Template:    in.Template,
		Scope:       scope,
		OrgID:       in.OrgID,
		CreatedBy:   u.ID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	created, err := h.Repo.Create(r.Context(), tpl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create prompt template")
		return
	}
	writeJSON(w, http.StatusCreated, toPromptTplResp(created))
}

// List GET /api/v1/prompt_templates?limit=20&offset=0
func (h *PromptTemplateHandlers) List(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	orgIDs, err := visiblePromptTemplateOrgIDs(r.Context(), h.Orgs, u)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve visible organizations")
		return
	}

	tpls, total, err := h.Repo.ListVisible(r.Context(), u.ID, orgIDs, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list prompt templates")
		return
	}

	items := make([]promptTplResp, len(tpls))
	for i := range tpls {
		items[i] = toPromptTplResp(&tpls[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
	})
}

// Get GET /api/v1/prompt_templates/{id}
func (h *PromptTemplateHandlers) Get(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	tpl, err := h.Repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "prompt template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get prompt template")
		return
	}
	allowed, err := canAccessPromptTemplate(r.Context(), h.Orgs, u.ID, tpl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate prompt template access")
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	writeJSON(w, http.StatusOK, toPromptTplResp(tpl))
}

type patchPromptTplReq struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Template    *string `json:"template"`
}

// Update PATCH /api/v1/prompt_templates/{id}
func (h *PromptTemplateHandlers) Update(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")

	// 先取现有记录校验归属
	tpl, err := h.Repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "prompt template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get prompt template")
		return
	}
	allowed, err := canManagePromptTemplate(r.Context(), h.Orgs, u.ID, tpl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate prompt template access")
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "only the creator or organization admin can update this template")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var in patchPromptTplReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Name != nil {
		v := strings.TrimSpace(*in.Name)
		in.Name = &v
	}
	if in.Description != nil {
		v := strings.TrimSpace(*in.Description)
		in.Description = &v
	}
	if in.Template != nil {
		v := strings.TrimSpace(*in.Template)
		in.Template = &v
	}

	upd := domain.PromptTemplateUpdate{
		Name:        in.Name,
		Description: in.Description,
		Template:    in.Template,
	}
	// 防止将关键字段更新为空字符串
	if upd.Name != nil && *upd.Name == "" {
		writeError(w, http.StatusBadRequest, "name cannot be empty")
		return
	}
	if upd.Template != nil && *upd.Template == "" {
		writeError(w, http.StatusBadRequest, "template cannot be empty")
		return
	}
	if err := h.Repo.Update(r.Context(), id, upd); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update prompt template")
		return
	}
	// 回读更新后的记录
	updated, err := h.Repo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get updated template")
		return
	}
	writeJSON(w, http.StatusOK, toPromptTplResp(updated))
}

// Delete DELETE /api/v1/prompt_templates/{id}
func (h *PromptTemplateHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")

	tpl, err := h.Repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "prompt template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get prompt template")
		return
	}
	allowed, err := canManagePromptTemplate(r.Context(), h.Orgs, u.ID, tpl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate prompt template access")
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "only the creator or organization admin can delete this template")
		return
	}

	if err := h.Repo.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete prompt template")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
