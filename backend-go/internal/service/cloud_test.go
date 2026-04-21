package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/rs/zerolog"
)

// memCloudTaskRepo · CloudTaskRepository 桩
type memCloudTaskRepo struct {
	mu      sync.Mutex
	data    map[string]*domain.CloudTask
	queue   []string // ID FIFO
	doneIDs map[string][]string
}

func newMemCloudTaskRepo() *memCloudTaskRepo {
	return &memCloudTaskRepo{data: map[string]*domain.CloudTask{}, doneIDs: map[string][]string{}}
}
func (r *memCloudTaskRepo) Create(_ context.Context, t *domain.CloudTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[t.ID] = t
	r.queue = append(r.queue, t.ID)
	return nil
}
func (r *memCloudTaskRepo) GetByID(_ context.Context, id string) (*domain.CloudTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.data[id]; ok {
		return t, nil
	}
	return nil, domain.ErrNotFound
}
func (r *memCloudTaskRepo) ListByOwner(_ context.Context, ownerID string, _ int) ([]domain.CloudTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.CloudTask{}
	for _, t := range r.data {
		if t.OwnerID == ownerID {
			out = append(out, *t)
		}
	}
	return out, nil
}
func (r *memCloudTaskRepo) ClaimNext(_ context.Context) (*domain.CloudTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for len(r.queue) > 0 {
		id := r.queue[0]
		r.queue = r.queue[1:]
		t, ok := r.data[id]
		if !ok || t.Status != domain.CloudStatusQueued {
			continue
		}
		t.Status = domain.CloudStatusRunning
		now := time.Now().UTC()
		t.StartedAt = &now
		return t, nil
	}
	return nil, domain.ErrNotFound
}
func (r *memCloudTaskRepo) MarkDone(_ context.Context, id string, ids []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.data[id]; ok {
		t.Status = domain.CloudStatusDone
		t.ResultNodeIDs = ids
		now := time.Now().UTC()
		t.CompletedAt = &now
	}
	r.doneIDs[id] = ids
	return nil
}
func (r *memCloudTaskRepo) MarkFailed(_ context.Context, id string, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.data[id]; ok {
		t.Status = domain.CloudStatusFailed
		t.LastError = reason
		t.RetryCount++
	}
	return nil
}
func (r *memCloudTaskRepo) RecoverRunning(_ context.Context) (int64, error) { return 0, nil }

// fakeLLM · llm.Client 桩
type fakeLLM struct {
	candidates []string
	err        error
}

func (f *fakeLLM) Generate(_ context.Context, _ string, n int) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	if n > len(f.candidates) {
		return f.candidates, nil
	}
	return f.candidates[:n], nil
}

// ---------- Tests ----------

func TestCloud_Generate_ValidatesInput(t *testing.T) {
	ctx := context.Background()
	tasks := newMemCloudTaskRepo()
	lakes := newMemLakeRepo()
	nodes := newMemNodeRepo()
	svc := NewCloudService(tasks, nodes, lakes)
	u := &domain.User{ID: "u1"}

	if _, err := svc.Generate(ctx, u, CreateCloudInput{Prompt: ""}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("empty prompt: want ErrInvalidInput, got %v", err)
	}
	if _, err := svc.Generate(ctx, u, CreateCloudInput{Prompt: "x", N: 99}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("n>10: want ErrInvalidInput, got %v", err)
	}
	if _, err := svc.Generate(ctx, u, CreateCloudInput{Prompt: "x", N: 3, NodeType: "BAD"}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("invalid type: want ErrInvalidInput, got %v", err)
	}
	if _, err := svc.Generate(ctx, u, CreateCloudInput{Prompt: "x", N: 3, LakeID: "no-such-lake"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing lake: want ErrNotFound, got %v", err)
	}
}

func TestCloud_Generate_EnqueuesQueued(t *testing.T) {
	ctx := context.Background()
	tasks := newMemCloudTaskRepo()
	svc := NewCloudService(tasks, newMemNodeRepo(), newMemLakeRepo())
	u := &domain.User{ID: "u1"}

	tk, err := svc.Generate(ctx, u, CreateCloudInput{Prompt: "hello", N: 3})
	if err != nil {
		t.Fatal(err)
	}
	if tk.Status != domain.CloudStatusQueued || tk.NodeType != domain.NodeTypeText {
		t.Fatalf("bad task: %+v", tk)
	}
}

func TestCloud_GetTask_OwnershipEnforced(t *testing.T) {
	ctx := context.Background()
	tasks := newMemCloudTaskRepo()
	svc := NewCloudService(tasks, newMemNodeRepo(), newMemLakeRepo())
	tasks.data["t1"] = &domain.CloudTask{ID: "t1", OwnerID: "u1", Status: domain.CloudStatusQueued}
	if _, err := svc.GetTask(ctx, &domain.User{ID: "u2"}, "t1"); !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("non-owner should be denied, got %v", err)
	}
}

func TestAIWeaver_Process_CreatesMistNodes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tasks := newMemCloudTaskRepo()
	nodes := newMemNodeRepo()
	tasks.data["task-1"] = &domain.CloudTask{
		ID: "task-1", OwnerID: "u1", LakeID: "", Prompt: "p", N: 3,
		NodeType: domain.NodeTypeText, Status: domain.CloudStatusQueued,
		CreatedAt: time.Now(),
	}
	tasks.queue = []string{"task-1"}

	weaver := NewAIWeaver(tasks, nodes, &fakeLLM{candidates: []string{"a", "b", "c"}}, nil, zerolog.Nop(), 1)
	go weaver.Run(ctx)

	// 等任务被处理
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout: task not processed")
		default:
		}
		tasks.mu.Lock()
		t1 := tasks.data["task-1"]
		tasks.mu.Unlock()
		if t1.Status == domain.CloudStatusDone {
			if len(t1.ResultNodeIDs) != 3 {
				t.Fatalf("want 3 nodes, got %d", len(t1.ResultNodeIDs))
			}
			// 校验 mist 状态 + ttl
			for _, id := range t1.ResultNodeIDs {
				n := nodes.data[id]
				if n.State != domain.StateMist {
					t.Fatalf("node should be MIST, got %s", n.State)
				}
				if n.TTLAt == nil {
					t.Fatalf("MIST node must have TTL")
				}
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestAIWeaver_Process_LLMFailureMarksFailed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tasks := newMemCloudTaskRepo()
	tasks.data["task-2"] = &domain.CloudTask{
		ID: "task-2", OwnerID: "u1", Prompt: "p", N: 2,
		NodeType: domain.NodeTypeText, Status: domain.CloudStatusQueued,
		CreatedAt: time.Now(),
	}
	tasks.queue = []string{"task-2"}

	weaver := NewAIWeaver(tasks, newMemNodeRepo(), &fakeLLM{err: errors.New("boom")}, nil, zerolog.Nop(), 1)
	go weaver.Run(ctx)

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout")
		default:
		}
		tasks.mu.Lock()
		t2 := tasks.data["task-2"]
		tasks.mu.Unlock()
		if t2.Status == domain.CloudStatusFailed {
			if t2.LastError == "" {
				t.Fatalf("want last_error set")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
