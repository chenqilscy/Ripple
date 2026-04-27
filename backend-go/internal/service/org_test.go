package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
)

type stubOrgRepo struct {
	roles       map[string]domain.OrgRole
	removedUser string
}

func (s *stubOrgRepo) Create(_ context.Context, _ *domain.Organization) error { return nil }
func (s *stubOrgRepo) GetByID(_ context.Context, _ string) (*domain.Organization, error) {
	return &domain.Organization{ID: "org-1", Name: "org", Slug: "org-1", OwnerID: "owner", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}
func (s *stubOrgRepo) GetBySlug(_ context.Context, _ string) (*domain.Organization, error) {
	return s.GetByID(context.Background(), "org-1")
}
func (s *stubOrgRepo) ListByUser(_ context.Context, _ string) ([]domain.Organization, error) {
	return nil, nil
}
func (s *stubOrgRepo) AddMember(_ context.Context, _ *domain.OrgMember) error { return nil }
func (s *stubOrgRepo) GetMemberRole(_ context.Context, _ string, userID string) (domain.OrgRole, error) {
	r, ok := s.roles[userID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return r, nil
}
func (s *stubOrgRepo) ListMembers(_ context.Context, _ string) ([]domain.OrgMember, error) { return nil, nil }
func (s *stubOrgRepo) UpdateMemberRole(_ context.Context, _, _ string, _ domain.OrgRole) error { return nil }
func (s *stubOrgRepo) RemoveMember(_ context.Context, _ string, userID string) error {
	s.removedUser = userID
	return nil
}
func (s *stubOrgRepo) CountOwners(_ context.Context, _ string) (int, error) { return 1, nil }

func TestOrgService_RemoveMember_RejectsSelfRemoval(t *testing.T) {
	repo := &stubOrgRepo{roles: map[string]domain.OrgRole{
		"admin-1":  domain.OrgRoleAdmin,
		"member-1": domain.OrgRoleMember,
	}}
	svc := NewOrgService(repo)

	err := svc.RemoveMember(context.Background(), &domain.User{ID: "admin-1"}, "org-1", "admin-1")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
	if repo.removedUser != "" {
		t.Fatalf("expected no repo removal call, got removedUser=%s", repo.removedUser)
	}
}

func TestOrgService_RemoveMember_AllowsAdminRemovingMember(t *testing.T) {
	repo := &stubOrgRepo{roles: map[string]domain.OrgRole{
		"admin-1":  domain.OrgRoleAdmin,
		"member-1": domain.OrgRoleMember,
	}}
	svc := NewOrgService(repo)

	err := svc.RemoveMember(context.Background(), &domain.User{ID: "admin-1"}, "org-1", "member-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.removedUser != "member-1" {
		t.Fatalf("expected member-1 to be removed, got %q", repo.removedUser)
	}
}
