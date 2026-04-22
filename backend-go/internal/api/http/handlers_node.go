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

// --- 编辑历史 M2-F3 ---

type updateContentReq struct {
	Content    string `json:"content"`
	EditReason string `json:"edit_reason"`
}

type revisionResp struct {
	ID         string    `json:"id"`
	NodeID     string    `json:"node_id"`
	RevNumber  int       `json:"rev_number"`
	Content    string    `json:"content"`
	Title      string    `json:"title"`
	EditorID   string    `json:"editor_id"`
	EditReason string    `json:"edit_reason"`
	CreatedAt  time.Time `json:"created_at"`
}

func toRevisionResp(rev *domain.NodeRevision) revisionResp {
	return revisionResp{
		ID: rev.ID, NodeID: rev.NodeID, RevNumber: rev.RevNumber,
		Content: rev.Content, Title: rev.Title,
		EditorID: rev.EditorID, EditReason: rev.EditReason,
		CreatedAt: rev.CreatedAt,
	}
}

// UpdateContent PUT /api/v1/nodes/{id}/content
func (h *NodeHandlers) UpdateContent(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	var body updateContentReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	n, err := h.Nodes.UpdateContent(r.Context(), u, service.UpdateContentInput{
		NodeID:     id,
		Content:    body.Content,
		EditReason: body.EditReason,
	})
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toNodeResp(n))
}

// ListRevisions GET /api/v1/nodes/{id}/revisions?limit=50
func (h *NodeHandlers) ListRevisions(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	revs, err := h.Nodes.ListRevisions(r.Context(), u, id, limit)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	out := make([]revisionResp, 0, len(revs))
	for i := range revs {
		out = append(out, toRevisionResp(&revs[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"revisions": out})
}

// GetRevision GET /api/v1/nodes/{id}/revisions/{rev}
func (h *NodeHandlers) GetRevision(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	revStr := chi.URLParam(r, "rev")
	rev, err := strconv.Atoi(revStr)
	if err != nil || rev <= 0 {
		writeError(w, http.StatusBadRequest, "invalid rev")
		return
	}
	r2, err := h.Nodes.GetRevision(r.Context(), u, id, rev)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toRevisionResp(r2))
}

// Rollback POST /api/v1/nodes/{id}/rollback {"target_rev_number": N}
func (h *NodeHandlers) Rollback(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	var body struct {
		TargetRevNumber int `json:"target_rev_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	n, err := h.Nodes.Rollback(r.Context(), u, id, body.TargetRevNumber)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toNodeResp(n))
}

// Search GET /api/v1/search?q=TEXT&lake_id=UUID&limit=N
// P12-D：湖内节点全文搜索，调用方须是湖成员（或湖为公开湖）。
func (h *NodeHandlers) Search(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}
	if len(q) > 500 {
		writeError(w, http.StatusBadRequest, "q too long (max 500 chars)")
		return
	}
	lakeID := r.URL.Query().Get("lake_id")
	if lakeID == "" {
		writeError(w, http.StatusBadRequest, "lake_id is required")
		return
	}
	limit := 20
	if ls := r.URL.Query().Get("limit"); ls != "" {
		if n, err := strconv.Atoi(ls); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}
	results, err := h.Nodes.SearchNodes(r.Context(), u, lakeID, q, limit)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	type searchHit struct {
		NodeID  string  `json:"node_id"`
		LakeID  string  `json:"lake_id"`
		Snippet string  `json:"snippet"`
		Score   float64 `json:"score"`
	}
	out := make([]searchHit, 0, len(results))
	for _, r := range results {
		out = append(out, searchHit{NodeID: r.NodeID, LakeID: r.LakeID, Snippet: r.Snippet, Score: r.Score})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out})
}

// BatchImport POST /api/v1/lakes/{id}/nodes/batch
// P12-A：批量导入节点（最多 100 个），权限 NAVIGATOR+。
func (h *NodeHandlers) BatchImport(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	lakeID := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024*1024) // 4 MB

	var body struct {
		Nodes []struct {
			Content string `json:"content"`
			Type    string `json:"type"`
		} `json:"nodes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	items := make([]service.BatchImportItem, 0, len(body.Nodes))
	for _, n := range body.Nodes {
		items = append(items, service.BatchImportItem{
			Content: n.Content,
			Type:    domain.NodeType(n.Type),
		})
	}

	result, err := h.Nodes.BatchImportNodes(r.Context(), u, lakeID, items)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}

	out := make([]nodeResp, 0, len(result.Nodes))
	for _, n := range result.Nodes {
		out = append(out, toNodeResp(n))
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"created": result.Created,
		"nodes":   out,
	})
}

// BatchOperate POST /api/v1/lakes/{id}/nodes/batch_op  P14-C：批量节点操作。
// 支持 action=evaporate（蒸发）和 action=condense（凝露）。最多 200 个节点。
func (h *NodeHandlers) BatchOperate(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	lakeID := chi.URLParam(r, "id")

	var body struct {
		Action  string   `json:"action"`
		NodeIDs []string `json:"node_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(body.NodeIDs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"succeeded": 0, "failed": 0})
		return
	}

	result, err := h.Nodes.BatchOperate(r.Context(), u, lakeID, body.Action, body.NodeIDs)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"succeeded": result.Succeeded,
		"failed":    result.Failed,
	})
}
