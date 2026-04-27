package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// slugRe 校验组织 slug：小写字母/数字/连字符，3-40 字符，不以连字符起止。
var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$`)

// OrgService 组织与成员管理（P12-C）。
type OrgService struct {
	orgs store.OrgRepository
}

// NewOrgService 构造。
func NewOrgService(orgs store.OrgRepository) *OrgService {
	return &OrgService{orgs: orgs}
}

// CreateOrgInput 创建组织入参。
type CreateOrgInput struct {
	Name        string
	Slug        string
	Description string
}

// CreateOrg 创建组织并将 actor 设为 OWNER。
func (s *OrgService) CreateOrg(ctx context.Context, actor *domain.User, in CreateOrgInput) (*domain.Organization, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: name required", domain.ErrInvalidInput)
	}
	if !slugRe.MatchString(in.Slug) {
		return nil, fmt.Errorf("%w: slug must be 3-40 lowercase alphanumeric/hyphen, not starting/ending with hyphen", domain.ErrInvalidInput)
	}
	now := time.Now().UTC()
	org := &domain.Organization{
		ID:          platform.NewID(),
		Name:        in.Name,
		Slug:        in.Slug,
		Description: in.Description,
		OwnerID:     actor.ID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.orgs.Create(ctx, org); err != nil {
		return nil, err
	}
	return org, nil
}

// GetOrg 获取组织（调用者需是成员）。
func (s *OrgService) GetOrg(ctx context.Context, actor *domain.User, orgID string) (*domain.Organization, error) {
	if err := s.requireMember(ctx, actor.ID, orgID); err != nil {
		return nil, err
	}
	return s.orgs.GetByID(ctx, orgID)
}

// ListMyOrgs 列出 actor 所在的组织。
func (s *OrgService) ListMyOrgs(ctx context.Context, actor *domain.User) ([]domain.Organization, error) {
	return s.orgs.ListByUser(ctx, actor.ID)
}

// ListMembers 列出组织成员（需是成员）。
func (s *OrgService) ListMembers(ctx context.Context, actor *domain.User, orgID string) ([]domain.OrgMember, error) {
	if err := s.requireMember(ctx, actor.ID, orgID); err != nil {
		return nil, err
	}
	return s.orgs.ListMembers(ctx, orgID)
}

// AddMember 邀请成员（需是 ADMIN+）。OWNER 角色不能通过此接口设置（仅 ADMIN/MEMBER 可邀）。
func (s *OrgService) AddMember(ctx context.Context, actor *domain.User, orgID, targetUserID string, role domain.OrgRole) error {
	if role == domain.OrgRoleOwner {
		return fmt.Errorf("%w: cannot assign OWNER via invite", domain.ErrInvalidInput)
	}
	if !role.IsValid() {
		return fmt.Errorf("%w: invalid role", domain.ErrInvalidInput)
	}
	if err := s.requireAdmin(ctx, actor.ID, orgID); err != nil {
		return err
	}
	m := &domain.OrgMember{
		OrgID:    orgID,
		UserID:   targetUserID,
		Role:     role,
		JoinedAt: time.Now().UTC(),
	}
	return s.orgs.AddMember(ctx, m)
}

// UpdateMemberRole 变更成员角色（需是 ADMIN+；不能修改 OWNER；不能赋予 OWNER）。
func (s *OrgService) UpdateMemberRole(ctx context.Context, actor *domain.User, orgID, targetUserID string, newRole domain.OrgRole) error {
	if newRole == domain.OrgRoleOwner {
		return fmt.Errorf("%w: cannot promote to OWNER", domain.ErrInvalidInput)
	}
	if !newRole.IsValid() {
		return fmt.Errorf("%w: invalid role", domain.ErrInvalidInput)
	}
	if err := s.requireAdmin(ctx, actor.ID, orgID); err != nil {
		return err
	}
	// 保护：不能修改 OWNER
	targetRole, err := s.orgs.GetMemberRole(ctx, orgID, targetUserID)
	if err != nil {
		return err
	}
	if targetRole == domain.OrgRoleOwner {
		return fmt.Errorf("%w: cannot change OWNER role", domain.ErrPermissionDenied)
	}
	return s.orgs.UpdateMemberRole(ctx, orgID, targetUserID, newRole)
}

// RemoveMember 移除成员（需是 ADMIN+；不能移除 OWNER；不能自删最后一位成员）。
func (s *OrgService) RemoveMember(ctx context.Context, actor *domain.User, orgID, targetUserID string) error {
	if err := s.requireAdmin(ctx, actor.ID, orgID); err != nil {
		return err
	}
	if actor.ID == targetUserID {
		return fmt.Errorf("%w: cannot remove yourself", domain.ErrInvalidInput)
	}
	targetRole, err := s.orgs.GetMemberRole(ctx, orgID, targetUserID)
	if err != nil {
		return err
	}
	if targetRole == domain.OrgRoleOwner {
		return fmt.Errorf("%w: cannot remove OWNER", domain.ErrPermissionDenied)
	}
	return s.orgs.RemoveMember(ctx, orgID, targetUserID)
}

// --- 内部权限工具 ---

// IsMember P13-A：检查 userID 是否是 orgID 的成员（任意角色）。返回 false 而非错误当不是成员时。
func (s *OrgService) IsMember(ctx context.Context, userID, orgID string) (bool, error) {
	_, err := s.orgs.GetMemberRole(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *OrgService) requireMember(ctx context.Context, userID, orgID string) error {
	_, err := s.orgs.GetMemberRole(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrPermissionDenied
		}
		return err
	}
	return nil
}

func (s *OrgService) requireAdmin(ctx context.Context, userID, orgID string) error {
	role, err := s.orgs.GetMemberRole(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrPermissionDenied
		}
		return err
	}
	if !role.AtLeast(domain.OrgRoleAdmin) {
		return domain.ErrPermissionDenied
	}
	return nil
}
