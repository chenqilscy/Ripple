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
	revisions   store.NodeRevisionRepository // 可空：未接入时 Create/UpdateContent 不写历史
	memberships store.MembershipRepository
	lakes       store.LakeRepository
	broker      realtime.Broker // 可空（单测场景）
}

// NewNodeService 装配。broker / revisions 可为 nil。
func NewNodeService(
	nodes store.NodeRepository,
	memberships store.MembershipRepository,
	lakes store.LakeRepository,
	broker realtime.Broker,
) *NodeService {
	return &NodeService{nodes: nodes, memberships: memberships, lakes: lakes, broker: broker}
}

// WithRevisions 注入编辑历史仓库（装配阶段可选）。返回自身便于链式。
func (s *NodeService) WithRevisions(revs store.NodeRevisionRepository) *NodeService {
	s.revisions = revs
	return s
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
	// 初版 revision：rev_number=1。revisions 未注入时静默跳过（M2 前行为兼容）。
	s.recordRevision(ctx, n, actor.ID, "initial")
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
// 权限：（节点 owner 或 节点原 lake 有写权限）AND 目标 lake 有写权限。
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
	// 来源校验：必须是节点 owner 或对节点原 lake 有写权限，
	// 防止攻击者把别人的 MIST 节点夺到自己的湖。
	if n.OwnerID != actor.ID {
		if n.LakeID == "" {
			return nil, domain.ErrPermissionDenied
		}
		if err := s.requireWrite(ctx, actor, n.LakeID); err != nil {
			return nil, err
		}
	}
	// 目标校验：actor 在 target lake 必须有写权限。
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

// --- 编辑历史（M2-F3）---

// UpdateContentInput 更新节点内容入参。
type UpdateContentInput struct {
	NodeID     string
	Content    string
	EditReason string // 可选：审计注释
}

// UpdateContent 更新节点 content 并追加一条 revision。
// 权限沿用 assertCanMutateNode（owner 或 lake owner）。
// 若 revisions 未注入，仅更新 content，不记录历史。
func (s *NodeService) UpdateContent(ctx context.Context, actor *domain.User, in UpdateContentInput) (*domain.Node, error) {
	if in.NodeID == "" {
		return nil, fmt.Errorf("%w: node_id required", domain.ErrInvalidInput)
	}
	if in.Content == "" {
		return nil, fmt.Errorf("%w: content required", domain.ErrInvalidInput)
	}
	n, err := s.nodes.GetByID(ctx, in.NodeID)
	if err != nil {
		return nil, err
	}
	if err := s.assertCanMutateNode(ctx, actor, n); err != nil {
		return nil, err
	}
	// 内容未变化 → 幂等：不写 revision，不广播事件。
	if n.Content == in.Content {
		return n, nil
	}
	n.Content = in.Content
	n.UpdatedAt = time.Now().UTC()
	if err := s.nodes.UpdateContent(ctx, n); err != nil {
		return nil, err
	}
	s.recordRevision(ctx, n, actor.ID, in.EditReason)
	s.publish(ctx, n.LakeID, "node.updated", n)
	return n, nil
}

// ListRevisions 按时间倒序取节点历史。limit<=0 默认 50；limit>200 截断 200。
func (s *NodeService) ListRevisions(ctx context.Context, actor *domain.User, nodeID string, limit int) ([]domain.NodeRevision, error) {
	if s.revisions == nil {
		return []domain.NodeRevision{}, nil
	}
	n, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	// 读权限：节点所在湖可读即可（不要求写权限，历史透明）。
	if n.LakeID != "" {
		if err := s.assertReadable(ctx, actor, n.LakeID); err != nil {
			return nil, err
		}
	} else if n.OwnerID != actor.ID {
		return nil, domain.ErrPermissionDenied
	}
	return s.revisions.ListByNode(ctx, nodeID, limit)
}

// GetRevision 取单条 revision。权限同 ListRevisions。
func (s *NodeService) GetRevision(ctx context.Context, actor *domain.User, nodeID string, revNumber int) (*domain.NodeRevision, error) {
	if s.revisions == nil {
		return nil, domain.ErrNotFound
	}
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
	return s.revisions.GetByNodeAndRev(ctx, nodeID, revNumber)
}

// Rollback 把节点回滚到指定 revision。等价于 UpdateContent(target.content, reason="rollback to N")。
// 权限同 UpdateContent。
func (s *NodeService) Rollback(ctx context.Context, actor *domain.User, nodeID string, targetRev int) (*domain.Node, error) {
	if s.revisions == nil {
		return nil, fmt.Errorf("%w: revisions not enabled", domain.ErrInvalidInput)
	}
	if targetRev <= 0 {
		return nil, fmt.Errorf("%w: target_rev_number must be > 0", domain.ErrInvalidInput)
	}
	target, err := s.revisions.GetByNodeAndRev(ctx, nodeID, targetRev)
	if err != nil {
		return nil, err
	}
	return s.UpdateContent(ctx, actor, UpdateContentInput{
		NodeID:     nodeID,
		Content:    target.Content,
		EditReason: fmt.Sprintf("rollback to rev %d", targetRev),
	})
}

// recordRevision 尝试追加 revision；失败只记日志不影响主流程（revisions 是审计数据，不应阻塞写）。
// revisions 未注入时（单测 / 尚未装配）静默跳过。
func (s *NodeService) recordRevision(ctx context.Context, n *domain.Node, editorID, reason string) {
	if s.revisions == nil {
		return
	}
	rev := &domain.NodeRevision{
		ID:         platform.NewID(),
		NodeID:     n.ID,
		Content:    n.Content,
		Title:      "",
		EditorID:   editorID,
		EditReason: reason,
		CreatedAt:  n.UpdatedAt,
	}
	if err := s.revisions.InsertNext(ctx, rev); err != nil {
		// 审计数据丢失不阻塞主流程，但要可观测。
		fmt.Printf("[warn] record node revision failed: node=%s err=%v\n", n.ID, err)
	}
}

// --- 内部权限工具 ---

// RequireWrite 校验 actor 对指定湖有 PASSENGER 以上写权限（P8-C doc_state 外部调用）。
func (s *NodeService) RequireWrite(ctx context.Context, actor *domain.User, lakeID string) error {
	return s.requireWrite(ctx, actor, lakeID)
}

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
