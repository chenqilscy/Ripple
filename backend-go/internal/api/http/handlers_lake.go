package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/presence"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// LakeHandlers 湖泊 HTTP 处理器。
type LakeHandlers struct {
	Lakes  *service.LakeService
	Spaces *service.SpaceService
	Orgs   *service.OrgService // P13-A：组织归属校验
	Nodes  *service.NodeService // P13-D：内容导出
	Edges  *service.EdgeService // P13-D：内容导出（边，可选）
	// Presence 可选：用于湖健康度在线人数统计。
	Presence *presence.Service
}

type createLakeReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
	SpaceID     string `json:"space_id,omitempty"`
}

type lakeResp struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
	OwnerID     string `json:"owner_id"`
	SpaceID     string `json:"space_id,omitempty"`
	OrgID       string `json:"org_id,omitempty"`
	Role        string `json:"role,omitempty"`
}

// Create POST /api/v1/lakes
func (h *LakeHandlers) Create(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	var in createLakeReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	// 若指定了 space_id，必须验证调用者是该 space 成员（OWNER/EDITOR/VIEWER 均可加湖）。
	if in.SpaceID != "" && h.Spaces != nil {
		if _, _, err := h.Spaces.Get(r.Context(), u, in.SpaceID); err != nil {
			writeError(w, mapDomainError(err), err.Error())
			return
		}
	}
	l, err := h.Lakes.Create(r.Context(), u, service.CreateLakeInput{
		Name: in.Name, Description: in.Description, IsPublic: in.IsPublic, SpaceID: in.SpaceID,
	})
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, lakeResp{
		ID: l.ID, Name: l.Name, Description: l.Description,
		IsPublic: l.IsPublic, OwnerID: l.OwnerID, SpaceID: l.SpaceID, OrgID: l.OrgID, Role: "OWNER",
	})
}

// Get GET /api/v1/lakes/{id}
func (h *LakeHandlers) Get(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	l, role, err := h.Lakes.Get(r.Context(), u, id)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, lakeResp{
		ID: l.ID, Name: l.Name, Description: l.Description,
		IsPublic: l.IsPublic, OwnerID: l.OwnerID, SpaceID: l.SpaceID, OrgID: l.OrgID, Role: string(role),
	})
}

type lakeHealthResp struct {
	LakeID      string         `json:"lake_id"`
	Score       int            `json:"score"`
	Grade       string         `json:"grade"`
	Trend       string         `json:"trend"`
	Thresholds  map[string]any `json:"thresholds"`
	Summary     map[string]any `json:"summary"`
	StateCounts map[string]int `json:"state_counts"`
}

// GetHealth GET /api/v1/lakes/{id}/health
// 3-P2-02：湖健康度仪表盘 API（实时快照，无历史回放）。
func (h *LakeHandlers) GetHealth(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	if h.Nodes == nil || h.Edges == nil {
		writeError(w, http.StatusServiceUnavailable, "lake health not available")
		return
	}

	nodes, err := h.Nodes.ListByLake(r.Context(), u, id, false)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	edges, err := h.Edges.ListByLake(r.Context(), u, id, false)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}

	stateCounts := map[string]int{
		"DROP": 0, "MIST": 0, "CLOUD": 0, "PERMA": 0,
		"VAPOR": 0, "ERASED": 0, "FROZEN": 0,
	}
	for i := range nodes {
		state := string(nodes[i].State)
		if _, ok := stateCounts[state]; ok {
			stateCounts[state]++
		}
	}

	totalNodes := len(nodes)
	activeNodes := totalNodes - stateCounts["VAPOR"] - stateCounts["ERASED"]
	edgeCount := len(edges)
	density := 0.0
	if totalNodes > 1 {
		density = float64(edgeCount) / (float64(totalNodes*(totalNodes-1)) / 2.0)
	}
	permaRatio := 0.0
	if totalNodes > 0 {
		permaRatio = float64(stateCounts["PERMA"]) / float64(totalNodes)
	}

	onlineUsers := 0
	if h.Presence != nil {
		if users, perr := h.Presence.List(r.Context(), id); perr == nil {
			onlineUsers = len(users)
		}
	}
	// collaboration: 归一化在线人数；目标为 5 人满分（见 thresholds.collaboration_goal）
	collaboration := math.Min(float64(onlineUsers)/5.0, 1.0)
	score := int(math.Round(math.Min(100.0, 45.0*permaRatio+35.0*math.Min(density/0.35, 1.0)+20.0*collaboration)))

	grade := "偏弱"
	trend := "weak"
	switch {
	case score >= 80:
		grade, trend = "优秀", "rising"
	case score >= 60:
		grade, trend = "健康", "steady"
	case score >= 40:
		grade, trend = "一般", "steady"
	}

	writeJSON(w, http.StatusOK, lakeHealthResp{
		LakeID: id,
		Score:  score,
		Grade:  grade,
		Trend:  trend,
		Thresholds: map[string]any{
			"excellent_score":   80,
			"healthy_score":     60,
			"density_target":    0.35,
			"collaboration_goal": 5,
		},
		Summary: map[string]any{
			"total_nodes":   totalNodes,
			"active_nodes":  activeNodes,
			"edge_count":    edgeCount,
			"density":       density,
			"perma_ratio":   permaRatio,
			"online_users":  onlineUsers,
		},
		StateCounts: stateCounts,
	})
}

// ListMine GET /api/v1/lakes  （可选 ?space_id=xxx 过滤）
func (h *LakeHandlers) ListMine(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	spaceID := r.URL.Query().Get("space_id")
	items, err := h.Lakes.ListMineBySpace(r.Context(), u, spaceID)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	out := make([]lakeResp, 0, len(items))
	ids := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, lakeResp{
			ID: it.Lake.ID, Name: it.Lake.Name, Description: it.Lake.Description,
			IsPublic: it.Lake.IsPublic, OwnerID: it.Lake.OwnerID, SpaceID: it.Lake.SpaceID, OrgID: it.Lake.OrgID, Role: string(it.Role),
		})
		ids = append(ids, it.Lake.ID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"lakes": out, "lake_ids": ids})
}

// Move PATCH /api/v1/lakes/{id}/space  body: {"space_id": "" | "<uuid>"}
// 仅 Owner；目标 space 非空时 actor 必须是该 space 的成员。
func (h *LakeHandlers) Move(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "lake id required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var in struct {
		SpaceID string `json:"space_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	l, err := h.Lakes.MoveToSpace(r.Context(), u, id, in.SpaceID, h.Spaces)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, lakeResp{
		ID: l.ID, Name: l.Name, Description: l.Description,
		IsPublic: l.IsPublic, OwnerID: l.OwnerID, SpaceID: l.SpaceID, OrgID: l.OrgID,
	})
}

// SetLakeOrg PATCH /api/v1/lakes/{id}/org  P13-A：设置/清除湖的组织归属。
// body: {"org_id": "" | "<uuid>"}
func (h *LakeHandlers) SetLakeOrg(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "lake id required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var in struct {
		OrgID string `json:"org_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	l, err := h.Lakes.SetLakeOrg(r.Context(), u, id, in.OrgID, h.Orgs)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, lakeResp{
		ID: l.ID, Name: l.Name, Description: l.Description,
		IsPublic: l.IsPublic, OwnerID: l.OwnerID, SpaceID: l.SpaceID, OrgID: l.OrgID,
	})
}

// UpdateMemberRole PUT /api/v1/lakes/{id}/members/{userID}/role
// P10-C：变更湖成员角色（仅 OWNER 可操作；不能升级为 OWNER；不能修改自己）。
func (h *LakeHandlers) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	lakeID := chi.URLParam(r, "id")
	targetUserID := chi.URLParam(r, "userID")
	if lakeID == "" || targetUserID == "" {
		writeError(w, http.StatusBadRequest, "lake id and user id required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 512)
	var in struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Role == "" {
		writeError(w, http.StatusBadRequest, "role is required")
		return
	}
	if err := h.Lakes.UpdateMemberRole(r.Context(), actor, lakeID, targetUserID, domain.Role(in.Role)); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListMembers GET /api/v1/lakes/{id}/members
// P11-C：返回湖成员列表，调用方须至少是 OBSERVER。
func (h *LakeHandlers) ListMembers(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	lakeID := chi.URLParam(r, "id")
	if lakeID == "" {
		writeError(w, http.StatusBadRequest, "lake id required")
		return
	}
	members, err := h.Lakes.ListMembers(r.Context(), actor, lakeID)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	type memberView struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	out := make([]memberView, 0, len(members))
	for _, m := range members {
		out = append(out, memberView{UserID: m.UserID, Role: string(m.Role)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": out})
}

// RemoveMember DELETE /api/v1/lakes/{id}/members/{userID}
// P16-C：从湖中移除成员（仅 OWNER 可操作；不能移除自身或另一个 OWNER）。
func (h *LakeHandlers) RemoveMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	lakeID := chi.URLParam(r, "id")
	targetUserID := chi.URLParam(r, "userID")
	if lakeID == "" || targetUserID == "" {
		writeError(w, http.StatusBadRequest, "lake id and user id required")
		return
	}
	if err := h.Lakes.RemoveMember(r.Context(), actor, lakeID, targetUserID); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Export GET /api/v1/lakes/{id}/export?format=json|markdown  P13-D：内容导出。
// 调用方须至少是 OBSERVER；最多导出 10000 节点。
const exportMaxNodes = 10_000

func (h *LakeHandlers) Export(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	lakeID := chi.URLParam(r, "id")
	if lakeID == "" {
		writeError(w, http.StatusBadRequest, "lake id required")
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "markdown" {
		writeError(w, http.StatusBadRequest, "format must be json or markdown")
		return
	}

	l, _, err := h.Lakes.Get(r.Context(), actor, lakeID)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}

	nodes, err := h.Nodes.ListByLake(r.Context(), actor, lakeID, false)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	if len(nodes) > exportMaxNodes {
		writeError(w, http.StatusInsufficientStorage,
			fmt.Sprintf("lake has too many nodes (%d), export limit is %d", len(nodes), exportMaxNodes))
		return
	}

	var edges []domain.Edge
	if h.Edges != nil {
		edges, err = h.Edges.ListByLake(r.Context(), actor, lakeID, false)
		if err != nil {
			writeError(w, mapDomainError(err), err.Error())
			return
		}
	} else {
		edges = []domain.Edge{}
	}

	switch format {
	case "json":
		type nodeExport struct {
			ID        string  `json:"id"`
			Content   string  `json:"content"`
			Type      string  `json:"type"`
			State     string  `json:"state"`
			OwnerID   string  `json:"owner_id"`
			CreatedAt string  `json:"created_at"`
			UpdatedAt string  `json:"updated_at"`
		}
		type edgeExport struct {
			ID        string `json:"id"`
			SrcNodeID string `json:"src_node_id"`
			DstNodeID string `json:"dst_node_id"`
			Kind      string `json:"kind"`
			Label     string `json:"label,omitempty"`
		}
		type lakeExport struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			IsPublic    bool   `json:"is_public"`
			OwnerID     string `json:"owner_id"`
		}
		nodeOut := make([]nodeExport, 0, len(nodes))
		for _, n := range nodes {
			nodeOut = append(nodeOut, nodeExport{
				ID: n.ID, Content: n.Content, Type: string(n.Type),
				State: string(n.State), OwnerID: n.OwnerID,
				CreatedAt: n.CreatedAt.Format("2006-01-02T15:04:05Z"),
				UpdatedAt: n.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			})
		}
		edgeOut := make([]edgeExport, 0, len(edges))
		for _, e := range edges {
			edgeOut = append(edgeOut, edgeExport{
				ID: e.ID, SrcNodeID: e.SrcNodeID, DstNodeID: e.DstNodeID,
				Kind: string(e.Kind), Label: e.Label,
			})
		}
		payload := map[string]any{
			"lake":  lakeExport{ID: l.ID, Name: l.Name, Description: l.Description, IsPublic: l.IsPublic, OwnerID: l.OwnerID},
			"nodes": nodeOut,
			"edges": edgeOut,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "export failed")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="lake-%s.json"`, lakeID))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)

	case "markdown":
		var sb strings.Builder
		sb.WriteString("# ")
		sb.WriteString(l.Name)
		sb.WriteString("\n\n")
		if l.Description != "" {
			sb.WriteString("> ")
			sb.WriteString(l.Description)
			sb.WriteString("\n\n")
		}
		for _, n := range nodes {
			if n.State == domain.StateVapor || n.State == domain.StateErased {
				continue
			}
			sb.WriteString("## ")
			sb.WriteString(fmt.Sprintf("[%s · %s]", string(n.Type), string(n.State)))
			sb.WriteString("\n\n")
			sb.WriteString(n.Content)
			sb.WriteString("\n\n---\n\n")
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="lake-%s.md"`, lakeID))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sb.String()))
	}
}

// Import POST /api/v1/lakes/{id}/import  P13-E：导入外部内容。
// 支持 .json（与 P13-D 导出格式相同）和 .md（Markdown，按 ## 标题分割为节点）。
// 最大文件 10 MB；最多创建 importMaxNodes 个节点。
const importMaxNodes = exportMaxNodes

func (h *LakeHandlers) Import(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	lakeID := chi.URLParam(r, "id")

	// 文件大小限制 10 MB
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid form: "+err.Error())
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field required")
		return
	}
	defer f.Close()

	// 检测格式：按文件名后缀
	name := strings.ToLower(hdr.Filename)
	var format string
	switch {
	case strings.HasSuffix(name, ".json"):
		format = "json"
	case strings.HasSuffix(name, ".md"), strings.HasSuffix(name, ".markdown"):
		format = "markdown"
	default:
		writeError(w, http.StatusBadRequest, "unsupported file type, must be .json or .md")
		return
	}

	var contents []string // 每条为一个节点的 content
	switch format {
	case "json":
		type nodeImport struct {
			Content string `json:"content"`
			Type    string `json:"type"`
		}
		type importPayload struct {
			Nodes []nodeImport `json:"nodes"`
		}
		var payload importPayload
		if err := json.NewDecoder(f).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
		for _, n := range payload.Nodes {
			c := strings.TrimSpace(n.Content)
			if c != "" {
				contents = append(contents, c)
			}
		}
	case "markdown":
		data, err := io.ReadAll(f)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read file: "+err.Error())
			return
		}
		sections := splitMarkdownSections(string(data))
		contents = append(contents, sections...)
	}

	if len(contents) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"imported": 0, "skipped": 0})
		return
	}
	if len(contents) > importMaxNodes {
		writeError(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("too many nodes to import (%d), limit is %d", len(contents), importMaxNodes))
		return
	}

	imported, skipped := 0, 0
	for _, c := range contents {
		_, err := h.Nodes.Create(r.Context(), actor, service.CreateNodeInput{
			LakeID:  lakeID,
			Content: c,
			Type:    domain.NodeTypeText,
		})
		if err != nil {
			skipped++
		} else {
			imported++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": imported, "skipped": skipped})
}

// splitMarkdownSections 按 ## 二级标题将 Markdown 文本分割为段落。
// 无标题时将整个文本作为一个节点。
func splitMarkdownSections(md string) []string {
	lines := strings.Split(md, "\n")
	var sections []string
	var cur strings.Builder

	flush := func() {
		s := strings.TrimSpace(cur.String())
		if s != "" {
			sections = append(sections, s)
		}
		cur.Reset()
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# ") {
			flush()
			// 把标题行本身也纳入段落内容（用作节点内容的第一行）
			cur.WriteString(line)
			cur.WriteString("\n")
		} else {
			cur.WriteString(line)
			cur.WriteString("\n")
		}
	}
	flush()
	return sections
}