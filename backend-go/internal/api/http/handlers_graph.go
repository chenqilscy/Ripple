package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/llm"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

// GraphAnalysisHandlers 图谱分析端点（推荐/路径/聚类/规划）
type GraphAnalysisHandlers struct {
	Nodes       *service.NodeService
	Edges       *service.EdgeService
	Recommender *service.RecommenderService
	Feedback    store.FeedbackRepository
	LLM         llm.Router // AI 标签生成（可选；nil 时回退到 "领域 N" 占位符）
	Heat        *service.HeatService // 热度计算（Phase 3-B.3）
}

// ---- 推荐 ----

// GetRecommendations GET /api/v1/lakes/{lake_id}/recommendations
func (h *GraphAnalysisHandlers) GetRecommendations(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	lakeID := chi.URLParam(r, "lake_id")

	// 构建已有边集合（用于 fallback 的内容相似度推荐）
	edges, err := h.Edges.ListByLake(r.Context(), u, lakeID, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list edges: "+err.Error())
		return
	}
	edgeSet := make(map[[2]string]bool)
	for _, e := range edges {
		if e.DeletedAt == nil {
			edgeSet[[2]string{e.SrcNodeID, e.DstNodeID}] = true
		}
	}

	// 提取节点内容对（用于 fallback 的内容相似度推荐）
	nodes, err := h.Nodes.ListByLake(r.Context(), u, lakeID, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list nodes: "+err.Error())
		return
	}
	nodePairs := make([]struct {
		ID      string
		Content string
	}, len(nodes))
	for i, n := range nodes {
		if n.State == domain.StateErased || n.State == domain.StateGhost {
			continue
		}
		nodePairs[i] = struct {
			ID      string
			Content string
		}{ID: n.ID, Content: n.Content}
	}

	var recs []recommendationRes

	// 优先尝试协同过滤推荐
	if h.Recommender != nil {
		collabRecs, err := h.Recommender.Recommend(r.Context(), u, service.RecommendInput{
			TargetType: "node",
			Limit:      20,
		})
		if err == nil && len(collabRecs) > 0 {
			// 计算最大分用于置信度归一化
			maxScore := int64(0)
			for _, rec := range collabRecs {
				if rec.Score > maxScore {
					maxScore = rec.Score
				}
			}
			now := time.Now().Format(time.RFC3339)
			for _, rec := range collabRecs {
				confidence := float64(0)
				if maxScore > 0 {
					confidence = float64(rec.Score) / float64(maxScore)
				}
				recs = append(recs, recommendationRes{
					ID:           uuid.New().String(),
					SourceNodeID: "", // 协同过滤不提供源节点
					TargetNodeID: rec.TargetID,
					Reason:       "协同过滤推荐（基于相似用户偏好）",
					Confidence:   confidence,
					CreatedAt:    now,
					Status:       "pending",
				})
			}
		}
	}

	// 如果协同过滤结果为空，fallback 到基于内容相似度的推荐
	if len(recs) == 0 {
		recs = generateRecommendations(nodePairs, edgeSet)
	}

	writeJSON(w, http.StatusOK, map[string]any{"recommendations": recs})
}

type recommendationRes struct {
	ID           string  `json:"id"`
	SourceNodeID string  `json:"source_node_id"`
	TargetNodeID string  `json:"target_node_id"`
	Reason       string  `json:"reason"`
	Confidence   float64 `json:"confidence"`
	CreatedAt    string  `json:"created_at"`
	Status       string  `json:"status"`
}

// generateRecommendations 基于内容相似度的推荐生成
func generateRecommendations(nodes []struct{ ID, Content string }, existingEdges map[[2]string]bool) []recommendationRes {
	// 相似度阈值：0.3（Jaccard），低于此值不推荐，避免噪音
	const THRESHOLD = 0.3
	var recs []recommendationRes

	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			if nodes[i].ID == "" || nodes[j].ID == "" {
				continue
			}
			key := [2]string{nodes[i].ID, nodes[j].ID}
			if existingEdges[key] {
				continue
			}
			sim := simpleContentSimilarity(nodes[i].Content, nodes[j].Content)
			if sim >= THRESHOLD {
				recs = append(recs, recommendationRes{
					ID:           uuid.New().String(),
					SourceNodeID: nodes[i].ID,
					TargetNodeID: nodes[j].ID,
					Reason:       fmt.Sprintf("内容相似度 %.0f%%", sim*100),
					Confidence:   sim,
					CreatedAt:    "",
					Status:       "pending",
				})
			}
		}
	}
	return recs
}

// simpleContentSimilarity 基于词重叠的简单相似度
func simpleContentSimilarity(a, b string) float64 {
	aWords := wordSet(a)
	bWords := wordSet(b)
	if len(aWords) == 0 || len(bWords) == 0 {
		return 0
	}
	intersection := 0
	for w := range aWords {
		if bWords[w] {
			intersection++
		}
	}
	union := float64(len(aWords) + len(bWords) - intersection)
	if union == 0 {
		return 0
	}
	return float64(intersection) / union
}

func wordSet(s string) map[string]bool {
	words := make(map[string]bool)
	prev := rune(0)
	cur := make([]rune, 0, len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r >= 0x4e00 {
			cur = append(cur, r)
		} else {
			if len(cur) >= 2 {
				words[string(cur)] = true
			}
			cur = cur[:0]
		}
		prev = r
	}
	if len(cur) >= 2 {
		words[string(cur)] = true
	}
	_ = prev
	return words
}

// ---- 路径 ----

type pathReq struct {
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	LakeID   string `json:"lake_id"` // lake_id 放在 body 中，与路由设计一致
}

// GetPath POST /api/v1/graph/path
func (h *GraphAnalysisHandlers) GetPath(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	var body pathReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if body.LakeID == "" {
		writeError(w, http.StatusBadRequest, "lake_id is required")
		return
	}
	lakeID := body.LakeID

	edges, err := h.Edges.ListByLake(r.Context(), u, lakeID, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 构建邻接表
	adj := make(map[string][]string)
	for _, e := range edges {
		if e.DeletedAt != nil {
			continue
		}
		adj[e.SrcNodeID] = append(adj[e.SrcNodeID], e.DstNodeID)
		// 双向
		adj[e.DstNodeID] = append(adj[e.DstNodeID], e.SrcNodeID)
	}

	path := bfsPath(adj, body.SourceID, body.TargetID)
	nodes, _ := h.Nodes.ListByLake(r.Context(), u, lakeID, true)
	nodeMap := make(map[string]string)
	for _, n := range nodes {
		if n.State == domain.StateErased || n.State == domain.StateGhost {
			continue
		}
		// 取前50字符作为 title
		content := n.Content
		if len(content) > 50 {
			content = content[:50] + "..."
		}
		nodeMap[n.ID] = content
	}

	pathNodes := make([]struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Reason string `json:"reason"`
	}, len(path))
	for i, id := range path {
		title := id
		if t, ok := nodeMap[id]; ok {
			title = t
		}
		reason := ""
		if i > 0 && i < len(path) {
			reason = "关联节点"
		}
		pathNodes[i] = struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Reason string `json:"reason"`
		}{ID: id, Title: title, Reason: reason}
	}

	steps := 0
	if len(path) > 1 {
		steps = len(path) - 1
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"source_id":   body.SourceID,
		"target_id":   body.TargetID,
		"nodes":       pathNodes,
		"edges":       []any{},
		"total_steps": steps,
	})
}

// bfsPath BFS 找两点间最短路径
func bfsPath(adj map[string][]string, src, dst string) []string {
	if src == dst {
		return []string{src}
	}
	queue := [][]string{{src}}
	visited := map[string]bool{src: true}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		curr := path[len(path)-1]
		for _, next := range adj[curr] {
			if next == dst {
				return append(path, next)
			}
			if !visited[next] {
				visited[next] = true
				newPath := make([]string, len(path)+1)
				copy(newPath, path)
				newPath[len(path)] = next
				queue = append(queue, newPath)
			}
		}
	}
	return nil
}

// ---- 聚类 ----

// GetClusters GET /api/v1/lakes/{lake_id}/clusters
func (h *GraphAnalysisHandlers) GetClusters(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	lakeID := chi.URLParam(r, "lake_id")
	nodes, err := h.Nodes.ListByLake(r.Context(), u, lakeID, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	edges, err := h.Edges.ListByLake(r.Context(), u, lakeID, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 基于连通分量聚类
	adj := make(map[string][]string)
	for _, e := range edges {
		if e.DeletedAt != nil {
			continue
		}
		adj[e.SrcNodeID] = append(adj[e.SrcNodeID], e.DstNodeID)
		adj[e.DstNodeID] = append(adj[e.DstNodeID], e.SrcNodeID)
	}

	colors := []string{"#2e8b90", "#4a8eff", "#52c41a", "#faad14", "#f5222d", "#722ed1", "#eb2f96", "#13c2c2"}
	visited := make(map[string]bool)
	var clusters []struct {
		ID            string   `json:"id"`
		Label         string   `json:"label"`
		NodeIDs       []string `json:"node_ids"`
		Color         string   `json:"color"`
		BridgeNodeIDs []string `json:"bridge_node_ids"`
		Density       float64  `json:"density"`
	}

	activeNodes := make(map[string]bool)
	for _, n := range nodes {
		if n.State != domain.StateErased && n.State != domain.StateGhost {
			activeNodes[n.ID] = true
		}
	}

	for _, n := range nodes {
		if n.State == domain.StateErased || n.State == domain.StateGhost {
			continue
		}
		if visited[n.ID] {
			continue
		}
		component := bfsComponent(adj, n.ID, visited)
		// 预建当前 component 的节点集合，用于密度计算和桥接节点识别
		compSet := make(map[string]bool)
		for _, nid := range component {
			compSet[nid] = true
		}

		// 统计当前 component 内部的边数（用于密度）
		clusterEdges := 0
		for _, e := range edges {
			if e.DeletedAt != nil {
				continue
			}
			if compSet[e.SrcNodeID] && compSet[e.DstNodeID] {
				clusterEdges++
			}
		}

		clusterID := fmt.Sprintf("cluster-%d", len(clusters))
		clusters = append(clusters, struct {
			ID            string   `json:"id"`
			Label         string   `json:"label"`
			NodeIDs       []string `json:"node_ids"`
			Color         string   `json:"color"`
			BridgeNodeIDs []string `json:"bridge_node_ids"`
			Density       float64  `json:"density"`
		}{
			ID:            clusterID,
			Label:         fmt.Sprintf("领域 %d", len(clusters)+1),
			NodeIDs:       component,
			Color:         colors[len(clusters)%len(colors)],
			BridgeNodeIDs: findBridgeNodes(component, adj, len(clusters)),
			Density:       float64(clusterEdges) / float64(len(component)+1),
		})
	}

	// AI 标签生成：如果 LLM 可用，用 AI 替换占位符 "领域 N"
	if h.LLM != nil && len(clusters) > 0 {
		nodeMap := make(map[string]string)
		for _, n := range nodes {
			if n.State != domain.StateErased && n.State != domain.StateGhost {
				nodeMap[n.ID] = n.Content
			}
		}

		for i := range clusters {
			var contents []string
			for _, nid := range clusters[i].NodeIDs {
				if c, ok := nodeMap[nid]; ok {
					contents = append(contents, c)
					if len(contents) >= 5 { // 最多取 5 个节点内容作为 context
						break
					}
				}
			}
			if len(contents) > 0 {
				prompt := fmt.Sprintf("这些知识节点的内容摘要如下：\n%s\n\n请用2-4个字概括这些节点所属的领域主题，只返回一个词，例如「系统设计」「算法原理」「产品运营」。", strings.Join(contents, "\n---\n"))
				cands, err := h.LLM.Generate(r.Context(), llm.GenerateRequest{
					Prompt:   prompt,
					N:        1,
					Modality: llm.ModalityText,
				})
				if err == nil && len(cands) > 0 && cands[0].Text != "" {
					label := strings.TrimSpace(cands[0].Text)
					label = strings.TrimPrefix(label, "\"")
					label = strings.TrimSuffix(label, "\"")
					clusters[i].Label = label
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"clusters": clusters})
}

func bfsComponent(adj map[string][]string, start string, visited map[string]bool) []string {
	var component []string
	queue := []string{start}
	visited[start] = true
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		component = append(component, curr)
		for _, next := range adj[curr] {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}
	return component
}

func findBridgeNodes(component []string, adj map[string][]string, clusterIdx int) []string {
	// 简单实现：在 component 内找出度数>1 的节点视为桥接节点
	var bridges []string
	compSet := make(map[string]bool)
	for _, id := range component {
		compSet[id] = true
	}
	for _, id := range component {
		for _, neighbor := range adj[id] {
			if compSet[neighbor] && neighbor != id {
				bridges = append(bridges, id)
				break
			}
		}
	}
	if len(bridges) > 5 {
		bridges = bridges[:5]
	}
	return bridges
}

// ---- 规划 ----

// GetPlanning GET /api/v1/lakes/{lake_id}/planning
func (h *GraphAnalysisHandlers) GetPlanning(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	lakeID := chi.URLParam(r, "lake_id")
	nodes, err := h.Nodes.ListByLake(r.Context(), u, lakeID, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	edges, err := h.Edges.ListByLake(r.Context(), u, lakeID, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 构建邻接表
	adj := make(map[string][]string)
	for _, e := range edges {
		if e.DeletedAt != nil {
			continue
		}
		adj[e.SrcNodeID] = append(adj[e.SrcNodeID], e.DstNodeID)
		adj[e.DstNodeID] = append(adj[e.DstNodeID], e.SrcNodeID)
	}

	var suggestions []struct {
		ID             string   `json:"id"`
		Type           string   `json:"type"`
		Title          string   `json:"title"`
		Description    string   `json:"description"`
		Priority       string   `json:"priority"`
		RelatedNodeIDs []string `json:"related_node_ids"`
	}

	// 孤岛检测
	for _, n := range nodes {
		if n.State == domain.StateErased || n.State == domain.StateGhost {
			continue
		}
		if len(adj[n.ID]) == 0 {
			suggestions = append(suggestions, struct {
				ID             string   `json:"id"`
				Type           string   `json:"type"`
				Title          string   `json:"title"`
				Description    string   `json:"description"`
				Priority       string   `json:"priority"`
				RelatedNodeIDs []string `json:"related_node_ids"`
			}{
				ID:             uuid.New().String(),
				Type:           "explore",
				Title:          "孤立节点",
				Description:    "该节点没有关联，考虑与其他节点建立关联以丰富知识网络",
				Priority:       "medium",
				RelatedNodeIDs: []string{n.ID},
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"suggestions": suggestions})
}

// AcceptRecommendation POST /api/v1/lakes/{lake_id}/recommendations/{id}/accept
func (h *GraphAnalysisHandlers) AcceptRecommendation(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	recID := chi.URLParam(r, "id")

	// 从请求体读取 source_node_id 和 target_node_id
	var body struct {
		SourceNodeID string `json:"source_node_id"`
		TargetNodeID string `json:"target_node_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.SourceNodeID == "" || body.TargetNodeID == "" {
		writeError(w, http.StatusBadRequest, "source_node_id and target_node_id required")
		return
	}

	// 调用 EdgeService.Create 创建关联
	edge, err := h.Edges.Create(r.Context(), u, service.CreateEdgeInput{
		SrcNodeID: body.SourceNodeID,
		DstNodeID: body.TargetNodeID,
		Kind:      domain.EdgeKindRelates,
		Strength:  0.5,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 同时写一条 positive feedback 事件（用于改进后续推荐质量）
	if h.Feedback != nil {
		h.Feedback.AddEvent(r.Context(), store.FeedbackEvent{
			ID:         uuid.NewString(),
			UserID:     u.ID,
			TargetType: "recommendation",
			TargetID:   recID,
			EventType:  "accept",
			Payload:    fmt.Sprintf(`{"accepted_edge_id":"%s"}`, edge.ID),
		})
	}

	writeJSON(w, http.StatusCreated, map[string]any{"id": edge.ID, "status": "accepted"})
}

// RejectRecommendation POST /api/v1/lakes/{lake_id}/recommendations/{id}/reject
func (h *GraphAnalysisHandlers) RejectRecommendation(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	recID := chi.URLParam(r, "id")
	if h.Feedback != nil {
		h.Feedback.AddEvent(r.Context(), store.FeedbackEvent{
			ID:         uuid.NewString(),
			UserID:     u.ID,
			TargetType: "recommendation",
			TargetID:   recID,
			EventType:  "reject",
			Payload:    "",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": recID, "status": "rejected"})
}

// IgnoreRecommendation POST /api/v1/lakes/{lake_id}/recommendations/{id}/ignore
func (h *GraphAnalysisHandlers) IgnoreRecommendation(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	recID := chi.URLParam(r, "id")
	if h.Feedback != nil {
		h.Feedback.AddEvent(r.Context(), store.FeedbackEvent{
			ID:         uuid.NewString(),
			UserID:     u.ID,
			TargetType: "recommendation",
			TargetID:   recID,
			EventType:  "ignore",
			Payload:    "",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": recID, "status": "ignored"})
}

// AcceptPlanning POST|PUT /api/v1/planning/{id}/accept
func (h *GraphAnalysisHandlers) AcceptPlanning(w http.ResponseWriter, r *http.Request) {
	suggestionID := chi.URLParam(r, "id")
	u, _ := CurrentUser(r.Context())

	// 解析请求体（支持 PUT body 或空 body 的 GET fallback）
	var body struct {
		Type           string   `json:"type"`
		Title          string   `json:"title"`
		Description    string   `json:"description"`
		Priority       string   `json:"priority"`
		RelatedNodeIDs []string `json:"related_node_ids"`
		LakeID         string   `json:"lake_id"`
	}
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
	}

	// 空 body 时走 GET fallback：仅记录 accepted
	if body.Type == "" {
		writeJSON(w, http.StatusOK, map[string]any{"id": suggestionID, "status": "accepted"})
		return
	}

	switch body.Type {
	case "add_node":
		h.acceptAddNode(w, r, u, suggestionID, body)
		return
	case "connect":
		h.acceptConnect(w, r, u, suggestionID, body)
		return
	case "explore":
		writeJSON(w, http.StatusOK, map[string]any{"id": suggestionID, "type": "explore", "status": "accepted"})
		return
	default:
		writeError(w, http.StatusBadRequest, "unsupported suggestion type: "+body.Type)
		return
	}
}

// acceptAddNode 处理 add_node 类型建议
func (h *GraphAnalysisHandlers) acceptAddNode(w http.ResponseWriter, r *http.Request, u *domain.User, suggestionID string, body struct {
	Type           string   `json:"type"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Priority       string   `json:"priority"`
	RelatedNodeIDs []string `json:"related_node_ids"`
	LakeID         string   `json:"lake_id"`
}) {
	lakeID := body.LakeID
	description := body.Description
	if description == "" {
		description = body.Title
	}

	// 获取湖内已有节点作为 context
	nodes, err := h.Nodes.ListByLake(r.Context(), u, lakeID, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list nodes: "+err.Error())
		return
	}

	// 构建已有节点标题列表
	var nodeTitles []string
	for _, n := range nodes {
		title := strings.TrimSpace(n.Content)
		if len(title) > 30 {
			title = title[:30] + "..."
		}
		if title != "" {
			nodeTitles = append(nodeTitles, title)
		}
	}
	existingNodesStr := strings.Join(nodeTitles, "、")
	if existingNodesStr == "" {
		existingNodesStr = "（暂无）"
	}

	// 准备默认值（LLM 不可用时的 fallback）
	content := description

	// 调用 LLM 生成标题/内容/标签
	if h.LLM != nil {
		prompt := fmt.Sprintf(`建议主题：%s
湖内已有节点：%s
请用 JSON 格式生成一个知识节点，格式：{"title":"标题","content":"内容摘要（100字以内）","tags":["标签1","标签2"]}。`, description, existingNodesStr)
		cands, llmErr := h.LLM.Generate(r.Context(), llm.GenerateRequest{
			Prompt:   prompt,
			N:        1,
			Modality: llm.ModalityText,
		})
		if llmErr == nil && len(cands) > 0 && cands[0].Text != "" {
			var gen struct {
				Title   string   `json:"title"`
				Content string   `json:"content"`
				Tags    []string `json:"tags"`
			}
			if jsonErr := json.Unmarshal([]byte(cands[0].Text), &gen); jsonErr == nil {
				if gen.Title != "" {
					content = gen.Title + "：" + content
				}
				if gen.Content != "" {
					content = gen.Content
				}
			}
		}
	}

	// 创建节点
	node, createErr := h.Nodes.Create(r.Context(), u, service.CreateNodeInput{
		LakeID:  lakeID,
		Content: content,
		Type:    domain.NodeTypeText,
	})
	if createErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to create node: "+createErr.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": suggestionID, "type": "add_node", "status": "accepted", "nodeId": node.ID})
}

// acceptConnect 处理 connect 类型建议
func (h *GraphAnalysisHandlers) acceptConnect(w http.ResponseWriter, r *http.Request, u *domain.User, suggestionID string, body struct {
	Type           string   `json:"type"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Priority       string   `json:"priority"`
	RelatedNodeIDs []string `json:"related_node_ids"`
	LakeID         string   `json:"lake_id"`
}) {
	lakeID := body.LakeID
	if len(body.RelatedNodeIDs) == 0 {
		writeError(w, http.StatusBadRequest, "related_node_ids required for connect")
		return
	}

	sourceID := body.RelatedNodeIDs[0]

	// 确定 target：优先用 relatedNodeIds[1]，否则搜索
	var targetID string
	if len(body.RelatedNodeIDs) >= 2 {
		targetID = body.RelatedNodeIDs[1]
	} else {
		// 搜索湖内节点，找关键词重叠最高的（排除 source）
		nodes, err := h.Nodes.ListByLake(r.Context(), u, lakeID, false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list nodes: "+err.Error())
			return
		}
		targetID, err = findBestTargetNode(r.Context(), nodes, sourceID, body.Title, body.Description)
		if err != nil {
			writeError(w, http.StatusBadRequest, "target node not found")
			return
		}
	}

	// 尝试创建边（幂等处理）
	edge, err := h.Edges.Create(r.Context(), u, service.CreateEdgeInput{
		SrcNodeID: sourceID,
		DstNodeID: targetID,
		Kind:      domain.EdgeKindRelates,
		Strength:  0,
	})
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			// 边已存在，查找已有边的 ID
			edges, _ := h.Edges.ListByLake(r.Context(), u, lakeID, false)
			for _, e := range edges {
				if e.SrcNodeID == sourceID && e.DstNodeID == targetID && e.DeletedAt == nil {
					writeJSON(w, http.StatusOK, map[string]any{"id": suggestionID, "type": "connect", "status": "accepted", "edgeId": e.ID})
					return
				}
			}
		}
		writeError(w, http.StatusInternalServerError, "failed to create edge: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": suggestionID, "type": "connect", "status": "accepted", "edgeId": edge.ID})
}

// findBestTargetNode 在湖内节点中搜索与关键词重叠最高的节点（排除 source）
func findBestTargetNode(ctx context.Context, nodes []domain.Node, sourceID, title, description string) (string, error) {
	if title == "" && description == "" {
		return "", fmt.Errorf("no keywords to search")
	}

	// 构建搜索文本
	searchText := strings.ToLower(title + " " + description)
	words := strings.Fields(searchText)
	if len(words) == 0 {
		return "", fmt.Errorf("no keywords")
	}

	var bestNodeID string
	bestScore := 0

	for _, node := range nodes {
		if node.ID == sourceID {
			continue
		}
		nodeText := strings.ToLower(node.Content)

		// 计算词组重叠分数
		score := 0
		for _, word := range words {
			if len(word) < 2 {
				continue
			}
			if strings.Contains(nodeText, word) {
				score++
			}
		}

		if score > bestScore {
			bestScore = score
			bestNodeID = node.ID
		}
	}

	if bestScore == 0 || bestNodeID == "" {
		return "", fmt.Errorf("target node not found")
	}
	return bestNodeID, nil
}

// GetHeatTrend GET /api/v1/lakes/{lake_id}/heat-trend
func (h *GraphAnalysisHandlers) GetHeatTrend(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	lakeID := chi.URLParam(r, "lake_id")

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	nodes, err := h.Heat.ComputeHeat(r.Context(), u, lakeID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute heat: "+err.Error())
		return
	}

	type heatNodeRes struct {
		NodeID           string  `json:"node_id"`
		Content          string  `json:"content"`
		ContentPreview   string  `json:"content_preview"`
		HeatScore        float64 `json:"heat_score"`
		EditingScore     float64 `json:"editing_score"`
		AssociationScore float64 `json:"association_score"`
		EditCount        int     `json:"edit_count"`
		EdgeCount        int     `json:"edge_count"`
		Rank             int     `json:"rank"`
	}
	res := make([]heatNodeRes, len(nodes))
	for i, n := range nodes {
		preview := n.Content
		if len(preview) > 50 {
			preview = preview[:50] + "..."
		}
		res[i] = heatNodeRes{
			NodeID:           n.NodeID,
			Content:          n.Content,
			ContentPreview:   preview,
			HeatScore:        n.HeatScore,
			EditingScore:     n.EditingScore,
			AssociationScore: n.AssociationScore,
			EditCount:        n.EditCount,
			EdgeCount:        n.EdgeCount,
			Rank:             n.Rank,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"heat_nodes":  res,
		"window_days": service.HeatWindowDays,
		"computed_at": time.Now().Format(time.RFC3339),
	})
}
