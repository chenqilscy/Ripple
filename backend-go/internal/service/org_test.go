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

type stubOrgQuotaRepo struct {
	quota   *domain.OrgQuota
	updated *domain.OrgQuota
}

func (s *stubOrgQuotaRepo) EnsureDefault(_ context.Context, orgID string) (*domain.OrgQuota, error) {
	if s.quota == nil {
		s.quota = domain.DefaultOrgQuota(orgID)
	}
	q := *s.quota
	return &q, nil
}

func (s *stubOrgQuotaRepo) GetByOrgID(_ context.Context, orgID string) (*domain.OrgQuota, error) {
	return s.EnsureDefault(context.Background(), orgID)
}

func (s *stubOrgQuotaRepo) Update(_ context.Context, quota *domain.OrgQuota) error {
	q := *quota
	s.updated = &q
	s.quota = &q
	return nil
}

type stubAuditRepo struct {
	written []*domain.AuditLog
}

func (s *stubAuditRepo) Write(_ context.Context, log *domain.AuditLog) error {
	s.written = append(s.written, log)
	return nil
}

func (s *stubAuditRepo) ListByResource(_ context.Context, _, _ string, _ int) ([]*domain.AuditLog, error) {
	return nil, nil
}

func (s *stubAuditRepo) PruneOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
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
func (s *stubOrgRepo) ListMembers(_ context.Context, _ string) ([]domain.OrgMember, error) {
	out := make([]domain.OrgMember, 0, len(s.roles))
	for userID, role := range s.roles {
		out = append(out, domain.OrgMember{OrgID: "org-1", UserID: userID, Role: role, JoinedAt: time.Now()})
	}
	return out, nil
}
func (s *stubOrgRepo) UpdateMemberRole(_ context.Context, _, _ string, _ domain.OrgRole) error {
	return nil
}
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

func TestOrgService_GetQuota_ReturnsDefaultForMember(t *testing.T) {
	repo := &stubOrgRepo{roles: map[string]domain.OrgRole{"member-1": domain.OrgRoleMember}}
	svc := NewOrgService(repo).WithQuotaRepository(&stubOrgQuotaRepo{})

	quota, err := svc.GetQuota(context.Background(), &domain.User{ID: "member-1"}, "org-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if quota.OrgID != "org-1" || quota.MaxMembers != 20 || quota.MaxAPIKeys != 10 {
		t.Fatalf("unexpected quota: %+v", quota)
	}
}

func TestOrgService_UpdateQuota_RequiresAdmin(t *testing.T) {
	repo := &stubOrgRepo{roles: map[string]domain.OrgRole{"member-1": domain.OrgRoleMember}}
	quotas := &stubOrgQuotaRepo{}
	svc := NewOrgService(repo).WithQuotaRepository(quotas)
	maxMembers := int64(30)

	_, err := svc.UpdateQuota(context.Background(), &domain.User{ID: "member-1"}, "org-1", UpdateOrgQuotaInput{MaxMembers: &maxMembers})
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("want ErrPermissionDenied, got %v", err)
	}
	if quotas.updated != nil {
		t.Fatalf("quota should not be updated by member")
	}
}

func TestOrgService_UpdateQuota_WritesAudit(t *testing.T) {
	repo := &stubOrgRepo{roles: map[string]domain.OrgRole{"admin-1": domain.OrgRoleAdmin}}
	quotas := &stubOrgQuotaRepo{quota: domain.DefaultOrgQuota("org-1")}
	audit := &stubAuditRepo{}
	svc := NewOrgService(repo).WithQuotaRepository(quotas).WithAuditLogRepository(audit)
	maxMembers := int64(42)

	updated, err := svc.UpdateQuota(context.Background(), &domain.User{ID: "admin-1"}, "org-1", UpdateOrgQuotaInput{MaxMembers: &maxMembers})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.MaxMembers != 42 || quotas.updated.MaxMembers != 42 {
		t.Fatalf("quota not updated: %+v", updated)
	}
	if len(audit.written) != 1 {
		t.Fatalf("want one audit log, got %d", len(audit.written))
	}
	if audit.written[0].Action != domain.AuditOrgQuotaUpdate || audit.written[0].ResourceID != "org-1" {
		t.Fatalf("unexpected audit log: %+v", audit.written[0])
	}
}

func TestOrgService_UpdateQuota_RejectsInvalidValues(t *testing.T) {
	repo := &stubOrgRepo{roles: map[string]domain.OrgRole{"admin-1": domain.OrgRoleAdmin}}
	svc := NewOrgService(repo).WithQuotaRepository(&stubOrgQuotaRepo{})
	zeroMembers := int64(0)

	_, err := svc.UpdateQuota(context.Background(), &domain.User{ID: "admin-1"}, "org-1", UpdateOrgQuotaInput{MaxMembers: &zeroMembers})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestOrgService_CheckQuota_ReturnsQuotaExceeded(t *testing.T) {
	repo := &stubOrgRepo{roles: map[string]domain.OrgRole{}}
	svc := NewOrgService(repo).WithQuotaRepository(&stubOrgQuotaRepo{quota: &domain.OrgQuota{OrgID: "org-1", MaxMembers: 2}})

	err := svc.CheckQuota(context.Background(), "org-1", domain.OrgQuotaMembers, 2, 1)
	if !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatalf("want ErrQuotaExceeded, got %v", err)
	}
}

func TestOrgService_CheckQuota_GuardsOverflow(t *testing.T) {
	repo := &stubOrgRepo{roles: map[string]domain.OrgRole{}}
	svc := NewOrgService(repo)
	maxInt64 := int64(^uint64(0) >> 1)

	err := svc.CheckQuota(context.Background(), "org-1", domain.OrgQuotaNodes, maxInt64, 1)
	if !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatalf("want ErrQuotaExceeded, got %v", err)
	}
}

func TestOrgService_AddMember_EnforcesMemberQuota(t *testing.T) {
	repo := &stubOrgRepo{roles: map[string]domain.OrgRole{
		"admin-1": domain.OrgRoleAdmin,
		"user-1":  domain.OrgRoleMember,
	}}
	quotas := &stubOrgQuotaRepo{quota: &domain.OrgQuota{OrgID: "org-1", MaxMembers: 2}}
	svc := NewOrgService(repo).WithQuotaRepository(quotas)

	err := svc.AddMember(context.Background(), &domain.User{ID: "admin-1"}, "org-1", "user-2", domain.OrgRoleMember)
	if !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatalf("want ErrQuotaExceeded, got %v", err)
	}
}
