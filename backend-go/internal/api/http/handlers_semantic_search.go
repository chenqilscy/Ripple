package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/llm"
)

// ssNodeSearcher 节点搜索接口（语义搜索依赖）。
type ssNodeSearcher interface {
	SearchNodes(ctx context.Context, actor *domain.User, lakeID, q string, limit int) ([]domain.NodeSearchResult, error)
}

// SemanticSearchHandlers 语义搜索处理器（P20-C）。
// 当 LLM Router 不可用时自动降级为全文搜索。
type SemanticSearchHandlers struct {
	Nodes  ssNodeSearcher
	Router llm.Router // 可为 nil，nil 时仅全文搜索
}

const ssMaxKeywords = 5
const ssLLMTimeout = 3 * time.Second
const ssMaxTopK = 20
const ssDefaultTopK = 8

// Search GET /api/v1/search?q=TEXT&lake_id=UUID&mode=semantic|fulltext&limit=N
// 兼容现有全文搜索接口，新增 mode=semantic 时走 LLM 关键词扩展路径。
func (h *SemanticSearchHandlers) Search(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
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

	limit := ssDefaultTopK
	if ls := r.URL.Query().Get("limit"); ls != "" {
		if n, err := strconv.Atoi(ls); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}
	if limit > ssMaxTopK {
		limit = ssMaxTopK
	}

	mode := r.URL.Query().Get("mode")
	if mode == "semantic" && h.Router != nil {
		results := h.semanticSearch(r.Context(), u, lakeID, q, limit)
		writeSearchResults(w, results)
		return
	}

	// 默认：全文搜索（兼容旧行为）
	results, err := h.Nodes.SearchNodes(r.Context(), u, lakeID, q, limit)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeSearchResults(w, results)
}

// semanticSearch LLM 关键词扩展 + 多轮全文搜索，错误时降级为全文搜索。
func (h *SemanticSearchHandlers) semanticSearch(
	ctx context.Context,
	actor *domain.User,
	lakeID, q string,
	limit int,
) []domain.NodeSearchResult {
	keywords := h.expandQueryWithLLM(ctx, q)
	if len(keywords) == 0 {
		// LLM 降级：直接用原始 query 全文搜索
		results, _ := h.Nodes.SearchNodes(ctx, actor, lakeID, q, limit)
		return results
	}

	// 对每个关键词搜索，合并去重，按累计分数排序
	seen := make(map[string]*domain.NodeSearchResult)
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		hits, err := h.Nodes.SearchNodes(ctx, actor, lakeID, kw, limit)
		if err != nil {
			continue
		}
		for i := range hits {
			hit := &hits[i]
			if existing, ok := seen[hit.NodeID]; ok {
				existing.Score += hit.Score
			} else {
				cp := *hit
				seen[hit.NodeID] = &cp
			}
		}
	}

	if len(seen) == 0 {
		// 所有关键词均无命中，再试原始 query 全文兜底
		results, _ := h.Nodes.SearchNodes(ctx, actor, lakeID, q, limit)
		return results
	}

	merged := make([]domain.NodeSearchResult, 0, len(seen))
	for _, v := range seen {
		merged = append(merged, *v)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})
	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged
}

// expandQueryWithLLM 调用 LLM 将用户查询扩展为关键词列表，3s 超时。
// 返回空列表表示降级。
func (h *SemanticSearchHandlers) expandQueryWithLLM(ctx context.Context, q string) []string {
	llmCtx, cancel := context.WithTimeout(ctx, ssLLMTimeout)
	defer cancel()

	prompt := fmt.Sprintf(
		"你是一个搜索关键词扩展助手。用户的查询是：「%s」\n"+
			"请给出最多 %d 个中文搜索关键词，每行一个，不要编号和解释，仅输出关键词。",
		q, ssMaxKeywords,
	)
	cands, err := h.Router.Generate(llmCtx, llm.GenerateRequest{
		Prompt:   prompt,
		N:        1,
		Modality: llm.ModalityText,
		Hints: llm.TextHints{
			Temperature: 0.3,
			MaxTokens:   80,
		},
	})
	if err != nil || len(cands) == 0 || cands[0].Text == "" {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(cands[0].Text), "\n")
	keywords := make([]string, 0, ssMaxKeywords+1)
	// 始终包含原始查询
	keywords = append(keywords, q)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == q {
			continue
		}
		keywords = append(keywords, line)
		if len(keywords) >= ssMaxKeywords+1 {
			break
		}
	}
	return keywords
}

// writeSearchResults 统一输出搜索结果 JSON。
func writeSearchResults(w http.ResponseWriter, results []domain.NodeSearchResult) {
	type searchHit struct {
		NodeID  string  `json:"node_id"`
		LakeID  string  `json:"lake_id"`
		Snippet string  `json:"snippet"`
		Score   float64 `json:"score"`
	}
	out := make([]searchHit, 0, len(results))
	for _, r := range results {
		out = append(out, searchHit{
			NodeID:  r.NodeID,
			LakeID:  r.LakeID,
			Snippet: r.Snippet,
			Score:   r.Score,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out})
}
