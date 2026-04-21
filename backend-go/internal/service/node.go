package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/realtime"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// VaporTTL 节点蒸发后保留时长（30 天，对齐设计文档）。
const VaporTTL = 30 * 24 * time.Hour

// NodeService 节点 CRUD + 状态机 + 广播。
type NodeService struct {
	nodes       store.NodeRepository
	memberships store.MembershipRepository
	lakes       store.LakeRepository
	broker      realtime.Broker // 可空（单测场景）
}

// NewNodeService 装配。broker 可为 nil，此时所有事件静默。
func NewNodeService(
	nodes store.NodeRepository,
	memberships store.MembershipRepository,
	lakes store.LakeRepository,
	broker realtime.Broker,
) *NodeService {
	return &NodeService{nodes: nodes, memberships: memberships, lakes: lakes, broker: broker}
}

// publish 非阻塞广播；broker 为 nil 时跳过。
func (s *NodeService) publish(ctx context.Context, lakeID, eventType string, n *domain.Node) {
	if s.broker == nil || lakeID == "" {
		return
	}
	_ = s.broker.Publish(ctx, realtime.LakeTopic(lakeID), realtime.Message{
		Type: eventType,
		Payload: map[string]any{
			"node_id":  n.ID,
			"lake_id":  n.LakeID,
			"state":    string(n.State),
			"owner_id": n.OwnerID,
		},
	})
}

// CreateNodeInput 创建节点入参。
type CreateNodeInput struct {
	LakeID   string // 必填：直接归湖落 DROP。MIST 由"造云"接口产出。
	Content  string
	Type     domain.NodeType
	Position *domain.Position
}

// Create 创建一个 DROP 节点。要求 actor 在 lake 内 >= Passenger。
func (s *NodeService) Create(ctx context.Context, actor *domain.User, in CreateNodeInput) (*domain.Node, error) {
	if in.LakeID == "" {
		return nil, fmt.Errorf("%w: lake_id required", domain.ErrInvalidInput)
	}
	if in.Content == "" {
		return nil, fmt.Errorf("%w: content required", domain.ErrInvalidInput)
	}
	if !in.Type.IsValid() {
		return nil, fmt.Errorf("%w: invalid type", domain.ErrInvalidInput)
	}
	if err := s.requireWrite(ctx, actor, in.LakeID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	n := &domain.Node{
		ID:        platform.NewID(),
		LakeID:    in.LakeID,
		OwnerID:   actor.ID,
		Content:   in.Content,
		Type:      in.Type,
		State:     domain.StateDrop,
		Position:  in.Position,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.nodes.Create(ctx, n); err != nil {
		return nil, err
	}
	s.publish(ctx, n.LakeID, "node.created", n)
	return n, nil
}

// Get 读节点。校验：湖必须可访问（私有湖需成员）。
func (s *NodeService) Get(ctx context.Context, actor *domain.User, nodeID string) (*domain.Node, error) {
	n, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if n.LakeID != "" {
		if err := s.assertReadable(ctx, actor, n.LakeID); err != nil {
			return nil, err
		}
	} else if n.OwnerID != actor.ID {
		return nil, domain.ErrPermissionDenied
	}
	return n, nil
}

// ListByLake 列出湖中的节点（默认隐藏 VAPOR）。
func (s *NodeService) ListByLake(ctx context.Context, actor *domain.User, lakeID string, includeVapor bool) ([]domain.Node, error) {
	if err := s.assertReadable(ctx, actor, lakeID); err != nil {
		return nil, err
	}
	return s.nodes.ListByLake(ctx, lakeID, includeVapor)
}

// Evaporate 蒸发节点。权限：节点 owner 或 lake owner（审查 L4-01）。
func (s *NodeService) Evaporate(ctx context.Context, actor *domain.User, nodeID string) (*domain.Node, error) {
	n, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if err := s.assertCanMutateNode(ctx, actor, n); err != nil {
		return nil, err
	}
	if err := n.Evaporate(time.Now().UTC(), VaporTTL); err != nil {
		return nil, err
	}
	if err := s.nodes.UpdateState(ctx, n); err != nil {
		return nil, err
	}
	s.publish(ctx, n.LakeID, "node.evaporated", n)
	return n, nil
}

// Condense 把 MIST 节点凝露到 lake：MIST→DROP，落归属。
// 权限：lake 的 EDITOR/OWNER（与 Create 一致——往湖里塞东西需要写权限）。
// 若 input.LakeID == ""，沿用节点当前 LakeID（造云时 weaver 已写）。
func (s *NodeService) Condense(ctx context.Context, actor *domain.User, nodeID string, targetLakeID string) (*domain.Node, error) {
	n, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	target := targetLakeID
	if target == "" {
		target = n.LakeID
	}
	if target == "" {
		return nil, domain.ErrInvalidInput
	}
	// 权限：目标 lake 的 EDITOR/OWNER
	if err := s.requireWrite(ctx, actor, target); err != nil {
		return nil, err
	}
	if err := n.Condense(time.Now().UTC(), target); err != nil {
		return nil, err
	}
	if err := s.nodes.UpdateState(ctx, n); err != nil {
		return nil, err
	}
	s.publish(ctx, n.LakeID, "node.condensed", n)
	return n, nil
}

// Restore 还原 VAPOR 回 DROP。同样要求 owner 或 lake owner。
func (s *NodeService) Restore(ctx context.Context, actor *domain.User, nodeID string) (*domain.Node, error) {
	n, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if err := s.assertCanMutateNode(ctx, actor, n); err != nil {
		return nil, err
	}
	if err := n.Restore(time.Now().UTC()); err != nil {
		return nil, err
	}
	if err := s.nodes.UpdateState(ctx, n); err != nil {
		return nil, err
	}
	s.publish(ctx, n.LakeID, "node.restored", n)
	return n, nil
}

// --- 内部权限工具 ---

func (s *NodeService) requireWrite(ctx context.Context, actor *domain.User, lakeID string) error {
	role, err := s.memberships.GetRole(ctx, actor.ID, lakeID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrPermissionDenied
		}
		return err
	}
	if !role.AtLeast(domain.RolePassenger) {
		return domain.ErrPermissionDenied
	}
	return nil
}

// assertReadable 私有湖必须有成员；公开湖任何人可读。
func (s *NodeService) assertReadable(ctx context.Context, actor *domain.User, lakeID string) error {
	l, err := s.lakes.GetByID(ctx, lakeID)
	if err != nil {
		return err
	}
	if l.IsPublic {
		return nil
	}
	if _, err := s.memberships.GetRole(ctx, actor.ID, lakeID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrPermissionDenied
		}
		return err
	}
	return nil
}

// assertCanMutateNode 节点 owner 本人 OR 该湖的 OWNER 可修改。
func (s *NodeService) assertCanMutateNode(ctx context.Context, actor *domain.User, n *domain.Node) error {
	if n.OwnerID == actor.ID {
		return nil
	}
	if n.LakeID == "" {
		return domain.ErrPermissionDenied
	}
	role, err := s.memberships.GetRole(ctx, actor.ID, n.LakeID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrPermissionDenied
		}
		return err
	}
	if role != domain.RoleOwner {
		return domain.ErrPermissionDenied
	}
	return nil
}
