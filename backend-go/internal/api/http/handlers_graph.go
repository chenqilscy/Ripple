package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// GraphAnalysisHandlers 图谱分析端点（推荐/路径/聚类/规划）
type GraphAnalysisHandlers struct {
	Nodes      *service.NodeService
	Edges      *service.EdgeService
	Recommender *service.RecommenderService
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
	recID := chi.URLParam(r, "id")
	// TODO: 将推荐状态持久化为 accepted，后续 GetRecommendations 过滤已接受推荐
	writeJSON(w, http.StatusOK, map[string]any{"id": recID, "status": "accepted"})
}

// RejectRecommendation POST /api/v1/lakes/{lake_id}/recommendations/{id}/reject
func (h *GraphAnalysisHandlers) RejectRecommendation(w http.ResponseWriter, r *http.Request) {
	recID := chi.URLParam(r, "id")
	// TODO: 将推荐状态持久化为 rejected，后续 GetRecommendations 过滤已拒绝推荐
	writeJSON(w, http.StatusOK, map[string]any{"id": recID, "status": "rejected"})
}

// IgnoreRecommendation POST /api/v1/lakes/{lake_id}/recommendations/{id}/ignore
func (h *GraphAnalysisHandlers) IgnoreRecommendation(w http.ResponseWriter, r *http.Request) {
	recID := chi.URLParam(r, "id")
	// TODO: 将推荐状态持久化为 ignored，后续 GetRecommendations 过滤已忽略推荐
	writeJSON(w, http.StatusOK, map[string]any{"id": recID, "status": "ignored"})
}

// AcceptPlanning POST /api/v1/planning/{id}/accept
func (h *GraphAnalysisHandlers) AcceptPlanning(w http.ResponseWriter, r *http.Request) {
	suggestionID := chi.URLParam(r, "id")
	// TODO: 根据 suggestion type 执行对应操作（如创建关联节点）
	writeJSON(w, http.StatusOK, map[string]any{"id": suggestionID, "status": "accepted"})
}
