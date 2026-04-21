// Package service · InviteService
//
// 湖邀请流程。权限模型：
//   - Create: actor 对 lake 有写权限（NAVIGATOR+）方可；Role 不能是 OWNER
//   - List:   actor 对 lake 有写权限方可（邀请 token 泄露危险）
//   - Revoke: 创建者本人 OR lake OWNER
//   - Preview: 登录用户输入 token 查看湖名（不消耗）
//   - Accept:  登录用户输入 token，原子消费并写 membership；已是成员则幂等（不再消耗 token）
//
// Token：crypto/rand 32B → base64.RawURLEncoding，43 字符。
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// InviteService 邀请业务。
type InviteService struct {
	invites     store.InviteRepository
	memberships store.MembershipRepository
	lakes       store.LakeRepository
}

// NewInviteService 装配。
func NewInviteService(
	invites store.InviteRepository,
	memberships store.MembershipRepository,
	lakes store.LakeRepository,
) *InviteService {
	return &InviteService{invites: invites, memberships: memberships, lakes: lakes}
}

// CreateInviteInput 创建邀请入参。
type CreateInviteInput struct {
	LakeID   string
	Role     domain.Role
	MaxUses  int
	TTL      time.Duration // 相对现在的有效期
}

// Validate 基本校验。
func (in *CreateInviteInput) Validate() error {
	if in.LakeID == "" {
		return fmt.Errorf("%w: lake_id required", domain.ErrInvalidInput)
	}
	if !in.Role.IsValid() {
		return fmt.Errorf("%w: invalid role", domain.ErrInvalidInput)
	}
	if in.Role == domain.RoleOwner {
		return fmt.Errorf("%w: cannot invite as OWNER", domain.ErrInvalidInput)
	}
	if in.MaxUses <= 0 || in.MaxUses > 10000 {
		return fmt.Errorf("%w: max_uses must be 1..10000", domain.ErrInvalidInput)
	}
	if in.TTL <= 0 || in.TTL > 365*24*time.Hour {
		return fmt.Errorf("%w: ttl must be 1s..365d", domain.ErrInvalidInput)
	}
	return nil
}

// Create 签发邀请。
func (s *InviteService) Create(ctx context.Context, actor *domain.User, in CreateInviteInput) (*domain.Invite, error) {
	if actor == nil {
		return nil, domain.ErrPermissionDenied
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	// 权限：actor 必须对 lake 有 NAVIGATOR+ 权限
	if err := s.requireWrite(ctx, actor.ID, in.LakeID); err != nil {
		return nil, err
	}
	token, err := newInviteToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	inv := &domain.Invite{
		ID:        platform.NewID(),
		LakeID:    in.LakeID,
		Token:     token,
		CreatedBy: actor.ID,
		Role:      in.Role,
		MaxUses:   in.MaxUses,
		UsedCount: 0,
		ExpiresAt: now.Add(in.TTL),
		CreatedAt: now,
	}
	if err := s.invites.Create(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

// ListByLake 列出邀请（仅写权限可见）。
func (s *InviteService) ListByLake(ctx context.Context, actor *domain.User, lakeID string, includeInactive bool) ([]domain.Invite, error) {
	if actor == nil {
		return nil, domain.ErrPermissionDenied
	}
	if err := s.requireWrite(ctx, actor.ID, lakeID); err != nil {
		return nil, err
	}
	return s.invites.ListByLake(ctx, lakeID, includeInactive)
}

// Revoke 撤销（创建者或 OWNER）。
func (s *InviteService) Revoke(ctx context.Context, actor *domain.User, inviteID string) error {
	if actor == nil {
		return domain.ErrPermissionDenied
	}
	inv, err := s.invites.GetByID(ctx, inviteID)
	if err != nil {
		return err
	}
	if inv.RevokedAt != nil {
		return nil // 幂等
	}
	if inv.CreatedBy != actor.ID {
		// 非创建者必须是 lake OWNER
		role, err := s.memberships.GetRole(ctx, actor.ID, inv.LakeID)
		if err != nil || !role.AtLeast(domain.RoleOwner) {
			return domain.ErrPermissionDenied
		}
	}
	if err := s.invites.Revoke(ctx, inviteID, time.Now().UTC()); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil // 并发幂等：另一请求已撤销
		}
		return err
	}
	return nil
}

// InvitePreview 预览信息（不消耗）。
type InvitePreview struct {
	LakeID      string
	LakeName    string
	InviterID   string
	Role        domain.Role
	ExpiresAt   time.Time
	UsedCount   int
	MaxUses     int
	Alive       bool
}

// Preview 根据 token 查看预览。
func (s *InviteService) Preview(ctx context.Context, token string) (*InvitePreview, error) {
	inv, err := s.invites.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	lake, err := s.lakes.GetByID(ctx, inv.LakeID)
	if err != nil {
		return nil, err
	}
	return &InvitePreview{
		LakeID:    inv.LakeID,
		LakeName:  lake.Name,
		InviterID: inv.CreatedBy,
		Role:      inv.Role,
		ExpiresAt: inv.ExpiresAt,
		UsedCount: inv.UsedCount,
		MaxUses:   inv.MaxUses,
		Alive:     inv.IsAlive(time.Now().UTC()),
	}, nil
}

// AcceptResult 接受结果。
type AcceptResult struct {
	LakeID       string
	Role         domain.Role
	AlreadyMember bool // 已是成员，未消费 token
}

// Accept 原子接受邀请。
//
// 实现：
//  1. 先查 token 拿到 LakeID
//  2. 若 actor 已是该湖成员：直接返回（不消耗 token）
//  3. 否则 ConsumeByToken（原子 +1 + alive 守护）
//  4. Upsert membership（不在同一事务；consume 成功后即便 upsert 失败，token 已扣减 —
//     属于可接受的最终一致性代价，用户可重试用另一个 token，或管理员手动补 membership）
func (s *InviteService) Accept(ctx context.Context, actor *domain.User, token string) (*AcceptResult, error) {
	if actor == nil {
		return nil, domain.ErrPermissionDenied
	}
	if token == "" {
		return nil, fmt.Errorf("%w: token required", domain.ErrInvalidInput)
	}
	inv, err := s.invites.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	// 已是成员：幂等返回
	if role, err := s.memberships.GetRole(ctx, actor.ID, inv.LakeID); err == nil {
		return &AcceptResult{LakeID: inv.LakeID, Role: role, AlreadyMember: true}, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	// 原子消费
	consumed, err := s.invites.ConsumeByToken(ctx, token, time.Now().UTC())
	if err != nil {
		// 不 alive：统一 400
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("%w: invite invalid or exhausted", domain.ErrInvalidInput)
		}
		return nil, err
	}
	// 写 membership（幂等 upsert）
	now := time.Now().UTC()
	m := &domain.LakeMembership{
		UserID:    actor.ID,
		LakeID:    consumed.LakeID,
		Role:      consumed.Role,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.memberships.Upsert(ctx, m); err != nil {
		return nil, err
	}
	return &AcceptResult{LakeID: consumed.LakeID, Role: consumed.Role, AlreadyMember: false}, nil
}

// requireWrite actor 对 lake 需 NAVIGATOR+。
//
// 注：这段逻辑与 NodeService/EdgeService 中重复。按当前架构约定（AGENTS.md），
// 不为一次性使用抽共享 helper；此处以内部方法保持服务内聚。
func (s *InviteService) requireWrite(ctx context.Context, actorID, lakeID string) error {
	role, err := s.memberships.GetRole(ctx, actorID, lakeID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrPermissionDenied
		}
		return err
	}
	if !role.AtLeast(domain.RoleNavigator) {
		return domain.ErrPermissionDenied
	}
	return nil
}

// newInviteToken crypto/rand 32B → base64.RawURLEncoding，43 字符。
func newInviteToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("token rand: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
