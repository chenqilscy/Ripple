package httpapi

import (
"context"
"encoding/json"
"fmt"
"net/http"
"strings"
"time"

"github.com/chenqilscy/ripple/backend-go/internal/domain"
"github.com/chenqilscy/ripple/backend-go/internal/llm"
"github.com/chenqilscy/ripple/backend-go/internal/service"
"github.com/go-chi/chi/v5"
)

const maxSummarizeNodes = 20
const minSummarizeNodes = 2
const maxSummaryContentRunes = 500

type sgNodeGetter interface {
Get(ctx context.Context, actor *domain.User, nodeID string) (*domain.Node, error)
}

type sgNodeCreator interface {
Create(ctx context.Context, actor *domain.User, in service.CreateNodeInput) (*domain.Node, error)
}

type sgEdgeCreator interface {
Create(ctx context.Context, actor *domain.User, in service.CreateEdgeInput) (*domain.Edge, error)
}

type SummarizeGraphHandlers struct {
NodeGetter  sgNodeGetter
NodeCreator sgNodeCreator
EdgeCreator sgEdgeCreator
Router      llm.Router
}

type summarizeGraphRequest struct {
NodeIDs   []string `json:"node_ids"`
TitleHint string   `json:"title_hint"`
}

type summarizeEdgeResult struct {
SourceID string `json:"source_id"`
TargetID string `json:"target_id"`
Kind     string `json:"kind"`
}

type summarizeNodeResult struct {
ID      string `json:"id"`
Content string `json:"content"`
}

type summarizeGraphResponse struct {
SummaryNode summarizeNodeResult   `json:"summary_node"`
Edges       []summarizeEdgeResult `json:"edges"`
SourceCount int                   `json:"source_count"`
}

func (h *SummarizeGraphHandlers) SummarizeGraph(w http.ResponseWriter, r *http.Request) {
lakeID := chi.URLParam(r, "id")
u, ok := CurrentUser(r.Context())
if !ok {
writeError(w, http.StatusUnauthorized, "unauthorized")
return
}

var req summarizeGraphRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
writeError(w, http.StatusBadRequest, "invalid request body")
return
}

if len(req.NodeIDs) < minSummarizeNodes {
writeError(w, http.StatusBadRequest, fmt.Sprintf("node_ids requires at least %d items", minSummarizeNodes))
return
}
if len(req.NodeIDs) > maxSummarizeNodes {
req.NodeIDs = req.NodeIDs[:maxSummarizeNodes]
}

// 去重（dedup 后再做 min 检查，防止全重复绕过下限）
seen := make(map[string]struct{}, len(req.NodeIDs))
deduped := make([]string, 0, len(req.NodeIDs))
for _, id := range req.NodeIDs {
if _, dup := seen[id]; !dup {
seen[id] = struct{}{}
deduped = append(deduped, id)
}
}
req.NodeIDs = deduped
if len(req.NodeIDs) < minSummarizeNodes {
writeError(w, http.StatusBadRequest, fmt.Sprintf("node_ids requires at least %d unique items", minSummarizeNodes))
return
}

nodes := make([]*domain.Node, 0, len(req.NodeIDs))
for _, nid := range req.NodeIDs {
n, err := h.NodeGetter.Get(r.Context(), u, nid)
if err != nil {
writeError(w, http.StatusBadRequest, "node not found or access denied")
return
}
if n.LakeID != lakeID {
writeError(w, http.StatusBadRequest, "node does not belong to this lake")
return
}
nodes = append(nodes, n)
}

summary, err := h.callLLMForSummary(r.Context(), nodes, req.TitleHint)
if err != nil {
writeError(w, http.StatusServiceUnavailable, "LLM summarization failed")
return
}

if runes := []rune(summary); len(runes) > maxSummaryContentRunes {
summary = string(runes[:maxSummaryContentRunes])
}

summaryNode, err := h.NodeCreator.Create(r.Context(), u, service.CreateNodeInput{
LakeID:  lakeID,
Content: summary,
Type:    domain.NodeTypeText,
})
if err != nil {
writeError(w, http.StatusInternalServerError, "failed to create summary node")
return
}

edges := make([]summarizeEdgeResult, 0, len(nodes))
for _, srcNode := range nodes {
e, edgeErr := h.EdgeCreator.Create(r.Context(), u, service.CreateEdgeInput{
SrcNodeID: summaryNode.ID,
DstNodeID: srcNode.ID,
Kind:      domain.EdgeKindSummarizes,
})
if edgeErr != nil {
continue
}
edges = append(edges, summarizeEdgeResult{
SourceID: e.SrcNodeID,
TargetID: e.DstNodeID,
Kind:     string(e.Kind),
})
}

writeJSON(w, http.StatusCreated, summarizeGraphResponse{
SummaryNode: summarizeNodeResult{
ID:      summaryNode.ID,
Content: summaryNode.Content,
},
Edges:       edges,
SourceCount: len(nodes),
})
}

func (h *SummarizeGraphHandlers) callLLMForSummary(ctx context.Context, nodes []*domain.Node, titleHint string) (string, error) {
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

var sb strings.Builder
for i, n := range nodes {
content := n.Content
content = strings.ReplaceAll(content, "<", "＜")
content = strings.ReplaceAll(content, ">", "＞")
runes := []rune(content)
if len(runes) > 500 {
content = string(runes[:500]) + "..."
}
sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, content))
}

hint := ""
if titleHint != "" {
if hr := []rune(titleHint); len(hr) > 200 {
titleHint = string(hr[:200])
}
hint = "\n方向提示（可选）：" + strings.ReplaceAll(strings.ReplaceAll(titleHint, "<", "＜"), ">", "＞")
}

prompt := "你是一个知识图谱助手。请对以下节点内容进行归纳，生成一段简洁的摘要（不超过 200 字）。" +
hint + "\n\n节点列表：\n" + sb.String() +
"只返回摘要正文，不要加标题、编号或任何前缀后缀。"

cands, err := h.Router.Generate(ctx, llm.GenerateRequest{
Prompt:   prompt,
N:        1,
Modality: llm.ModalityText,
Hints:    llm.TextHints{Temperature: 0.5, MaxTokens: 400},
})
if err != nil {
return "", err
}
if len(cands) == 0 || cands[0].Text == "" {
return "", fmt.Errorf("LLM returned empty response")
}
return strings.TrimSpace(cands[0].Text), nil
}
