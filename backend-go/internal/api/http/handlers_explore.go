package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/chenqilscy/ripple/backend-go/internal/llm"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// ExploreHandlers P19-A：AI 图谱探索处理器。
type ExploreHandlers struct {
	Nodes  *service.NodeService
	Router llm.Router // 可选；nil 时跳过 LLM 摘要
}

type exploreRequest struct {
	Query    string `json:"query"`
	MaxNodes int    `json:"max_nodes"`
}

type exploreNodeResult struct {
	NodeID  string  `json:"node_id"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

type exploreResponse struct {
	RelevantNodes []exploreNodeResult `json:"relevant_nodes"`
	Summary       string              `json:"summary"`
}

// Explore POST /api/v1/lakes/{id}/explore
//
// 算法：
//  1. ListByLake 取出全部非 VAPOR 节点（权限检查已在 service 层）
//  2. TF 关键词打分，取 top-k
//  3. 可选：单次 LLM 调用生成摘要（< 30s 超时）
//  4. 返回相关节点列表 + 摘要文本
func (h *ExploreHandlers) Explore(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	lakeID := chi.URLParam(r, "id")

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req exploreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}
	if len([]rune(query)) < 2 {
		writeError(w, http.StatusBadRequest, "query too short (min 2 chars)")
		return
	}
	if len([]rune(query)) > 500 {
		writeError(w, http.StatusBadRequest, "query too long (max 500 chars)")
		return
	}

	maxNodes := req.MaxNodes
	if maxNodes <= 0 || maxNodes > 50 {
		maxNodes = 20
	}

	// 1. 取湖内所有节点（不含 VAPOR），权限由 service 层校验
	nodes, err := h.Nodes.ListByLake(r.Context(), u, lakeID, false)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}

	// 软性限制：节点数量超过 500 时只取前 500，避免大图谱打分耗时过长。
	const exploreNodeCap = 500
	if len(nodes) > exploreNodeCap {
		nodes = nodes[:exploreNodeCap]
	}

	if len(nodes) == 0 {
		writeJSON(w, http.StatusOK, exploreResponse{
			RelevantNodes: []exploreNodeResult{},
			Summary:       "该图谱暂无节点。",
		})
		return
	}

	// 2. TF 关键词打分
	queryTokens := exploreTokenize(query)
	// query 全为特殊字符时 tokens 为空，直接返回空结果
	if len(queryTokens) == 0 {
		writeJSON(w, http.StatusOK, exploreResponse{
			RelevantNodes: []exploreNodeResult{},
			Summary:       "",
		})
		return
	}
	type scored struct {
		id      string
		content string
		score   float64
	}
	results := make([]scored, 0, len(nodes))
	for i := range nodes {
		s := exploreTFScore(nodes[i].Content, queryTokens)
		if s > 0 {
			results = append(results, scored{
				id:      nodes[i].ID,
				content: nodes[i].Content,
				score:   s,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})
	if len(results) > maxNodes {
		results = results[:maxNodes]
	}

	relevant := make([]exploreNodeResult, 0, len(results))
	for _, rs := range results {
		relevant = append(relevant, exploreNodeResult{
			NodeID:  rs.id,
			Content: exploreTruncate(rs.content, 200),
			Score:   rs.score,
		})
	}

	// 3. LLM 摘要（可选）
	summary := ""
	if h.Router != nil && len(relevant) > 0 {
		summary = h.generateExploreSummary(r.Context(), query, relevant)
	}

	writeJSON(w, http.StatusOK, exploreResponse{
		RelevantNodes: relevant,
		Summary:       summary,
	})
}

// generateExploreSummary 用一次 LLM 调用生成探索摘要。失败静默降级（返回空串）。
// 安全：用 XML 风格分隔符将用户 query 与节点内容隔离，缓解 prompt injection。
func (h *ExploreHandlers) generateExploreSummary(
	ctx context.Context,
	query string,
	relevant []exploreNodeResult,
) string {
	var sb strings.Builder
	// 系统指令在最前，用结构化标签包裹用户输入，防止注入攻击。
	sb.WriteString("你是一个图谱内容分析助手。请根据以下节点内容，用 2-3 句话概括核心主题和相互关系，并给出进一步探索的建议。只输出摘要本身，不加额外说明。\n\n")
	sb.WriteString("<nodes>\n")
	limit := len(relevant)
	if limit > 10 {
		limit = 10
	}
	for i := 0; i < limit; i++ {
		sb.WriteString("- ")
		sb.WriteString(relevant[i].Content)
		sb.WriteString("\n")
	}
	sb.WriteString("</nodes>\n\n")
	// 用户 query 放在最后并用标签包裹，使模型将其视为参考而非新指令。
	sb.WriteString("<query>")
	sb.WriteString(strings.NewReplacer("<", "＜", ">", "＞").Replace(query))
	sb.WriteString("</query>")

	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cands, err := h.Router.Generate(ctx2, llm.GenerateRequest{
		Prompt:   sb.String(),
		N:        1,
		Modality: llm.ModalityText,
		Hints:    llm.TextHints{Temperature: 0.5, MaxTokens: 300},
	})
	if err != nil || len(cands) == 0 {
		return ""
	}
	return strings.TrimSpace(cands[0].Text)
}

// exploreTokenize 将文本按非字母/数字边界切词（支持中英混排）。
func exploreTokenize(text string) []string {
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsDigit(c)
	}
	parts := strings.FieldsFunc(text, f)
	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(p)
		if p != "" {
			tokens = append(tokens, p)
		}
	}
	return tokens
}

// exploreTFScore 计算 query token 在节点内容中的词频归一化得分。
func exploreTFScore(content string, queryTokens []string) float64 {
	if len(queryTokens) == 0 || content == "" {
		return 0
	}
	contentLower := strings.ToLower(content)
	var score float64
	for _, qt := range queryTokens {
		score += float64(strings.Count(contentLower, qt))
	}
	if score == 0 {
		return 0
	}
	// 按内容长度归一化（rune 数），避免长节点天然占优
	runeLen := float64(len([]rune(content)))
	if runeLen > 0 {
		score = score / runeLen * 1000
	}
	return score
}

// exploreTruncate 截断文本到 maxRunes 个字符。
func exploreTruncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}
