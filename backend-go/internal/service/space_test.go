package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
)

type memSpaceRepo struct {
	mu      sync.Mutex
	spaces  map[string]*domain.Space
	members map[string]map[string]*domain.SpaceMember // spaceID -> userID -> member
}

func newMemSpaceRepo() *memSpaceRepo {
	return &memSpaceRepo{
		spaces:  map[string]*domain.Space{},
		members: map[string]map[string]*domain.SpaceMember{},
	}
}

func (r *memSpaceRepo) Create(_ context.Context, s *domain.Space) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *s
	r.spaces[s.ID] = &cp
	return nil
}

func (r *memSpaceRepo) GetByID(_ context.Context, id string) (*domain.Space, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.spaces[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (r *memSpaceRepo) UpdateMeta(_ context.Context, id, name, desc string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.spaces[id]
	if !ok {
		return domain.ErrNotFound
	}
	s.Name = name
	s.Description = desc
	return nil
}

func (r *memSpaceRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.spaces[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.spaces, id)
	delete(r.members, id)
	return nil
}

func (r *memSpaceRepo) UpsertMember(_ context.Context, m *domain.SpaceMember) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.members[m.SpaceID] == nil {
		r.members[m.SpaceID] = map[string]*domain.SpaceMember{}
	}
	cp := *m
	r.members[m.SpaceID][m.UserID] = &cp
	return nil
}

func (r *memSpaceRepo) RemoveMember(_ context.Context, spaceID, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.members[spaceID] == nil {
		return domain.ErrNotFound
	}
	if _, ok := r.members[spaceID][userID]; !ok {
		return domain.ErrNotFound
	}
	delete(r.members[spaceID], userID)
	return nil
}

func (r *memSpaceRepo) GetMemberRole(_ context.Context, spaceID, userID string) (domain.SpaceRole, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.members[spaceID] == nil {
		return "", domain.ErrNotFound
	}
	m, ok := r.members[spaceID][userID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return m.Role, nil
}

func (r *memSpaceRepo) ListMembers(_ context.Context, spaceID string) ([]domain.SpaceMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.SpaceMember, 0)
	for _, m := range r.members[spaceID] {
		out = append(out, *m)
	}
	return out, nil
}

func (r *memSpaceRepo) ListSpacesByUser(_ context.Context, userID string) ([]domain.Space, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Space, 0)
	for sid, mm := range r.members {
		if _, ok := mm[userID]; ok {
			if s, ok2 := r.spaces[sid]; ok2 {
				out = append(out, *s)
			}
		}
	}
	return out, nil
}

// ---- tests ----

func TestSpaceService_CreateAndGet(t *testing.T) {
	repo := newMemSpaceRepo()
	svc := NewSpaceService(repo)
	owner := &domain.User{ID: "u-owner", Email: "o@x.com"}
	sp, err := svc.Create(context.Background(), owner, CreateSpaceInput{Name: "Team A"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sp.ID == "" || sp.OwnerID != owner.ID {
		t.Fatalf("bad space: %+v", sp)
	}
	got, role, err := svc.Get(context.Background(), owner, sp.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Team A" || role != domain.SpaceRoleOwner {
		t.Fatalf("got %+v role=%s", got, role)
	}
}

func TestSpaceService_CreateRejectsEmptyName(t *testing.T) {
	svc := NewSpaceService(newMemSpaceRepo())
	_, err := svc.Create(context.Background(), &domain.User{ID: "u"}, CreateSpaceInput{Name: "  "})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestSpaceService_NonMemberDeniedGet(t *testing.T) {
	repo := newMemSpaceRepo()
	svc := NewSpaceService(repo)
	owner := &domain.User{ID: "owner"}
	sp, _ := svc.Create(context.Background(), owner, CreateSpaceInput{Name: "S"})
	other := &domain.User{ID: "other"}
	_, _, err := svc.Get(context.Background(), other, sp.ID)
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("want denied, got %v", err)
	}
}

func TestSpaceService_AddMember(t *testing.T) {
	repo := newMemSpaceRepo()
	svc := NewSpaceService(repo)
	owner := &domain.User{ID: "owner"}
	sp, _ := svc.Create(context.Background(), owner, CreateSpaceInput{Name: "S"})

	// add EDITOR
	if err := svc.AddMember(context.Background(), owner, sp.ID, "alice", domain.SpaceRoleEditor); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	// alice 现在能访问
	alice := &domain.User{ID: "alice"}
	_, role, err := svc.Get(context.Background(), alice, sp.ID)
	if err != nil || role != domain.SpaceRoleEditor {
		t.Fatalf("alice role=%s err=%v", role, err)
	}
	// alice (non-owner) 不能 AddMember
	if err := svc.AddMember(context.Background(), alice, sp.ID, "bob", domain.SpaceRoleViewer); !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("non-owner add should fail, got %v", err)
	}
	// 不能加 OWNER
	if err := svc.AddMember(context.Background(), owner, sp.ID, "x", domain.SpaceRoleOwner); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("AddMember OWNER should reject, got %v", err)
	}
}

func TestSpaceService_RemoveMember(t *testing.T) {
	repo := newMemSpaceRepo()
	svc := NewSpaceService(repo)
	owner := &domain.User{ID: "owner"}
	sp, _ := svc.Create(context.Background(), owner, CreateSpaceInput{Name: "S"})
	_ = svc.AddMember(context.Background(), owner, sp.ID, "bob", domain.SpaceRoleEditor)

	if err := svc.RemoveMember(context.Background(), owner, sp.ID, "bob"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	// owner 不能移除自己
	if err := svc.RemoveMember(context.Background(), owner, sp.ID, owner.ID); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("owner remove self should reject, got %v", err)
	}
}

func TestSpaceService_UpdateAndDelete(t *testing.T) {
	repo := newMemSpaceRepo()
	svc := NewSpaceService(repo)
	owner := &domain.User{ID: "owner"}
	sp, _ := svc.Create(context.Background(), owner, CreateSpaceInput{Name: "Old"})

	if err := svc.UpdateMeta(context.Background(), owner, sp.ID, UpdateMetaInput{Name: "New", Description: "d"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _, _ := svc.Get(context.Background(), owner, sp.ID)
	if got.Name != "New" {
		t.Fatalf("update lost: %+v", got)
	}

	// VIEWER 不能 Update
	_ = svc.AddMember(context.Background(), owner, sp.ID, "v", domain.SpaceRoleViewer)
	v := &domain.User{ID: "v"}
	if err := svc.UpdateMeta(context.Background(), v, sp.ID, UpdateMetaInput{Name: "x"}); !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("viewer update should fail, got %v", err)
	}

	if err := svc.Delete(context.Background(), owner, sp.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, _, err := svc.Get(context.Background(), owner, sp.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestSpaceService_ListMine(t *testing.T) {
	repo := newMemSpaceRepo()
	svc := NewSpaceService(repo)
	owner := &domain.User{ID: "u1"}
	_, _ = svc.Create(context.Background(), owner, CreateSpaceInput{Name: "A"})
	_, _ = svc.Create(context.Background(), owner, CreateSpaceInput{Name: "B"})
	list, err := svc.ListMine(context.Background(), owner)
	if err != nil {
		t.Fatalf("ListMine: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2, got %d", len(list))
	}
}
