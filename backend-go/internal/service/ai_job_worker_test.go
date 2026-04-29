package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/rs/zerolog"
)

type aiWorkerJobsStub struct {
	updates []domain.AiJobStatus
	errors  []string
}

type aiWorkerPromptTemplatesStub struct {
	tpl domain.PromptTemplate
}

func (s *aiWorkerPromptTemplatesStub) Create(context.Context, domain.PromptTemplate) (*domain.PromptTemplate, error) {
	return nil, nil
}

func (s *aiWorkerPromptTemplatesStub) GetByID(context.Context, string) (*domain.PromptTemplate, error) {
	cp := s.tpl
	return &cp, nil
}

func (s *aiWorkerPromptTemplatesStub) ListVisible(context.Context, string, []string, int, int) ([]domain.PromptTemplate, int, error) {
	return nil, 0, nil
}

func (s *aiWorkerPromptTemplatesStub) Update(context.Context, string, domain.PromptTemplateUpdate) error {
	return nil
}

func (s *aiWorkerPromptTemplatesStub) Delete(context.Context, string) error {
	return nil
}

func (s *aiWorkerJobsStub) CreateWithConflictCheck(context.Context, domain.AiJob) (*domain.AiJob, bool, error) {
	return nil, false, nil
}

func (s *aiWorkerJobsStub) GetByNodeID(context.Context, string) (*domain.AiJob, error) {
	return nil, domain.ErrNotFound
}

func (s *aiWorkerJobsStub) GetByID(context.Context, string) (*domain.AiJob, error) {
	return nil, domain.ErrNotFound
}

func (s *aiWorkerJobsStub) ListPending(context.Context, int) ([]domain.AiJob, error) {
	return nil, nil
}

func (s *aiWorkerJobsStub) UpdateStatus(_ context.Context, _ string, status domain.AiJobStatus, _ int, errMsg string) error {
	s.updates = append(s.updates, status)
	s.errors = append(s.errors, errMsg)
	return nil
}

func (s *aiWorkerJobsStub) RecoverProcessing(context.Context) (int64, error) {
	return 0, nil
}

func TestAiJobWorkerBuildVarsRejectsCrossLakeSelectedNodes(t *testing.T) {
	ctx := context.Background()
	nodes := newMemNodeRepo()
	_ = nodes.Create(ctx, &domain.Node{ID: "target", LakeID: "lake-1", OwnerID: "u-1", Content: "target", Type: domain.NodeTypeText})
	_ = nodes.Create(ctx, &domain.Node{ID: "secret", LakeID: "lake-2", OwnerID: "u-2", Content: "secret", Type: domain.NodeTypeText})

	worker := NewAiJobWorker(&aiWorkerJobsStub{}, nodes, newMemLakeRepo(), nil, nil, zerolog.Nop(), 1)
	vars, err := worker.buildVars(ctx, &domain.AiJob{ID: "job", LakeID: "lake-1", InputNodeIDs: []string{"secret"}}, nodes.data["target"])
	if err == nil {
		t.Fatalf("expected cross-lake selected node to return an error")
	}
	if _, ok := vars["selected_nodes"]; ok {
		t.Fatalf("selected_nodes should not be set when an input node is outside the job lake")
	}
}

func TestAiJobWorkerProcessFailsOnCrossLakeSelectedNodes(t *testing.T) {
	ctx := context.Background()
	nodes := newMemNodeRepo()
	_ = nodes.Create(ctx, &domain.Node{ID: "target", LakeID: "lake-1", OwnerID: "u-1", Content: "target", Type: domain.NodeTypeText})
	_ = nodes.Create(ctx, &domain.Node{ID: "secret", LakeID: "lake-2", OwnerID: "u-2", Content: "secret", Type: domain.NodeTypeText})
	jobs := &aiWorkerJobsStub{}
	templates := &aiWorkerPromptTemplatesStub{tpl: domain.PromptTemplate{ID: "tpl", Template: "{{selected_nodes}}"}}
	worker := NewAiJobWorker(jobs, nodes, newMemLakeRepo(), templates, newFakeRouter([]string{"should not run"}, nil), zerolog.Nop(), 1)

	worker.process(ctx, &domain.AiJob{ID: "job", NodeID: "target", LakeID: "lake-1", Status: domain.AiJobPending, PromptTemplateID: "tpl", InputNodeIDs: []string{"secret"}}, 0)

	if len(jobs.updates) == 0 || jobs.updates[len(jobs.updates)-1] != domain.AiJobFailed {
		t.Fatalf("expected final status failed, got %#v", jobs.updates)
	}
	if len(jobs.errors) == 0 || jobs.errors[len(jobs.errors)-1] != aiJobErrInputsUnavailable {
		t.Fatalf("expected visible cross-lake error, got %#v", jobs.errors)
	}
}

func TestAiJobWorkerProcessTrimsAndCapsLLMOutput(t *testing.T) {
	ctx := context.Background()
	nodes := newMemNodeRepo()
	_ = nodes.Create(ctx, &domain.Node{ID: "target", LakeID: "lake-1", OwnerID: "u-1", Content: "target", Type: domain.NodeTypeText, UpdatedAt: time.Now().UTC()})
	jobs := &aiWorkerJobsStub{}
	longOutput := " \n" + strings.Repeat("界", maxAIJobOutputRunes+5) + "\n "
	worker := NewAiJobWorker(jobs, nodes, newMemLakeRepo(), nil, newFakeRouter([]string{longOutput}, nil), zerolog.Nop(), 1)

	worker.process(ctx, &domain.AiJob{ID: "job", NodeID: "target", LakeID: "lake-1", Status: domain.AiJobPending}, 0)

	updated, err := nodes.GetByID(ctx, "target")
	if err != nil {
		t.Fatalf("get updated node: %v", err)
	}
	if got := len([]rune(updated.Content)); got != maxAIJobOutputRunes {
		t.Fatalf("expected capped output length %d, got %d", maxAIJobOutputRunes, got)
	}
	if strings.HasPrefix(updated.Content, " ") || strings.HasSuffix(updated.Content, " ") || strings.Contains(updated.Content, "\n ") {
		t.Fatalf("output should be trimmed before persisting, got %q", updated.Content[:1])
	}
	if len(jobs.updates) == 0 || jobs.updates[len(jobs.updates)-1] != domain.AiJobDone {
		t.Fatalf("expected final status done, got %#v", jobs.updates)
	}
}
