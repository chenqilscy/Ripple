package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/jackc/pgx/v5"
)

// OutboxEventLakeCreated 事件类型常量。
const OutboxEventLakeCreated = "LakeCreated"

// LakeService 湖泊用例（saga 版本）。
//
// Create 在 PG 单事务内：
//  1. upsert OWNER membership
//  2. enqueue outbox(LakeCreated payload=domain.Lake)
//
// 由 OutboxDispatcher 异步消费 outbox，把 Lake 写到 Neo4j。
// Create 立即返回；Neo4j 侧最终一致（通常 < 2s 延迟）。
type LakeService struct {
	lakes       store.LakeRepository
	memberships store.MembershipRepository
	outbox      store.OutboxRepository
	txRunner    store.TxRunner
}

// NewLakeService 装配。
func NewLakeService(
	lakes store.LakeRepository,
	memberships store.MembershipRepository,
	outbox store.OutboxRepository,
	txRunner store.TxRunner,
) *LakeService {
	return &LakeService{lakes: lakes, memberships: memberships, outbox: outbox, txRunner: txRunner}
}

// CreateLakeInput 创建湖入参。
type CreateLakeInput struct {
	Name        string
	Description string
	IsPublic    bool
}

// Create 创建湖（saga）。
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
	mem := &domain.LakeMembership{
		UserID:    owner.ID,
		LakeID:    l.ID,
		Role:      domain.RoleOwner,
		CreatedAt: now,
		UpdatedAt: now,
	}
	payload, err := json.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("marshal lake: %w", err)
	}
	err = s.txRunner.RunInTx(ctx, func(tx pgx.Tx) error {
		if err := s.memberships.UpsertInTx(ctx, tx, mem); err != nil {
			return err
		}
		return s.outbox.EnqueueInTx(ctx, tx, OutboxEventLakeCreated, payload)
	})
	if err != nil {
		return nil, err
	}
	return l, nil
}

// Get 读取湖。私有湖必须有成员关系才允许访问（审查 L4-02）。
// 注意：Neo4j 写入最终一致；若 dispatcher 尚未处理，会返回 ErrNotFound，
// 调用方可短暂重试（通常 < 2s 延迟）。
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
