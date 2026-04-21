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

// LakeService 湖泊用例。
//
// 当前实现：service 层串行写 PG(membership) + Neo4j(lake)。
// TODO：切换为 Outbox saga（在 PG 事务内写 outbox + membership，
//       由 dispatcher 异步消费写 Neo4j）。审查报告 L2-06。
type LakeService struct {
	lakes       store.LakeRepository
	memberships store.MembershipRepository
}

// NewLakeService 装配。
func NewLakeService(lakes store.LakeRepository, memberships store.MembershipRepository) *LakeService {
	return &LakeService{lakes: lakes, memberships: memberships}
}

// CreateLakeInput 创建湖入参。
type CreateLakeInput struct {
	Name        string
	Description string
	IsPublic    bool
}

// Create 创建湖：先 Neo4j 落 Lake，再 PG 写 OWNER 成员。
// 任一步失败返回错误（暂不补偿，等 Outbox 模式上线后修复）。
func (s *LakeService) Create(ctx context.Context, owner *domain.User, in CreateLakeInput) (*domain.Lake, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: name required", domain.ErrInvalidInput)
	}
	now := time.Now().UTC()
	l := &domain.Lake{
		ID:          platform.NewID(),
		Name:        name,
		Description: in.Description,
		IsPublic:    in.IsPublic,
		OwnerID:     owner.ID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.lakes.Create(ctx, l); err != nil {
		return nil, err
	}
	mem := &domain.LakeMembership{
		UserID:    owner.ID,
		LakeID:    l.ID,
		Role:      domain.RoleOwner,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.memberships.Upsert(ctx, mem); err != nil {
		// TODO Outbox：此处 Neo4j 已写但 PG 失败，状态不一致
		return nil, fmt.Errorf("lake created but membership write failed: %w", err)
	}
	return l, nil
}

// Get 读取湖。私有湖必须有成员关系才允许访问（审查 L4-02）。
func (s *LakeService) Get(ctx context.Context, actor *domain.User, lakeID string) (*domain.Lake, domain.Role, error) {
	l, err := s.lakes.GetByID(ctx, lakeID)
	if err != nil {
		return nil, "", err
	}
	role, err := s.memberships.GetRole(ctx, actor.ID, lakeID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return nil, "", err
		}
		// 不是成员
		if !l.IsPublic {
			return nil, "", domain.ErrPermissionDenied
		}
		role = domain.RoleObserver
	}
	return l, role, nil
}

// ListMine 列出我加入的湖 ID。
func (s *LakeService) ListMine(ctx context.Context, actor *domain.User) ([]string, error) {
	return s.memberships.ListLakesByUser(ctx, actor.ID)
}

// requireRole 取角色并校验权限层级。
func (s *LakeService) requireRole(ctx context.Context, actor *domain.User, lakeID string, min domain.Role) (domain.Role, error) {
	role, err := s.memberships.GetRole(ctx, actor.ID, lakeID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", domain.ErrPermissionDenied
		}
		return "", err
	}
	if !role.AtLeast(min) {
		return "", domain.ErrPermissionDenied
	}
	return role, nil
}

// RequireWrite 至少 Passenger 才能写。
func (s *LakeService) RequireWrite(ctx context.Context, actor *domain.User, lakeID string) error {
	_, err := s.requireRole(ctx, actor, lakeID, domain.RolePassenger)
	return err
}
