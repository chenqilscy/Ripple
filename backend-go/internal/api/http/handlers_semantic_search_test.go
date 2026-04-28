package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/llm"
)

var ssErrFake = errors.New("fake error")

// --- stubs ---

type ssStubSearcher struct {
	results map[string][]domain.NodeSearchResult // keyword -> results
	err     error
}

func (s *ssStubSearcher) SearchNodes(_ context.Context, _ *domain.User, _, q string, limit int) ([]domain.NodeSearchResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	res := s.results[q]
	if len(res) > limit {
		res = res[:limit]
	}
	return res, nil
}

type ssStubRouter struct {
	keywords []string // 每次 Generate 返回的关键词（每行一个）
	err      error
}

func (r *ssStubRouter) Generate(_ context.Context, _ llm.GenerateRequest) ([]llm.Candidate, error) {
	if r.err != nil {
		return nil, r.err
	}
	text := ""
	for _, kw := range r.keywords {
		text += kw + "\n"
	}
	return []llm.Candidate{{Text: text}}, nil
}

func (r *ssStubRouter) GenerateStream(_ context.Context, _ llm.GenerateRequest, _ func(llm.Candidate)) error {
	return nil
}

// makeSemanticReq 构建 GET /semantic-search 请求。
func makeSemanticReq(q, lakeID, mode string) *http.Request {
	u := &url.URL{Path: "/semantic-search"}
	q2 := url.Values{}
	q2.Set("q", q)
	q2.Set("lake_id", lakeID)
	if mode != "" {
		q2.Set("mode", mode)
	}
	u.RawQuery = q2.Encode()
	req := httptest.NewRequest(http.MethodGet, u.String(), nil)
	ctx := context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u1"})
	return req.WithContext(ctx)
}

// --- 测试 ---

func TestSemanticSearch_MissingQ(t *testing.T) {
	h := &SemanticSearchHandlers{Nodes: &ssStubSearcher{}, Router: nil}
	req := httptest.NewRequest(http.MethodGet, "/semantic-search?lake_id=L1", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u1"}))
	w := httptest.NewRecorder()
	h.Search(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestSemanticSearch_MissingLakeID(t *testing.T) {
	h := &SemanticSearchHandlers{Nodes: &ssStubSearcher{}, Router: nil}
	req := httptest.NewRequest(http.MethodGet, "/semantic-search?q=test", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u1"}))
	w := httptest.NewRecorder()
	h.Search(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestSemanticSearch_FulltextFallback_NoRouter(t *testing.T) {
	stubResults := []domain.NodeSearchResult{
		{NodeID: "n1", LakeID: "L1", Snippet: "content", Score: 1.0},
	}
	searcher := &ssStubSearcher{results: map[string][]domain.NodeSearchResult{"golang": stubResults}}
	h := &SemanticSearchHandlers{Nodes: searcher, Router: nil}
	req := makeSemanticReq("golang", "L1", "semantic") // mode=semantic but no router → fallback
	w := httptest.NewRecorder()
	h.Search(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !contains(body, "n1") {
		t.Errorf("expected n1 in response body, got: %s", body)
	}
}

func TestSemanticSearch_Semantic_MergesResults(t *testing.T) {
	// 两个关键词各命中不同节点，合并后都返回
	results := map[string][]domain.NodeSearchResult{
		"golang":    {{NodeID: "n1", LakeID: "L1", Snippet: "go", Score: 2.0}},
		"编程语言": {{NodeID: "n2", LakeID: "L1", Snippet: "lang", Score: 1.5}},
		"go语言":    {{NodeID: "n1", LakeID: "L1", Snippet: "go", Score: 1.0}}, // n1 再次命中，累计分数
	}
	searcher := &ssStubSearcher{results: results}
	router := &ssStubRouter{keywords: []string{"编程语言", "go语言"}}
	h := &SemanticSearchHandlers{Nodes: searcher, Router: router}
	req := makeSemanticReq("golang", "L1", "semantic")
	w := httptest.NewRecorder()
	h.Search(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !contains(body, "n1") {
		t.Errorf("expected n1 in response, got: %s", body)
	}
	if !contains(body, "n2") {
		t.Errorf("expected n2 in response, got: %s", body)
	}
}

func TestSemanticSearch_LLMError_FallsBackToFulltext(t *testing.T) {
	stubResults := []domain.NodeSearchResult{
		{NodeID: "n3", LakeID: "L1", Snippet: "fallback", Score: 0.8},
	}
	searcher := &ssStubSearcher{results: map[string][]domain.NodeSearchResult{"query": stubResults}}
	router := &ssStubRouter{err: ssErrFake}
	h := &SemanticSearchHandlers{Nodes: searcher, Router: router}
	req := makeSemanticReq("query", "L1", "semantic")
	w := httptest.NewRecorder()
	h.Search(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !contains(body, "n3") {
		t.Errorf("expected fallback result n3, got: %s", body)
	}
}

func TestSemanticSearch_Unauthenticated(t *testing.T) {
	h := &SemanticSearchHandlers{Nodes: &ssStubSearcher{}, Router: nil}
	req := httptest.NewRequest(http.MethodGet, "/semantic-search?q=x&lake_id=L1", nil)
	// 不注入 user
	w := httptest.NewRecorder()
	h.Search(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

// contains 简单字符串子串检查（避免引入 strings 包冲突）
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && searchIn(s, substr))
}

func searchIn(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
