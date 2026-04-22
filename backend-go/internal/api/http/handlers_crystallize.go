package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/chenqilscy/ripple/backend-go/internal/service"
)

// CrystallizeHandlers M3-S2 凝结相关 HTTP 端点。
type CrystallizeHandlers struct {
	Svc *service.CrystallizeService
}

type crystallizeReq struct {
	LakeID        string   `json:"lake_id"`
	SourceNodeIDs []string `json:"source_node_ids"`
	TitleHint     string   `json:"title_hint,omitempty"`
}

type permaResp struct {
	ID            string   `json:"id"`
	LakeID        string   `json:"lake_id"`
	OwnerID       string   `json:"owner_id"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	SourceNodeIDs []string `json:"source_node_ids"`
	LLMProvider   string   `json:"llm_provider,omitempty"`
	LLMCostTokens int64    `json:"llm_cost_tokens,omitempty"`
	CreatedAt     string   `json:"created_at"`
}

// Create POST /api/v1/perma_nodes
func (h *CrystallizeHandlers) Create(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	var in crystallizeReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	p, err := h.Svc.Crystallize(r.Context(), u, service.CrystallizeInput{
		LakeID: in.LakeID, SourceNodeIDs: in.SourceNodeIDs, TitleHint: in.TitleHint,
	})
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, permaResp{
		ID: p.ID, LakeID: p.LakeID, OwnerID: p.OwnerID,
		Title: p.Title, Summary: p.Summary, SourceNodeIDs: p.SourceNodeIDs,
		LLMProvider: p.LLMProvider, LLMCostTokens: p.LLMCostTokens,
		CreatedAt: p.CreatedAt.UTC().Format("2006-01-02T15:04:05.999Z"),
	})
}
