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
	orgs   store.OrgRepository
	quotas store.OrgQuotaRepository
	audit  store.AuditLogRepository
}

// NewOrgService 构造。
func NewOrgService(orgs store.OrgRepository) *OrgService {
	return &OrgService{orgs: orgs}
}

// WithQuotaRepository 注入组织配额仓储（P14-A）。
func (s *OrgService) WithQuotaRepository(quotas store.OrgQuotaRepository) *OrgService {
	s.quotas = quotas
	return s
}

// WithAuditLogRepository 注入审计日志仓储，用于记录配额变更。
func (s *OrgService) WithAuditLogRepository(audit store.AuditLogRepository) *OrgService {
	s.audit = audit
	return s
}

// CreateOrgInput 创建组织入参。
type CreateOrgInput struct {
	Name        string
	Slug        string
	Description string
}

// UpdateOrgQuotaInput 更新组织配额入参。nil 表示该维度不变。
type UpdateOrgQuotaInput struct {
	MaxMembers     *int64
	MaxLakes       *int64
	MaxNodes       *int64
	MaxAttachments *int64
	MaxAPIKeys     *int64
	MaxStorageMB   *int64
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

// GetQuota 获取组织配额。调用者需是组织成员。
func (s *OrgService) GetQuota(ctx context.Context, actor *domain.User, orgID string) (*domain.OrgQuota, error) {
	if err := s.requireMember(ctx, actor.ID, orgID); err != nil {
		return nil, err
	}
	if s.quotas == nil {
		return domain.DefaultOrgQuota(orgID), nil
	}
	return s.quotas.EnsureDefault(ctx, orgID)
}

// UpdateQuota 更新组织配额。调用者需是 ADMIN+。
func (s *OrgService) UpdateQuota(ctx context.Context, actor *domain.User, orgID string, in UpdateOrgQuotaInput) (*domain.OrgQuota, error) {
	if s.quotas == nil {
		return nil, fmt.Errorf("%w: quota repository not configured", domain.ErrInvalidInput)
	}
	if err := s.requireAdmin(ctx, actor.ID, orgID); err != nil {
		return nil, err
	}
	current, err := s.quotas.EnsureDefault(ctx, orgID)
	if err != nil {
		return nil, err
	}
	next := *current
	if err := applyQuotaUpdate(&next, in); err != nil {
		return nil, err
	}
	next.UpdatedAt = time.Now().UTC()
	if err := s.quotas.Update(ctx, &next); err != nil {
		return nil, err
	}
	if s.audit != nil {
		if err := s.audit.Write(ctx, &domain.AuditLog{
			ID:           platform.NewID(),
			ActorID:      actor.ID,
			Action:       domain.AuditOrgQuotaUpdate,
			ResourceType: "org_quota",
			ResourceID:   orgID,
			Detail: map[string]any{
				"before": quotaAuditDetail(current),
				"after":  quotaAuditDetail(&next),
			},
			CreatedAt: time.Now().UTC(),
		}); err != nil {
			return nil, fmt.Errorf("quota updated but audit failed: %w", err)
		}
	}
	return &next, nil
}

// CheckQuota 检查某个资源维度是否会超限。used 是当前用量，delta 是即将新增的用量。
func (s *OrgService) CheckQuota(ctx context.Context, orgID string, key domain.OrgQuotaKey, used, delta int64) error {
	if used < 0 || delta < 0 {
		return fmt.Errorf("%w: quota usage must be non-negative", domain.ErrInvalidInput)
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	if delta > maxInt64-used {
		return fmt.Errorf("%w: org %s %s quota usage overflow", domain.ErrQuotaExceeded, orgID, key)
	}
	quota := domain.DefaultOrgQuota(orgID)
	if s.quotas != nil {
		q, err := s.quotas.EnsureDefault(ctx, orgID)
		if err != nil {
			return err
		}
		quota = q
	}
	limit, err := quota.LimitFor(key)
	if err != nil {
		return err
	}
	if used+delta > limit {
		return fmt.Errorf("%w: org %s %s quota %d would be exceeded by %d", domain.ErrQuotaExceeded, orgID, key, limit, used+delta)
	}
	return nil
}

func applyQuotaUpdate(q *domain.OrgQuota, in UpdateOrgQuotaInput) error {
	changed := false
	if in.MaxMembers != nil {
		if *in.MaxMembers < 1 {
			return fmt.Errorf("%w: max_members must be >= 1", domain.ErrInvalidInput)
		}
		q.MaxMembers = *in.MaxMembers
		changed = true
	}
	updates := []struct {
		name string
		in   *int64
		set  func(int64)
	}{
		{name: "max_lakes", in: in.MaxLakes, set: func(v int64) { q.MaxLakes = v }},
		{name: "max_nodes", in: in.MaxNodes, set: func(v int64) { q.MaxNodes = v }},
		{name: "max_attachments", in: in.MaxAttachments, set: func(v int64) { q.MaxAttachments = v }},
		{name: "max_api_keys", in: in.MaxAPIKeys, set: func(v int64) { q.MaxAPIKeys = v }},
		{name: "max_storage_mb", in: in.MaxStorageMB, set: func(v int64) { q.MaxStorageMB = v }},
	}
	for _, u := range updates {
		if u.in == nil {
			continue
		}
		if *u.in < 0 {
			return fmt.Errorf("%w: %s must be >= 0", domain.ErrInvalidInput, u.name)
		}
		u.set(*u.in)
		changed = true
	}
	if !changed {
		return fmt.Errorf("%w: no quota fields provided", domain.ErrInvalidInput)
	}
	return nil
}

func quotaAuditDetail(q *domain.OrgQuota) map[string]int64 {
	return map[string]int64{
		"max_members":     q.MaxMembers,
		"max_lakes":       q.MaxLakes,
		"max_nodes":       q.MaxNodes,
		"max_attachments": q.MaxAttachments,
		"max_api_keys":    q.MaxAPIKeys,
		"max_storage_mb":  q.MaxStorageMB,
	}
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
