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
