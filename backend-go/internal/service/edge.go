// Package service · EdgeService
//
// 节点关系管理。权限模型：
//   - Create: actor 必须对 src/dst 所在 lake 有写权限（>=PASSENGER）
//             且 src/dst 必须同一个 lake（repo 层 Cypher 已校验，service 提前快速失败）
//   - Delete: edge.owner 本人 OR lake 写权限
//   - List:   actor 必须对 lake 可读（私有湖需成员）
//
// 实时事件：
//   - edge.created  payload: {edge_id, src, dst, kind, label, owner_id}
//   - edge.deleted  payload: {edge_id, lake_id}
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

// EdgeService 边管理。
type EdgeService struct {
	edges       store.EdgeRepository
	nodes       store.NodeRepository
	memberships store.MembershipRepository
	lakes       store.LakeRepository
	broker      realtime.Broker // 可空（单测）
}

// NewEdgeService 装配。
func NewEdgeService(
	edges store.EdgeRepository,
	nodes store.NodeRepository,
	memberships store.MembershipRepository,
	lakes store.LakeRepository,
	broker realtime.Broker,
) *EdgeService {
	return &EdgeService{edges: edges, nodes: nodes, memberships: memberships, lakes: lakes, broker: broker}
}

// CreateEdgeInput 创建边入参。
type CreateEdgeInput struct {
	SrcNodeID string
	DstNodeID string
	Kind      domain.EdgeKind
	Label     string // kind=custom 时必填
}

// Create 在两个节点之间建一条有向边。
func (s *EdgeService) Create(ctx context.Context, actor *domain.User, in CreateEdgeInput) (*domain.Edge, error) {
	if in.SrcNodeID == "" || in.DstNodeID == "" {
		return nil, fmt.Errorf("%w: src/dst required", domain.ErrInvalidInput)
	}
	if in.SrcNodeID == in.DstNodeID {
		return nil, fmt.Errorf("%w: self loop forbidden", domain.ErrInvalidInput)
	}
	if !in.Kind.IsValid() {
		return nil, fmt.Errorf("%w: invalid kind", domain.ErrInvalidInput)
	}
	if in.Kind == domain.EdgeKindCustom && in.Label == "" {
		return nil, fmt.Errorf("%w: custom edge requires label", domain.ErrInvalidInput)
	}

	// 拉两端节点：校验存在 + 同湖 + 取 lake_id
	src, err := s.nodes.GetByID(ctx, in.SrcNodeID)
	if err != nil {
		return nil, err
	}
	dst, err := s.nodes.GetByID(ctx, in.DstNodeID)
	if err != nil {
		return nil, err
	}
	if src.LakeID == "" || src.LakeID != dst.LakeID {
		return nil, fmt.Errorf("%w: src and dst must be in same lake", domain.ErrInvalidInput)
	}

	// 写权限
	if err := s.requireWrite(ctx, actor, src.LakeID); err != nil {
		return nil, err
	}

	// 重复检测（同 src,dst,kind 且 alive）
	exists, err := s.edges.ExistsAlive(ctx, in.SrcNodeID, in.DstNodeID, in.Kind)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("%w: edge with same kind already exists", domain.ErrAlreadyExists)
	}

	now := time.Now().UTC()
	e := &domain.Edge{
		ID:        platform.NewID(),
		LakeID:    src.LakeID,
		SrcNodeID: in.SrcNodeID,
		DstNodeID: in.DstNodeID,
		Kind:      in.Kind,
		Label:     in.Label,
		OwnerID:   actor.ID,
		CreatedAt: now,
	}
	if err := s.edges.Create(ctx, e); err != nil {
		return nil, err
	}
	s.publishCreated(ctx, e)
	return e, nil
}

// ListByLake 列出湖内的边。
func (s *EdgeService) ListByLake(ctx context.Context, actor *domain.User, lakeID string, includeDeleted bool) ([]domain.Edge, error) {
	if err := s.assertReadable(ctx, actor, lakeID); err != nil {
		return nil, err
	}
	return s.edges.ListByLake(ctx, lakeID, includeDeleted)
}

// Delete 软删边。权限：边的 owner OR 湖写权限。
func (s *EdgeService) Delete(ctx context.Context, actor *domain.User, edgeID string) error {
	e, err := s.edges.GetByID(ctx, edgeID)
	if err != nil {
		return err
	}
	if e.DeletedAt != nil {
		return nil // 幂等：已删则成功
	}
	if e.OwnerID != actor.ID {
		if err := s.requireWrite(ctx, actor, e.LakeID); err != nil {
			return err
		}
	}
	if err := s.edges.SoftDelete(ctx, edgeID, time.Now().UTC()); err != nil {
		// 并发场景：另一请求已 SoftDelete，repo 返回 ErrNotFound（WHERE deleted_at IS NULL 守护）
		// 视为成功（幂等）。
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}
	s.publishDeleted(ctx, e)
	return nil
}

// --- 权限 helpers（与 NodeService 相同语义；不复用是为了避免跨服务耦合）---

func (s *EdgeService) requireWrite(ctx context.Context, actor *domain.User, lakeID string) error {
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

func (s *EdgeService) assertReadable(ctx context.Context, actor *domain.User, lakeID string) error {
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

// --- 广播 helpers ---

func (s *EdgeService) publishCreated(ctx context.Context, e *domain.Edge) {
	if s.broker == nil || e.LakeID == "" {
		return
	}
	_ = s.broker.Publish(ctx, realtime.LakeTopic(e.LakeID), realtime.Message{
		Type: "edge.created",
		Payload: map[string]any{
			"edge_id":  e.ID,
			"lake_id":  e.LakeID,
			"src":      e.SrcNodeID,
			"dst":      e.DstNodeID,
			"kind":     string(e.Kind),
			"label":    e.Label,
			"owner_id": e.OwnerID,
		},
	})
}

func (s *EdgeService) publishDeleted(ctx context.Context, e *domain.Edge) {
	if s.broker == nil || e.LakeID == "" {
		return
	}
	_ = s.broker.Publish(ctx, realtime.LakeTopic(e.LakeID), realtime.Message{
		Type: "edge.deleted",
		Payload: map[string]any{
			"edge_id": e.ID,
			"lake_id": e.LakeID,
		},
	})
}
