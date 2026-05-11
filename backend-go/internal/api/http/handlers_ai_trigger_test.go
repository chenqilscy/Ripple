package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/go-chi/chi/v5"
)

type aiTriggerNodeGetterStub struct {
	nodes map[string]*domain.Node
}

func (s *aiTriggerNodeGetterStub) Get(_ context.Context, _ *domain.User, nodeID string) (*domain.Node, error) {
	n, ok := s.nodes[nodeID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *n
	return &cp, nil
}

type aiTriggerJobsStub struct {
	created            *domain.AiJob
	createErr          error
	conflict           bool
	byNode             *domain.AiJob
	getErr             error
	getByIdempErr      error
	idempKeyNodeID     string
	setIdempErr        error
}

func (s *aiTriggerJobsStub) CreateWithConflictCheck(_ context.Context, job domain.AiJob) (*domain.AiJob, bool, error) {
	if s.createErr != nil {
		return nil, false, s.createErr
	}
	if s.conflict {
		return nil, false, nil
	}
	s.created = &job
	return &job, true, nil
}

func (s *aiTriggerJobsStub) GetByNodeID(_ context.Context, _ string) (*domain.AiJob, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.byNode == nil {
		return nil, domain.ErrNotFound
	}
	cp := *s.byNode
	return &cp, nil
}

func (s *aiTriggerJobsStub) GetByID(_ context.Context, _ string) (*domain.AiJob, error) {
	return nil, domain.ErrNotFound
}

func (s *aiTriggerJobsStub) ListPending(_ context.Context, _ int) ([]domain.AiJob, error) {
	return nil, nil
}

func (s *aiTriggerJobsStub) UpdateStatus(_ context.Context, _ string, _ domain.AiJobStatus, _ int, _ string) error {
	return nil
}

func (s *aiTriggerJobsStub) RecoverProcessing(_ context.Context) (int64, error) {
	return 0, nil
}

func (s *aiTriggerJobsStub) GetByIdempotencyKey(_ context.Context, _, _ string) (string, error) {
	if s.getByIdempErr != nil {
		return "", s.getByIdempErr
	}
	return s.idempKeyNodeID, nil
}

func (s *aiTriggerJobsStub) SetIdempotencyKey(_ context.Context, _, _, _ string) error {
	return s.setIdempErr
}

func aiTriggerRequest(method, body, lakeID, nodeID string) *http.Request {
	req := httptest.NewRequest(method, "/", bytes.NewBufferString(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("lake_id", lakeID)
	rctx.URLParams.Add("node_id", nodeID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, ctxUserKey, &domain.User{ID: "u-test"})
	return req.WithContext(ctx)
}

func TestAiTriggerRejectsTargetNodeOutsideLake(t *testing.T) {
	jobs := &aiTriggerJobsStub{}
	h := &AiTriggerHandlers{
		Jobs: jobs,
		Nodes: &aiTriggerNodeGetterStub{nodes: map[string]*domain.Node{
			"node-1": {ID: "node-1", LakeID: "lake-other", OwnerID: "u-test", Content: "x", Type: domain.NodeTypeText},
		}},
	}

	w := httptest.NewRecorder()
	h.Trigger(w, aiTriggerRequest(http.MethodPost, `{}`, "lake-1", "node-1"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
	if jobs.created != nil {
		t.Fatalf("job should not be created for cross-lake target node")
	}
}

func TestAiTriggerRejectsInputNodeOutsideLake(t *testing.T) {
	jobs := &aiTriggerJobsStub{}
	h := &AiTriggerHandlers{
		Jobs: jobs,
		Nodes: &aiTriggerNodeGetterStub{nodes: map[string]*domain.Node{
			"node-1": {ID: "node-1", LakeID: "lake-1", OwnerID: "u-test", Content: "target", Type: domain.NodeTypeText},
			"ctx-1":  {ID: "ctx-1", LakeID: "lake-other", OwnerID: "u-test", Content: "secret", Type: domain.NodeTypeText},
		}},
	}

	w := httptest.NewRecorder()
	h.Trigger(w, aiTriggerRequest(http.MethodPost, `{"input_node_ids":["ctx-1"]}`, "lake-1", "node-1"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
	if jobs.created != nil {
		t.Fatalf("job should not be created with cross-lake input node")
	}
}

func TestAiTriggerSanitizesInputNodesAndResponse(t *testing.T) {
	jobs := &aiTriggerJobsStub{}
	h := &AiTriggerHandlers{
		Jobs: jobs,
		Nodes: &aiTriggerNodeGetterStub{nodes: map[string]*domain.Node{
			"node-1": {ID: "node-1", LakeID: "lake-1", OwnerID: "u-test", Content: "target", Type: domain.NodeTypeText},
			"ctx-1":  {ID: "ctx-1", LakeID: "lake-1", OwnerID: "u-test", Content: "context", Type: domain.NodeTypeText},
		}},
	}

	w := httptest.NewRecorder()
	body := `{"input_node_ids":[" ctx-1 ","ctx-1", ""],"override_vars":{" custom_key ":"custom-value", " ":"drop"}}`
	h.Trigger(w, aiTriggerRequest(http.MethodPost, body, "lake-1", "node-1"))
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", w.Code, w.Body.String())
	}
	if jobs.created == nil {
		t.Fatalf("expected job to be created")
	}
	if !reflect.DeepEqual(jobs.created.InputNodeIDs, []string{"ctx-1"}) {
		t.Fatalf("unexpected input node ids: %#v", jobs.created.InputNodeIDs)
	}
	if got := jobs.created.OverrideVars["custom_key"]; got != "custom-value" {
		t.Fatalf("expected normalized override var, got %q", got)
	}
	if _, ok := jobs.created.OverrideVars[""]; ok {
		t.Fatalf("empty override var key should be dropped")
	}

	var resp aiTriggerResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.JobID == "" || resp.AIJobID != resp.JobID || resp.NodeID != "node-1" || resp.EstimatedSeconds != aiTriggerEstimatedSeconds {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestAiStatusDoesNotLeakCrossLakeJob(t *testing.T) {
	h := &AiTriggerHandlers{
		Jobs: &aiTriggerJobsStub{byNode: &domain.AiJob{ID: "job-1", NodeID: "node-1", LakeID: "lake-secret", Status: domain.AiJobProcessing}},
	}

	w := httptest.NewRecorder()
	h.Status(w, aiTriggerRequest(http.MethodGet, ``, "lake-public", "node-1"))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAiStatusSanitizesInternalError(t *testing.T) {
	h := &AiTriggerHandlers{
		Jobs: &aiTriggerJobsStub{byNode: &domain.AiJob{ID: "job-1", NodeID: "node-1", LakeID: "lake-1", Status: domain.AiJobFailed, Error: "pq: password authentication failed for host fn.cky:15432"}},
	}

	w := httptest.NewRecorder()
	h.Status(w, aiTriggerRequest(http.MethodGet, ``, "lake-1", "node-1"))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp aiStatusResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != safeAIJobErrorText {
		t.Fatalf("expected sanitized error %q, got %q", safeAIJobErrorText, resp.Error)
	}
	if strings.Contains(resp.Error, "pq:") || strings.Contains(resp.Error, "fn.cky") {
		t.Fatalf("status error leaked internal detail: %q", resp.Error)
	}
}

func TestAiTriggerIdempotencyReturnsConflict(t *testing.T) {
	jobs := &aiTriggerJobsStub{
		idempKeyNodeID: "node-existing",
	}
	h := &AiTriggerHandlers{
		Jobs: jobs,
		Nodes: &aiTriggerNodeGetterStub{nodes: map[string]*domain.Node{
			"node-1": {ID: "node-1", LakeID: "lake-1", OwnerID: "u-test", Content: "target", Type: domain.NodeTypeText},
		}},
	}

	w := httptest.NewRecorder()
	body := `{"idempotency_key":"my-request-123"}`
	h.Trigger(w, aiTriggerRequest(http.MethodPost, body, "lake-1", "node-1"))
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", w.Code, w.Body.String())
	}
	if jobs.created != nil {
		t.Fatalf("job should not be created for idempotent request")
	}
	var resp aiTriggerResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.NodeID != "node-existing" {
		t.Fatalf("expected NodeID=node-existing, got %q", resp.NodeID)
	}
	if resp.Status != "idempotent_repeat" {
		t.Fatalf("expected status=idempotent_repeat, got %q", resp.Status)
	}
}

func TestAiTriggerWithoutIdempotencyKeySucceeds(t *testing.T) {
	jobs := &aiTriggerJobsStub{}
	h := &AiTriggerHandlers{
		Jobs: jobs,
		Nodes: &aiTriggerNodeGetterStub{nodes: map[string]*domain.Node{
			"node-1": {ID: "node-1", LakeID: "lake-1", OwnerID: "u-test", Content: "target", Type: domain.NodeTypeText},
		}},
	}

	w := httptest.NewRecorder()
	body := `{"idempotency_key":""}`
	h.Trigger(w, aiTriggerRequest(http.MethodPost, body, "lake-1", "node-1"))
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", w.Code, w.Body.String())
	}
	if jobs.created == nil {
		t.Fatalf("job should be created when no idempotency key")
	}
}

func TestAiTriggerIdempotencyKeySavedAfterJobCreation(t *testing.T) {
	jobs := &aiTriggerJobsStub{}
	h := &AiTriggerHandlers{
		Jobs: jobs,
		Nodes: &aiTriggerNodeGetterStub{nodes: map[string]*domain.Node{
			"node-1": {ID: "node-1", LakeID: "lake-1", OwnerID: "u-test", Content: "target", Type: domain.NodeTypeText},
		}},
	}

	w := httptest.NewRecorder()
	body := `{"idempotency_key":"save-key-456"}`
	h.Trigger(w, aiTriggerRequest(http.MethodPost, body, "lake-1", "node-1"))
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", w.Code, w.Body.String())
	}
	if jobs.created == nil {
		t.Fatalf("job should be created")
	}
}

// ---------------------------------------------------------------------------
// Rate Limiter Tests
// ---------------------------------------------------------------------------

// mockHandler 用于测试中间件的最小 handler。
type mockHandler struct{ called int }

func (m *mockHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	m.called++
	w.WriteHeader(http.StatusOK)
}

// createTestRateLimiter 创建一个独立的测试限流器（避免污染全局 aiTriggerLimiter）。
func createTestRateLimiter(rps float64, burst int) *userRateLimiter {
	return newUserRateLimiter(rps, burst)
}

func TestUserRateLimiterRespectsBurst(t *testing.T) {
	// burst=5 意味着最多 5 个请求可以同时通过
	lim := createTestRateLimiter(1.0, 5)

	// 前 5 个请求应允许（burst 容量）
	for i := 0; i < 5; i++ {
		if !lim.allow("user-1") {
			t.Errorf("request %d should be allowed (within burst)", i+1)
		}
	}

	// 第 6 个请求应被拒绝（burst 已耗尽，rate=1.0 即每秒补充 1 个 token，等待时间不足）
	if lim.allow("user-1") {
		t.Error("request 6 should be blocked (burst exhausted)")
	}
}

func TestUserRateLimiterBlocksOverLimit(t *testing.T) {
	lim := createTestRateLimiter(1.0, 1) // burst=1

	// 第一个请求应允许（burst=1）
	if !lim.allow("user-2") {
		t.Fatal("first request should be allowed")
	}

	// 后续请求应被拒绝（rate 限制，未到补充时间）
	if lim.allow("user-2") {
		t.Fatal("second request should be blocked by rate limit")
	}
}

func TestUserRateLimiterPerUserIsolation(t *testing.T) {
	lim := createTestRateLimiter(1.0, 1) // burst=1

	// user-3 消耗所有额度
	if !lim.allow("user-3") {
		t.Fatal("first request for user-3 should be allowed")
	}
	if lim.allow("user-3") {
		t.Fatal("second request for user-3 should be blocked")
	}

	// user-4 是独立用户，仍应有额度
	if !lim.allow("user-4") {
		t.Fatal("user-4 should have independent quota")
	}
}

func TestRateLimitMiddlewareAllowsWithinBurst(t *testing.T) {
	// 创建局部限流器用于测试（burst=10）
	testLimiter := createTestRateLimiter(10.0, 10)

	// 临时替换（仅测试用）
	origLimiter := aiTriggerLimiter
	defer func() { aiTriggerLimiter = origLimiter }()
	aiTriggerLimiter = testLimiter

	mw := AITriggerRateLimitMiddleware()
	handler := &mockHandler{}
	wrapped := mw(handler)

	// 前 10 个请求在 burst 范围内应通过
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("POST", "/test", nil)
		req = req.WithContext(context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "rate-test-user"}))
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimitMiddlewareReturns429WhenExceeded(t *testing.T) {
	// 创建严格限流器（burst=0，几乎不允许任何请求）
	testLimiter := createTestRateLimiter(0.001, 0)

	origLimiter := aiTriggerLimiter
	defer func() { aiTriggerLimiter = origLimiter }()
	aiTriggerLimiter = testLimiter

	mw := AITriggerRateLimitMiddleware()
	handler := &mockHandler{}
	wrapped := mw(handler)

	req := httptest.NewRequest("POST", "/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "rate-test-user-2"}))
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
	if handler.called != 0 {
		t.Errorf("handler should not be called when rate limited, got called=%d", handler.called)
	}
}

func TestRateLimitMiddlewareSkipsWhenNoUser(t *testing.T) {
	mw := AITriggerRateLimitMiddleware()
	handler := &mockHandler{}
	wrapped := mw(handler)

	// 无用户上下文的请求应跳过限流，直接通过
	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for no-user request, got %d", w.Code)
	}
	if handler.called != 1 {
		t.Errorf("handler should be called once, got %d", handler.called)
	}
}
