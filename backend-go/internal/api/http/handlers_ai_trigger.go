package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AiTriggerHandlers Phase 15-C：AI 节点填充触发器。
type AiTriggerHandlers struct {
	Jobs        store.AiJobRepository
	Memberships store.MembershipRepository
}

type aiTriggerReq struct {
	PromptTemplateID string            `json:"prompt_template_id"`
	InputNodeIDs     []string          `json:"input_node_ids"`
	OverrideVars     map[string]string `json:"override_vars"`
}

type aiTriggerResp struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
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

	w.Header().Set("Location", "/api/v1/lakes/"+lakeID+"/nodes/"+nodeID+"/ai_status")
	writeJSON(w, http.StatusAccepted, aiTriggerResp{
		JobID:  created.ID,
		Status: string(created.Status),
	})
}

type aiStatusResp struct {
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

	writeJSON(w, http.StatusOK, aiStatusResp{
		JobID:       job.ID,
		NodeID:      job.NodeID,
		Status:      string(job.Status),
		ProgressPct: job.ProgressPct,
		Error:       job.Error,
		StartedAt:   job.StartedAt,
		FinishedAt:  job.FinishedAt,
	})
}
