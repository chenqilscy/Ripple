package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// CloudHandlers 造云 HTTP 处理器。
type CloudHandlers struct {
	Clouds *service.CloudService
}

type createCloudReq struct {
	LakeID string `json:"lake_id"` // optional
	Prompt string `json:"prompt"`
	N      int    `json:"n"`
	Type   string `json:"type"`
}

type cloudTaskResp struct {
	ID            string     `json:"id"`
	OwnerID       string     `json:"owner_id"`
	LakeID        string     `json:"lake_id,omitempty"`
	Prompt        string     `json:"prompt"`
	N             int        `json:"n"`
	NodeType      string     `json:"node_type"`
	Status        string     `json:"status"`
	RetryCount    int        `json:"retry_count"`
	LastError     string     `json:"last_error,omitempty"`
	ResultNodeIDs []string   `json:"result_node_ids"`
	CreatedAt     time.Time  `json:"created_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

func toCloudResp(t *domain.CloudTask) cloudTaskResp {
	return cloudTaskResp{
		ID: t.ID, OwnerID: t.OwnerID, LakeID: t.LakeID, Prompt: t.Prompt,
		N: t.N, NodeType: string(t.NodeType), Status: string(t.Status),
		RetryCount: t.RetryCount, LastError: t.LastError,
		ResultNodeIDs: t.ResultNodeIDs,
		CreatedAt:     t.CreatedAt, StartedAt: t.StartedAt, CompletedAt: t.CompletedAt,
	}
}

// Create POST /api/v1/clouds
func (h *CloudHandlers) Create(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	var in createCloudReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	t, err := h.Clouds.Generate(r.Context(), u, service.CreateCloudInput{
		LakeID: in.LakeID, Prompt: in.Prompt, N: in.N,
		NodeType: domain.NodeType(in.Type),
	})
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toCloudResp(t))
}

// Get GET /api/v1/clouds/{id}
func (h *CloudHandlers) Get(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	t, err := h.Clouds.GetTask(r.Context(), u, id)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toCloudResp(t))
}

// ListMine GET /api/v1/clouds
func (h *CloudHandlers) ListMine(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	tasks, err := h.Clouds.ListMyTasks(r.Context(), u, 20)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	out := make([]cloudTaskResp, 0, len(tasks))
	for i := range tasks {
		out = append(out, toCloudResp(&tasks[i]))
	}
	writeJSON(w, http.StatusOK, out)
}
