// Package service · SpaceService（M3-S1）。
//
// 业务流：
//   - Create：单 SQL 事务（spaces INSERT + space_members INSERT OWNER）
//   - 权限：所有变更操作要求调用者 ≥ EDITOR；删除/转让/成员管理要求 OWNER
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// SpaceService 工作空间用例。
type SpaceService struct {
	spaces store.SpaceRepository
}

// NewSpaceService 装配。
func NewSpaceService(spaces store.SpaceRepository) *SpaceService {
	return &SpaceService{spaces: spaces}
}

// CreateSpaceInput 创建入参。
type CreateSpaceInput struct {
	Name        string
	Description string
}

// Create 创建空间。当前实现：先 INSERT spaces，再 INSERT OWNER 成员。
// 不强制使用 PG 事务（spaces 写入失败则不会有 member 残留；
// member 写入失败时由 caller 重试或后续清理 cron 处理）。
// TODO(S1.5): 接入 TxRunner 做事务化包装。
func (s *SpaceService) Create(ctx context.Context, owner *domain.User, in CreateSpaceInput) (*domain.Space, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: name required", domain.ErrInvalidInput)
	}
	if len(name) > 64 {
		return nil, fmt.Errorf("%w: name too long (max 64)", domain.ErrInvalidInput)
	}
	now := time.Now().UTC()
	sp := &domain.Space{
		ID:                  platform.NewID(),
		OwnerID:             owner.ID,
		Name:                name,
		Description:         strings.TrimSpace(in.Description),
		LLMQuotaMonthly:     10000,
		LLMUsedCurrentMonth: 0,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := s.spaces.Create(ctx, sp); err != nil {
		return nil, err
	}
	mem := &domain.SpaceMember{
		SpaceID:   sp.ID,
		UserID:    owner.ID,
		Role:      domain.SpaceRoleOwner,
		JoinedAt:  now,
		UpdatedAt: now,
	}
	if err := s.spaces.UpsertMember(ctx, mem); err != nil {
		return nil, fmt.Errorf("space create: add owner member: %w", err)
	}
	return sp, nil
}

// Get 读取 Space。要求 actor 是成员。
func (s *SpaceService) Get(ctx context.Context, actor *domain.User, spaceID string) (*domain.Space, domain.SpaceRole, error) {
	sp, err := s.spaces.GetByID(ctx, spaceID)
	if err != nil {
		return nil, "", err
	}
	role, err := s.spaces.GetMemberRole(ctx, spaceID, actor.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, "", domain.ErrPermissionDenied
		}
		return nil, "", err
	}
	return sp, role, nil
}

// ListMine 列出 actor 加入的所有 Space。
func (s *SpaceService) ListMine(ctx context.Context, actor *domain.User) ([]domain.Space, error) {
	return s.spaces.ListSpacesByUser(ctx, actor.ID)
}

// UpdateMetaInput 更新入参。
type UpdateMetaInput struct {
	Name        string
	Description string
}

// UpdateMeta 仅 OWNER/EDITOR 可改。
func (s *SpaceService) UpdateMeta(ctx context.Context, actor *domain.User, spaceID string, in UpdateMetaInput) error {
	role, err := s.spaces.GetMemberRole(ctx, spaceID, actor.ID)
	if err != nil {
		return err
	}
	if !role.AtLeast(domain.SpaceRoleEditor) {
		return domain.ErrPermissionDenied
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return fmt.Errorf("%w: name required", domain.ErrInvalidInput)
	}
	if len(name) > 64 {
		return fmt.Errorf("%w: name too long", domain.ErrInvalidInput)
	}
	return s.spaces.UpdateMeta(ctx, spaceID, name, strings.TrimSpace(in.Description))
}

// Delete 仅 OWNER。
func (s *SpaceService) Delete(ctx context.Context, actor *domain.User, spaceID string) error {
	role, err := s.spaces.GetMemberRole(ctx, spaceID, actor.ID)
	if err != nil {
		return err
	}
	if role != domain.SpaceRoleOwner {
		return domain.ErrPermissionDenied
	}
	return s.spaces.Delete(ctx, spaceID)
}

// AddMember 仅 OWNER。不允许添加 OWNER（OWNER 转让走单独流程）。
func (s *SpaceService) AddMember(ctx context.Context, actor *domain.User, spaceID, targetUserID string, role domain.SpaceRole) error {
	myRole, err := s.spaces.GetMemberRole(ctx, spaceID, actor.ID)
	if err != nil {
		return err
	}
	if myRole != domain.SpaceRoleOwner {
		return domain.ErrPermissionDenied
	}
	if role == domain.SpaceRoleOwner {
		return fmt.Errorf("%w: cannot add OWNER via AddMember", domain.ErrInvalidInput)
	}
	if !role.IsValid() {
		return fmt.Errorf("%w: invalid role", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(targetUserID) == "" {
		return fmt.Errorf("%w: user id required", domain.ErrInvalidInput)
	}
	now := time.Now().UTC()
	return s.spaces.UpsertMember(ctx, &domain.SpaceMember{
		SpaceID: spaceID, UserID: targetUserID, Role: role,
		JoinedAt: now, UpdatedAt: now,
	})
}

// RemoveMember 仅 OWNER；不可移除自己（防止孤儿空间）。
func (s *SpaceService) RemoveMember(ctx context.Context, actor *domain.User, spaceID, targetUserID string) error {
	myRole, err := s.spaces.GetMemberRole(ctx, spaceID, actor.ID)
	if err != nil {
		return err
	}
	if myRole != domain.SpaceRoleOwner {
		return domain.ErrPermissionDenied
	}
	if targetUserID == actor.ID {
		return fmt.Errorf("%w: owner cannot remove self", domain.ErrInvalidInput)
	}
	return s.spaces.RemoveMember(ctx, spaceID, targetUserID)
}

// ListMembers 任意成员都可查看。
func (s *SpaceService) ListMembers(ctx context.Context, actor *domain.User, spaceID string) ([]domain.SpaceMember, error) {
	if _, err := s.spaces.GetMemberRole(ctx, spaceID, actor.ID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrPermissionDenied
		}
		return nil, err
	}
	return s.spaces.ListMembers(ctx, spaceID)
}
