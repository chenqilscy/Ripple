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
	// SpaceID 可选；非空表示湖归属某 Space。Service 不校验 actor 是否为 Space 成员，
	// 由调用方（HTTP handler）在创建前做好；这样 Service 保持与 Space 解耦。
	SpaceID string
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
		SpaceID:     strings.TrimSpace(in.SpaceID),
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

// LakeWithRole 列表返回结构：湖 + 当前用户在其中的角色。
type LakeWithRole struct {
	Lake *domain.Lake
	Role domain.Role
}

// ListMineFull 一次性拉取 user 的所有湖（含角色），避免 handler 端 N+1。
//
// 实现：
//  1. 单 SQL 取 [(lake_id, role), ...]
//  2. 单 Cypher 批量取 Lake 节点
//  3. 按 lake_id zip 起来；membership 存在但 Lake 不存在（outbox 投影滞后）静默跳过
func (s *LakeService) ListMineFull(ctx context.Context, actor *domain.User) ([]LakeWithRole, error) {
	return s.listMineFull(ctx, actor, "")
}

// ListMineBySpace 按 space_id 过滤；spaceID="" 表示只看个人湖（无 space 归属）。
// 复用 ListMineFull 的 N+1 防御逻辑。
func (s *LakeService) ListMineBySpace(ctx context.Context, actor *domain.User, spaceID string) ([]LakeWithRole, error) {
	return s.listMineFull(ctx, actor, spaceID)
}

// listMineFull 内部实现，spaceFilter="" 表示不过滤。
func (s *LakeService) listMineFull(ctx context.Context, actor *domain.User, spaceFilter string) ([]LakeWithRole, error) {
	ms, err := s.memberships.ListLakesByUserWithRole(ctx, actor.ID)
	if err != nil {
		return nil, err
	}
	if len(ms) == 0 {
		return []LakeWithRole{}, nil
	}
	ids := make([]string, 0, len(ms))
	for _, m := range ms {
		ids = append(ids, m.LakeID)
	}
	lakes, err := s.lakes.GetManyByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*domain.Lake, len(lakes))
	for i := range lakes {
		byID[lakes[i].ID] = &lakes[i]
	}
	// 按 membership 顺序（updated_at desc）输出，跳过缺失的 + 按 spaceFilter 过滤
	out := make([]LakeWithRole, 0, len(ms))
	for _, m := range ms {
		l, ok := byID[m.LakeID]
		if !ok {
			continue
		}
		// spaceFilter 语义：
		//   ""        → 不过滤，全部返回（原 ListMineFull 行为）
		//   "<id>"    → 仅返回 SpaceID == 该 id 的湖
		// 调用方若想"只看个人湖"应单独走另一个 ListMinePersonal（暂未提供，按需扩展）
		if spaceFilter != "" && l.SpaceID != spaceFilter {
			continue
		}
		out = append(out, LakeWithRole{Lake: l, Role: m.Role})
	}
	return out, nil
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
