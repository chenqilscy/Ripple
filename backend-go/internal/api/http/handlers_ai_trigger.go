package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxAITriggerInputNodes = 20
const aiTriggerEstimatedSeconds = 15
const safeAIJobErrorText = "ai job failed; please retry later"

type aiNodeGetter interface {
	Get(ctx context.Context, actor *domain.User, nodeID string) (*domain.Node, error)
}

// AiTriggerHandlers Phase 15-C：AI 节点填充触发器。
type AiTriggerHandlers struct {
	Jobs            store.AiJobRepository
	Nodes           aiNodeGetter
	Memberships     store.MembershipRepository
	PromptTemplates store.PromptTemplateRepository
	Orgs            *service.OrgService
}

type aiTriggerReq struct {
	PromptTemplateID string            `json:"prompt_template_id"`
	InputNodeIDs     []string          `json:"input_node_ids"`
	OverrideVars     map[string]string `json:"override_vars"`
}

type aiTriggerResp struct {
	JobID            string `json:"job_id"`
	AIJobID          string `json:"ai_job_id"`
	NodeID           string `json:"node_id"`
	Status           string `json:"status"`
	ProgressPct      int    `json:"progress_pct"`
	EstimatedSeconds int    `json:"estimated_seconds"`
}

// Trigger POST /api/v1/lakes/{lake_id}/nodes/{node_id}/ai_trigger
func (h *AiTriggerHandlers) Trigger(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	lakeID := chi.URLParam(r, "lake_id")
	nodeID := chi.URLParam(r, "node_id")
	if h.Jobs == nil || h.Nodes == nil {
		writeError(w, http.StatusInternalServerError, "ai trigger unavailable")
		return
	}

	// 权限校验：至少需要 NAVIGATOR（编辑）权限
	if h.Memberships != nil {
		role, err := h.Memberships.GetRole(r.Context(), u.ID, lakeID)
		if err != nil || (role != domain.RoleNavigator && role != domain.RoleOwner) {
			writeError(w, http.StatusForbidden, "requires navigator or owner role")
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var in aiTriggerReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	in.PromptTemplateID = strings.TrimSpace(in.PromptTemplateID)
	inputNodeIDs, err := normalizeAITriggerInputNodeIDs(in.InputNodeIDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	in.InputNodeIDs = inputNodeIDs
	in.OverrideVars = normalizeAITriggerOverrideVars(in.OverrideVars)

	targetNode, err := h.Nodes.Get(r.Context(), u, nodeID)
	if err != nil || targetNode == nil {
		writeError(w, http.StatusNotFound, "node not found or access denied")
		return
	}
	if targetNode.LakeID != lakeID {
		writeError(w, http.StatusBadRequest, "node does not belong to this lake")
		return
	}
	for _, inputNodeID := range in.InputNodeIDs {
		inputNode, err := h.Nodes.Get(r.Context(), u, inputNodeID)
		if err != nil || inputNode == nil {
			writeError(w, http.StatusBadRequest, "input node not found or access denied")
			return
		}
		if inputNode.LakeID != lakeID {
			writeError(w, http.StatusBadRequest, "input node does not belong to this lake")
			return
		}
	}

	if in.PromptTemplateID != "" {
		if h.PromptTemplates == nil {
			writeError(w, http.StatusInternalServerError, "prompt template unavailable")
			return
		}
		tpl, err := h.PromptTemplates.GetByID(r.Context(), in.PromptTemplateID)
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
	}

	job := domain.AiJob{
		ID:               uuid.New().String(),
		NodeID:           nodeID,
		LakeID:           lakeID,
		PromptTemplateID: in.PromptTemplateID,
		Status:           domain.AiJobPending,
		InputNodeIDs:     in.InputNodeIDs,
		OverrideVars:     in.OverrideVars,
		CreatedBy:        u.ID,
		CreatedAt:        time.Now().UTC(),
	}

	created, ok2, err := h.Jobs.CreateWithConflictCheck(r.Context(), job)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create ai job")
		return
	}
	if !ok2 {
		// 冲突：该节点已有 pending/processing 任务
		writeError(w, http.StatusConflict, "node already has an active ai job")
		return
	}
	if created == nil {
		writeError(w, http.StatusInternalServerError, "failed to create ai job")
		return
	}

	w.Header().Set("Location", "/api/v1/lakes/"+lakeID+"/nodes/"+nodeID+"/ai_status")
	writeJSON(w, http.StatusAccepted, aiTriggerResp{
		JobID:            created.ID,
		AIJobID:          created.ID,
		NodeID:           created.NodeID,
		Status:           string(created.Status),
		ProgressPct:      created.ProgressPct,
		EstimatedSeconds: aiTriggerEstimatedSeconds,
	})
}

func normalizeAITriggerInputNodeIDs(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, id := range raw {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		if len(out) == maxAITriggerInputNodes {
			return nil, errors.New("input_node_ids too many (max 20)")
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func normalizeAITriggerOverrideVars(vars map[string]string) map[string]string {
	if len(vars) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(vars))
	for key, value := range vars {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

type aiStatusResp struct {
	AIJobID     string     `json:"ai_job_id"`
	JobID       string     `json:"job_id"`
	NodeID      string     `json:"node_id"`
	Status      string     `json:"status"`
	ProgressPct int        `json:"progress_pct"`
	Error       string     `json:"error,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

// Status GET /api/v1/lakes/{lake_id}/nodes/{node_id}/ai_status
func (h *AiTriggerHandlers) Status(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	lakeID := chi.URLParam(r, "lake_id")
	nodeID := chi.URLParam(r, "node_id")
	if h.Jobs == nil {
		writeError(w, http.StatusInternalServerError, "ai trigger unavailable")
		return
	}

	// 权限校验：OBSERVER+ 可查状态
	if h.Memberships != nil {
		_, err := h.Memberships.GetRole(r.Context(), u.ID, lakeID)
		if err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
	}

	job, err := h.Jobs.GetByNodeID(r.Context(), nodeID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no ai job for this node")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get ai job status")
		return
	}
	if job == nil {
		writeError(w, http.StatusNotFound, "no ai job for this node")
		return
	}
	if job.LakeID != lakeID {
		writeError(w, http.StatusNotFound, "no ai job for this node")
		return
	}

	writeJSON(w, http.StatusOK, aiStatusResp{
		AIJobID:     job.ID,
		JobID:       job.ID,
		NodeID:      job.NodeID,
		Status:      string(job.Status),
		ProgressPct: job.ProgressPct,
		Error:       sanitizeAIJobError(job.Error),
		StartedAt:   job.StartedAt,
		FinishedAt:  job.FinishedAt,
	})
}

func sanitizeAIJobError(errMsg string) string {
	if strings.TrimSpace(errMsg) == "" {
		return ""
	}
	return safeAIJobErrorText
}
