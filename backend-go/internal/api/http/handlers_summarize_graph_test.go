package httpapi

import (
"bytes"
"context"
"errors"
"net/http"
"net/http/httptest"
"testing"

"github.com/chenqilscy/ripple/backend-go/internal/domain"
"github.com/chenqilscy/ripple/backend-go/internal/service"
)

// ── stub implementations ───────────────────────────────────────────────────

type sgStubNodeGetter struct {
node *domain.Node
err  error
}

func (s *sgStubNodeGetter) Get(_ context.Context, _ *domain.User, _ string) (*domain.Node, error) {
return s.node, s.err
}

type sgStubNodeCreator struct {
node *domain.Node
err  error
}

func (s *sgStubNodeCreator) Create(_ context.Context, _ *domain.User, _ service.CreateNodeInput) (*domain.Node, error) {
return s.node, s.err
}

type sgStubEdgeCreator struct {
edge *domain.Edge
err  error
}

func (s *sgStubEdgeCreator) Create(_ context.Context, _ *domain.User, _ service.CreateEdgeInput) (*domain.Edge, error) {
return s.edge, s.err
}

// ── helper ─────────────────────────────────────────────────────────────────

func sgWithLakeID(r *http.Request, lakeID string) *http.Request {
return itWithLakeID(r, lakeID) // reuse helper from handlers_import_text_test.go
}

// ── tests ──────────────────────────────────────────────────────────────────

func TestSummarizeGraph_TooFewNodes(t *testing.T) {
h := &SummarizeGraphHandlers{Router: &itStubRouter{resp: "摘要内容"}}
body := `{"node_ids":["n1"]}`
r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
r = sgWithLakeID(r, "lake-1")
w := httptest.NewRecorder()
h.SummarizeGraph(w, r)
if w.Code != http.StatusBadRequest {
t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
}
}

func TestSummarizeGraph_InvalidJSON(t *testing.T) {
h := &SummarizeGraphHandlers{Router: &itStubRouter{resp: "摘要内容"}}
r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("notjson"))
r = sgWithLakeID(r, "lake-1")
w := httptest.NewRecorder()
h.SummarizeGraph(w, r)
if w.Code != http.StatusBadRequest {
t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
}
}

func TestSummarizeGraph_NodeNotFound(t *testing.T) {
h := &SummarizeGraphHandlers{
NodeGetter: &sgStubNodeGetter{err: errors.New("not found")},
Router:     &itStubRouter{resp: "摘要内容"},
}
body := `{"node_ids":["n1","n2"]}`
r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
r = sgWithLakeID(r, "lake-1")
w := httptest.NewRecorder()
h.SummarizeGraph(w, r)
if w.Code != http.StatusBadRequest {
t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
}
}

func TestSummarizeGraph_NodeWrongLake(t *testing.T) {
h := &SummarizeGraphHandlers{
NodeGetter: &sgStubNodeGetter{node: &domain.Node{ID: "n1", LakeID: "other-lake"}},
Router:     &itStubRouter{resp: "摘要内容"},
}
body := `{"node_ids":["n1","n2"]}`
r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
r = sgWithLakeID(r, "lake-1")
w := httptest.NewRecorder()
h.SummarizeGraph(w, r)
if w.Code != http.StatusBadRequest {
t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
}
}

func TestSummarizeGraph_LLMError(t *testing.T) {
nodes := []*domain.Node{
{ID: "n1", LakeID: "lake-1", Content: "内容一"},
{ID: "n2", LakeID: "lake-1", Content: "内容二"},
}
callIdx := 0
getter := &sgStubNodeGetterMulti{nodes: nodes, idx: &callIdx}

h := &SummarizeGraphHandlers{
NodeGetter: getter,
Router:     &itStubRouter{err: errors.New("llm down")},
}
body := `{"node_ids":["n1","n2"]}`
r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
r = sgWithLakeID(r, "lake-1")
w := httptest.NewRecorder()
h.SummarizeGraph(w, r)
if w.Code != http.StatusServiceUnavailable {
t.Fatalf("expected 503, got %d body=%s", w.Code, w.Body.String())
}
}

func TestSummarizeGraph_Success(t *testing.T) {
nodes := []*domain.Node{
{ID: "n1", LakeID: "lake-1", Content: "内容一"},
{ID: "n2", LakeID: "lake-1", Content: "内容二"},
}
callIdx := 0
getter := &sgStubNodeGetterMulti{nodes: nodes, idx: &callIdx}

summaryNode := &domain.Node{ID: "s1", LakeID: "lake-1", Content: "综合摘要"}
createdEdge := &domain.Edge{SrcNodeID: "s1", DstNodeID: "n1", Kind: domain.EdgeKindSummarizes}

h := &SummarizeGraphHandlers{
NodeGetter:  getter,
NodeCreator: &sgStubNodeCreator{node: summaryNode},
EdgeCreator: &sgStubEdgeCreator{edge: createdEdge},
Router:      &itStubRouter{resp: "综合摘要"},
}
body := `{"node_ids":["n1","n2"]}`
r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
r = sgWithLakeID(r, "lake-1")
w := httptest.NewRecorder()
h.SummarizeGraph(w, r)
if w.Code != http.StatusCreated {
t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
}
}

func TestSummarizeGraph_Dedup(t *testing.T) {
// 重复 node_ids 应被去重
node := &domain.Node{ID: "n1", LakeID: "lake-1", Content: "内容一"}
node2 := &domain.Node{ID: "n2", LakeID: "lake-1", Content: "内容二"}
callIdx := 0
getter := &sgStubNodeGetterMulti{nodes: []*domain.Node{node, node2}, idx: &callIdx}

summaryNode := &domain.Node{ID: "s1", LakeID: "lake-1", Content: "摘要"}
createdEdge := &domain.Edge{SrcNodeID: "s1", DstNodeID: "n1", Kind: domain.EdgeKindSummarizes}

h := &SummarizeGraphHandlers{
NodeGetter:  getter,
NodeCreator: &sgStubNodeCreator{node: summaryNode},
EdgeCreator: &sgStubEdgeCreator{edge: createdEdge},
Router:      &itStubRouter{resp: "摘要"},
}
// 含重复 n1
body := `{"node_ids":["n1","n1","n2"]}`
r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
r = sgWithLakeID(r, "lake-1")
w := httptest.NewRecorder()
h.SummarizeGraph(w, r)
if w.Code != http.StatusCreated {
t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
}
// 验证去重后只查了 2 次 Get
if *getter.idx != 2 {
t.Errorf("expected 2 Get calls after dedup, got %d", *getter.idx)
}
}

// ── multi-node getter helper ───────────────────────────────────────────────

type sgStubNodeGetterMulti struct {
nodes []*domain.Node
idx   *int
}

func (s *sgStubNodeGetterMulti) Get(_ context.Context, _ *domain.User, _ string) (*domain.Node, error) {
if *s.idx >= len(s.nodes) {
return nil, errors.New("out of range")
}
n := s.nodes[*s.idx]
*s.idx++
return n, nil
}
