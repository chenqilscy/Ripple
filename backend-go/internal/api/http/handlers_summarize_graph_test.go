package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
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

type sgRecordingEdgeCreator struct {
	calls  []service.CreateEdgeInput
	errFor map[string]error
}

func (s *sgRecordingEdgeCreator) Create(_ context.Context, _ *domain.User, in service.CreateEdgeInput) (*domain.Edge, error) {
	s.calls = append(s.calls, in)
	if s.errFor != nil {
		if err, ok := s.errFor[in.DstNodeID]; ok {
			return nil, err
		}
	}
	return &domain.Edge{SrcNodeID: in.SrcNodeID, DstNodeID: in.DstNodeID, Kind: in.Kind}, nil
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
		NodeGetter:  getter,
		NodeCreator: &sgStubNodeCreator{node: &domain.Node{ID: "s1", LakeID: "lake-1", Content: "摘要"}},
		EdgeCreator: &sgRecordingEdgeCreator{},
		Router:      &itStubRouter{err: errors.New("llm down")},
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

func TestSummarizeGraph_LLMBlankResponse(t *testing.T) {
	nodes := []*domain.Node{
		{ID: "n1", LakeID: "lake-1", Content: "内容一"},
		{ID: "n2", LakeID: "lake-1", Content: "内容二"},
	}
	callIdx := 0
	getter := &sgStubNodeGetterMulti{nodes: nodes, idx: &callIdx}

	h := &SummarizeGraphHandlers{
		NodeGetter:  getter,
		NodeCreator: &sgStubNodeCreator{node: &domain.Node{ID: "s1", LakeID: "lake-1", Content: "摘要"}},
		EdgeCreator: &sgRecordingEdgeCreator{},
		Router:      &itStubRouter{resp: "   \n\t  "},
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
	edgeCreator := &sgRecordingEdgeCreator{}

	h := &SummarizeGraphHandlers{
		NodeGetter:  getter,
		NodeCreator: &sgStubNodeCreator{node: summaryNode},
		EdgeCreator: edgeCreator,
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

	var got summarizeGraphResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.SummaryNode.ID != "s1" || got.SummaryNode.Content != "综合摘要" {
		t.Fatalf("unexpected summary node: %+v", got.SummaryNode)
	}
	if got.SourceCount != 2 || len(got.Sources) != 2 {
		t.Fatalf("expected 2 sources, got source_count=%d sources=%+v", got.SourceCount, got.Sources)
	}
	if got.Sources[0].ID != "n1" || got.Sources[0].ContentSnippet != "内容一" || got.Sources[0].ContentLength != 3 {
		t.Fatalf("unexpected first source preview: %+v", got.Sources[0])
	}
	if got.EdgeKind != string(domain.EdgeKindSummarizes) || !got.Complete {
		t.Fatalf("expected complete summarizes response, got edge_kind=%q complete=%v", got.EdgeKind, got.Complete)
	}
	if len(got.Edges) != 2 || len(got.EdgeFailures) != 0 {
		t.Fatalf("expected 2 edges and no failures, got edges=%+v failures=%+v", got.Edges, got.EdgeFailures)
	}
	if got.Edges[0].SourceID != "s1" || got.Edges[0].TargetID != "n1" || got.Edges[0].Kind != string(domain.EdgeKindSummarizes) {
		t.Fatalf("unexpected first edge: %+v", got.Edges[0])
	}
	if got.Edges[1].SourceID != "s1" || got.Edges[1].TargetID != "n2" || got.Edges[1].Kind != string(domain.EdgeKindSummarizes) {
		t.Fatalf("unexpected second edge: %+v", got.Edges[1])
	}
	if len(edgeCreator.calls) != 2 {
		t.Fatalf("expected 2 edge create calls, got %d", len(edgeCreator.calls))
	}
}

func TestSummarizeGraph_PartialEdgeFailureIsReturned(t *testing.T) {
	nodes := []*domain.Node{
		{ID: "n1", LakeID: "lake-1", Content: "内容一"},
		{ID: "n2", LakeID: "lake-1", Content: "内容二"},
	}
	callIdx := 0
	getter := &sgStubNodeGetterMulti{nodes: nodes, idx: &callIdx}

	summaryNode := &domain.Node{ID: "s1", LakeID: "lake-1", Content: "综合摘要"}
	edgeCreator := &sgRecordingEdgeCreator{errFor: map[string]error{"n2": errors.New("neo4j down")}}

	h := &SummarizeGraphHandlers{
		NodeGetter:  getter,
		NodeCreator: &sgStubNodeCreator{node: summaryNode},
		EdgeCreator: edgeCreator,
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

	var got summarizeGraphResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Complete {
		t.Fatalf("expected incomplete response when an edge create fails")
	}
	if len(got.Edges) != 1 || got.Edges[0].TargetID != "n1" {
		t.Fatalf("expected only n1 edge, got %+v", got.Edges)
	}
	if len(got.EdgeFailures) != 1 || got.EdgeFailures[0].SourceID != "s1" || got.EdgeFailures[0].TargetID != "n2" {
		t.Fatalf("expected visible n2 edge failure, got %+v", got.EdgeFailures)
	}
	if got.EdgeFailures[0].Reason == "" {
		t.Fatalf("edge failure reason should not be empty")
	}
}

func TestSummarizeGraph_Dedup(t *testing.T) {
	// 重复 node_ids 应被去重
	node := &domain.Node{ID: "n1", LakeID: "lake-1", Content: "内容一"}
	node2 := &domain.Node{ID: "n2", LakeID: "lake-1", Content: "内容二"}
	callIdx := 0
	getter := &sgStubNodeGetterMulti{nodes: []*domain.Node{node, node2}, idx: &callIdx}

	summaryNode := &domain.Node{ID: "s1", LakeID: "lake-1", Content: "摘要"}
	edgeCreator := &sgRecordingEdgeCreator{}

	h := &SummarizeGraphHandlers{
		NodeGetter:  getter,
		NodeCreator: &sgStubNodeCreator{node: summaryNode},
		EdgeCreator: edgeCreator,
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
	if len(edgeCreator.calls) != 2 {
		t.Errorf("expected 2 edge create calls after dedup, got %d", len(edgeCreator.calls))
	}
}

func TestSummarizeGraph_DedupBeforeMaxLimit(t *testing.T) {
	node := &domain.Node{ID: "n1", LakeID: "lake-1", Content: "内容一"}
	node2 := &domain.Node{ID: "n2", LakeID: "lake-1", Content: "内容二"}
	callIdx := 0
	getter := &sgStubNodeGetterMulti{nodes: []*domain.Node{node, node2}, idx: &callIdx}

	summaryNode := &domain.Node{ID: "s1", LakeID: "lake-1", Content: "摘要"}
	edgeCreator := &sgRecordingEdgeCreator{}

	h := &SummarizeGraphHandlers{
		NodeGetter:  getter,
		NodeCreator: &sgStubNodeCreator{node: summaryNode},
		EdgeCreator: edgeCreator,
		Router:      &itStubRouter{resp: "摘要"},
	}
	body := `{"node_ids":["n1","n1","n1","n1","n1","n1","n1","n1","n1","n1","n1","n1","n1","n1","n1","n1","n1","n1","n1","n1","n2"]}`
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	r = sgWithLakeID(r, "lake-1")
	w := httptest.NewRecorder()
	h.SummarizeGraph(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	if *getter.idx != 2 {
		t.Errorf("expected 2 Get calls after dedup before max limit, got %d", *getter.idx)
	}
	if len(edgeCreator.calls) != 2 {
		t.Errorf("expected 2 edge create calls after dedup before max limit, got %d", len(edgeCreator.calls))
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
