package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/llm"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// maxImportTextRunes 导入文本最大字符数（防止 LLM token 超限）。
const maxImportTextRunes = 4000

// maxImportNodes 每次导入最多节点数硬上限。
const maxImportNodes = 50

// ImportTextHandlers P20-A：自由文本一键转图谱处理器。
type ImportTextHandlers struct {
	Nodes  *service.NodeService
	Edges  *service.EdgeService
	Router llm.Router
}

type importTextRequest struct {
	Text     string `json:"text"`
	MaxNodes int    `json:"max_nodes"`
}

type importTextNodeResult struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

type importTextEdgeResult struct {
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	Kind     string `json:"kind"`
}

type importTextResponse struct {
	Nodes    []importTextNodeResult `json:"nodes"`
	Edges    []importTextEdgeResult `json:"edges"`
	Imported int                    `json:"imported"`
}

// llmGraphNode LLM 返回的节点结构（使用 ref 作本地引用 ID）。
type llmGraphNode struct {
	Ref     string `json:"ref"`
	Content string `json:"content"`
}

// llmGraphEdge LLM 返回的边结构（src/dst 引用节点 ref）。
type llmGraphEdge struct {
	Src  string `json:"src"`
	Dst  string `json:"dst"`
	Kind string `json:"kind"`
}

// llmGraphResult LLM 返回的完整图谱 JSON。
type llmGraphResult struct {
	Nodes []llmGraphNode `json:"nodes"`
	Edges []llmGraphEdge `json:"edges"`
}

// ImportText POST /api/v1/lakes/{id}/import/text
//
// 将纯文本通过 LLM 解析成图谱节点和边，批量写入指定湖。
// 流程：截断文本 → LLM 生成 JSON → 解析 → 批量创建节点 → 批量创建边 → 返回结果。
func (h *ImportTextHandlers) ImportText(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	lakeID := chi.URLParam(r, "id")

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // 64KB 请求体上限
	var req importTextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}

	// 服务端强制截断超长文本
	if utf8.RuneCountInString(text) > maxImportTextRunes {
		runes := []rune(text)
		text = string(runes[:maxImportTextRunes])
	}

	// max_nodes 默认 10，服务端强制上限 50
	maxNodes := req.MaxNodes
	if maxNodes <= 0 {
		maxNodes = 10
	}
	if maxNodes > maxImportNodes {
		maxNodes = maxImportNodes
	}

	// 调用 LLM 解析文本为图谱结构
	graph, err := h.callLLMForGraph(r.Context(), text, maxNodes)
	if err != nil || graph == nil || len(graph.Nodes) == 0 {
		writeError(w, http.StatusServiceUnavailable, "LLM graph extraction failed")
		return
	}

	// 创建节点，建立 ref → 实际 ID 映射
	refToID := make(map[string]string, len(graph.Nodes))
	createdNodes := make([]importTextNodeResult, 0, len(graph.Nodes))

	for _, gn := range graph.Nodes {
		content := strings.TrimSpace(gn.Content)
		if content == "" || gn.Ref == "" {
			continue
		}
		n, createErr := h.Nodes.Create(r.Context(), u, service.CreateNodeInput{
			LakeID:  lakeID,
			Content: content,
			Type:    domain.NodeTypeText,
		})
		if createErr != nil {
			// 权限错误直接返回，其他错误静默跳过
			if mapDomainError(createErr) == http.StatusForbidden {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
			continue
		}
		refToID[gn.Ref] = n.ID
		createdNodes = append(createdNodes, importTextNodeResult{
			ID:      n.ID,
			Content: content,
		})
	}

	// 创建边（失败静默跳过，不影响整体结果）
	createdEdges := make([]importTextEdgeResult, 0, len(graph.Edges))
	if h.Edges != nil {
		for _, ge := range graph.Edges {
			srcID, srcOk := refToID[ge.Src]
			dstID, dstOk := refToID[ge.Dst]
			if !srcOk || !dstOk || srcID == dstID {
				continue
			}
			kind := parseEdgeKind(ge.Kind)
			_, edgeErr := h.Edges.Create(r.Context(), u, service.CreateEdgeInput{
				SrcNodeID: srcID,
				DstNodeID: dstID,
				Kind:      kind,
			})
			if edgeErr != nil {
				continue
			}
			createdEdges = append(createdEdges, importTextEdgeResult{
				SourceID: srcID,
				TargetID: dstID,
				Kind:     string(kind),
			})
		}
	}

	writeJSON(w, http.StatusOK, importTextResponse{
		Nodes:    createdNodes,
		Edges:    createdEdges,
		Imported: len(createdNodes),
	})
}

// callLLMForGraph 调用 LLM 将文本解析为图谱结构。30s 超时。
// 安全：用户文本通过 XML 标签隔离，防止 prompt injection。
func (h *ImportTextHandlers) callLLMForGraph(ctx context.Context, text string, maxNodes int) (*llmGraphResult, error) {
	// 对文本中的 XML 特殊字符转义，防止注入攻击
	safeText := strings.NewReplacer(
		"<", "＜",
		">", "＞",
		"</", "＜/",
	).Replace(text)

	var sb strings.Builder
	sb.WriteString(`你是一个知识图谱提取助手。请从下面 <text> 标签内的文本中，提取关键概念和它们之间的关系，构建知识图谱。

要求：
1. 提取不超过 `)
	sb.WriteString(strconv.Itoa(maxNodes))
	sb.WriteString(` 个最重要的节点，每个节点用简洁的文本描述（50字以内）。
2. 每个节点分配唯一的 ref（如 n1, n2, ...）。
3. 提取节点间的关系作为边，kind 只能是以下之一：relates / derives / opposes / refines / groups。
4. 严格返回合法 JSON，不添加任何解释文字，不使用 Markdown 代码块。
5. 格式：{"nodes":[{"ref":"n1","content":"..."}],"edges":[{"src":"n1","dst":"n2","kind":"relates"}]}

<text>`)
	sb.WriteString(safeText)
	sb.WriteString(`</text>`)

	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cands, err := h.Router.Generate(ctx2, llm.GenerateRequest{
		Prompt:   sb.String(),
		N:        1,
		Modality: llm.ModalityText,
		Hints:    llm.TextHints{Temperature: 0.3, MaxTokens: 2000},
	})
	if err != nil || len(cands) == 0 {
		return nil, err
	}

	raw := strings.TrimSpace(cands[0].Text)
	// 剥除 markdown 代码块（LLM 可能忽略 "不使用" 的要求）
	raw = stripMarkdownCodeFence(raw)

	var result llmGraphResult
	if jsonErr := json.Unmarshal([]byte(raw), &result); jsonErr != nil {
		return nil, jsonErr
	}
	return &result, nil
}

// stripMarkdownCodeFence 剥除 LLM 返回中可能存在的 Markdown ```json ... ``` 代码块包装。
func stripMarkdownCodeFence(s string) string {
	// 处理 ```json\n...\n``` 或 ```\n...\n```
	for _, fence := range []string{"```json", "```"} {
		if idx := strings.Index(s, fence); idx != -1 {
			s = s[idx+len(fence):]
			if end := strings.LastIndex(s, "```"); end != -1 {
				s = s[:end]
			}
			return strings.TrimSpace(s)
		}
	}
	return s
}

// parseEdgeKind 将字符串映射到 domain.EdgeKind，未知类型降级为 relates。
func parseEdgeKind(k string) domain.EdgeKind {
	k = strings.ToLower(strings.TrimSpace(k))
	switch k {
	case "derives":
		return domain.EdgeKindDerives
	case "opposes":
		return domain.EdgeKindOpposes
	case "refines":
		return domain.EdgeKindRefines
	case "groups":
		return domain.EdgeKindGroups
	default:
		return domain.EdgeKindRelates
	}
}
