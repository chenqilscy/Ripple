package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/llm"
)

// memPermaRepo
type memPermaRepo struct {
	mu   sync.Mutex
	data map[string]*domain.PermaNode
}

func newMemPermaRepo() *memPermaRepo {
	return &memPermaRepo{data: map[string]*domain.PermaNode{}}
}
func (r *memPermaRepo) Create(_ context.Context, p *domain.PermaNode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[p.ID] = p
	return nil
}
func (r *memPermaRepo) GetByID(_ context.Context, id string) (*domain.PermaNode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.data[id]; ok {
		return p, nil
	}
	return nil, domain.ErrNotFound
}
func (r *memPermaRepo) ListByLake(_ context.Context, lakeID string, limit int) ([]domain.PermaNode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.PermaNode{}
	for _, p := range r.data {
		if p.LakeID == lakeID {
			out = append(out, *p)
		}
	}
	return out, nil
}

// memMembers
type memMembers struct {
	roles map[string]domain.Role // key = userID|lakeID
}

func memberKey(u, l string) string { return u + "|" + l }
func (m *memMembers) GetRole(_ context.Context, u, l string) (domain.Role, error) {
	if r, ok := m.roles[memberKey(u, l)]; ok {
		return r, nil
	}
	return "", domain.ErrNotFound
}

// 复用 cloud_test.go 已有的 newMemNodeRepo / newFakeRouter
// 但 newFakeRouter 只支持 chunks slice，OK。

func TestCrystallize_HappyPath(t *testing.T) {
	perma := newMemPermaRepo()
	nodes := newMemNodeRepo()
	// 准备 3 个 mist 节点，同 lake
	uid := "user-1"
	lid := "lake-1"
	ids := []string{"n1", "n2", "n3"}
	for _, id := range ids {
		_ = nodes.Create(context.Background(), &domain.Node{
			ID: id, LakeID: lid, OwnerID: uid, Content: "想法 " + id, Type: domain.NodeTypeText, State: domain.StateMist,
		})
	}
	members := &memMembers{roles: map[string]domain.Role{memberKey(uid, lid): domain.RoleOwner}}
	router := newFakeRouter([]string{"这是一段总结。"}, nil)

	svc := NewCrystallizeService(perma, nodes, members, router)
	p, err := svc.Crystallize(context.Background(), &domain.User{ID: uid}, CrystallizeInput{
		LakeID: lid, SourceNodeIDs: ids, TitleHint: "灵感",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Summary != "这是一段总结。" {
		t.Fatalf("summary=%q", p.Summary)
	}
	if len(p.SourceNodeIDs) != 3 {
		t.Fatalf("source ids=%v", p.SourceNodeIDs)
	}
	if p.Title != "灵感" {
		t.Fatalf("title=%q", p.Title)
	}
}

func TestCrystallize_RejectsTooFewSources(t *testing.T) {
	svc := NewCrystallizeService(newMemPermaRepo(), newMemNodeRepo(), &memMembers{}, newFakeRouter(nil, nil))
	_, err := svc.Crystallize(context.Background(), &domain.User{ID: "u"}, CrystallizeInput{LakeID: "l", SourceNodeIDs: []string{"n1"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestCrystallize_NonMemberDenied(t *testing.T) {
	nodes := newMemNodeRepo()
	for _, id := range []string{"n1", "n2"} {
		_ = nodes.Create(context.Background(), &domain.Node{ID: id, LakeID: "lake-x", OwnerID: "u", Content: "x", Type: domain.NodeTypeText, State: domain.StateMist})
	}
	svc := NewCrystallizeService(newMemPermaRepo(), nodes, &memMembers{roles: map[string]domain.Role{}}, newFakeRouter([]string{"s"}, nil))
	_, err := svc.Crystallize(context.Background(), &domain.User{ID: "stranger"}, CrystallizeInput{LakeID: "lake-x", SourceNodeIDs: []string{"n1", "n2"}})
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("want ErrPermissionDenied, got %v", err)
	}
}

func TestCrystallize_RejectsCrossLake(t *testing.T) {
	nodes := newMemNodeRepo()
	_ = nodes.Create(context.Background(), &domain.Node{ID: "n1", LakeID: "lake-a", OwnerID: "u", Content: "a", Type: domain.NodeTypeText, State: domain.StateMist})
	_ = nodes.Create(context.Background(), &domain.Node{ID: "n2", LakeID: "lake-b", OwnerID: "u", Content: "b", Type: domain.NodeTypeText, State: domain.StateMist})
	members := &memMembers{roles: map[string]domain.Role{memberKey("u", "lake-a"): domain.RoleOwner}}
	svc := NewCrystallizeService(newMemPermaRepo(), nodes, members, newFakeRouter([]string{"x"}, nil))
	_, err := svc.Crystallize(context.Background(), &domain.User{ID: "u"}, CrystallizeInput{LakeID: "lake-a", SourceNodeIDs: []string{"n1", "n2"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestCrystallize_LLMError(t *testing.T) {
	nodes := newMemNodeRepo()
	for _, id := range []string{"n1", "n2"} {
		_ = nodes.Create(context.Background(), &domain.Node{ID: id, LakeID: "l", OwnerID: "u", Content: "c", Type: domain.NodeTypeText, State: domain.StateMist})
	}
	members := &memMembers{roles: map[string]domain.Role{memberKey("u", "l"): domain.RoleOwner}}
	svc := NewCrystallizeService(newMemPermaRepo(), nodes, members, newFakeRouter(nil, errors.New("llm down")))
	_, err := svc.Crystallize(context.Background(), &domain.User{ID: "u"}, CrystallizeInput{LakeID: "l", SourceNodeIDs: []string{"n1", "n2"}})
	if err == nil {
		t.Fatal("want error")
	}
}

// 防止 llm 包未使用警告
var _ = llm.ModalityText

// OBSERVER 角色不能凝结（M3-S2 P4 安全）
func TestCrystallize_ObserverDenied(t *testing.T) {
	nodes := newMemNodeRepo()
	for _, id := range []string{"n1", "n2"} {
		_ = nodes.Create(context.Background(), &domain.Node{ID: id, LakeID: "l", OwnerID: "u", Content: "c", Type: domain.NodeTypeText, State: domain.StateMist})
	}
	members := &memMembers{roles: map[string]domain.Role{memberKey("u", "l"): domain.RoleObserver}}
	svc := NewCrystallizeService(newMemPermaRepo(), nodes, members, newFakeRouter([]string{"ok"}, nil))
	_, err := svc.Crystallize(context.Background(), &domain.User{ID: "u"}, CrystallizeInput{LakeID: "l", SourceNodeIDs: []string{"n1", "n2"}})
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("want ErrPermissionDenied, got %v", err)
	}
}
