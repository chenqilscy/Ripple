package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	orgs        *OrgService
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

// WithOrgService 注入组织服务，用于组织配额检查（P14.2）。
func (s *NodeService) WithOrgService(orgs *OrgService) *NodeService {
	s.orgs = orgs
	return s
}

type nodeOrgCounter interface {
	CountByOrg(ctx context.Context, orgID string) (int64, error)
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
	if err := s.checkNodeQuota(ctx, in.LakeID, 1); err != nil {
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

// GetPublicByID P18-B：通过 share token 已验证后直接取节点，不做权限检查。
// 仅供 NodeShareHandlers.GetSharedNode 调用。
func (s *NodeService) GetPublicByID(ctx context.Context, nodeID string) (*domain.Node, error) {
	return s.nodes.GetByID(ctx, nodeID)
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
	delta := int64(1)
	if n.LakeID == target {
		delta = 0
	} else if n.LakeID != "" {
		if sourceLake, err := s.lakes.GetByID(ctx, n.LakeID); err == nil {
			if targetLake, err := s.lakes.GetByID(ctx, target); err == nil && sourceLake.OrgID != "" && sourceLake.OrgID == targetLake.OrgID {
				delta = 0
			}
		}
	}
	if err := s.checkNodeQuota(ctx, target, delta); err != nil {
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

// SearchNodes 在指定湖内全文搜索节点（P12-D）。
// 调用方至少是 OBSERVER，或湖为公开湖。
func (s *NodeService) SearchNodes(ctx context.Context, actor *domain.User, lakeID, q string, limit int) ([]domain.NodeSearchResult, error) {
	if err := s.assertReadable(ctx, actor, lakeID); err != nil {
		return nil, err
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	// Escape Lucene special characters to prevent query parse errors.
	q = escapeLuceneQuery(q)
	return s.nodes.Search(ctx, lakeID, q, limit)
}

// SearchNodesFiltered P22：全文搜索 + state/type 过滤。
// state 和 nodeType 为空字符串时不过滤。
func (s *NodeService) SearchNodesFiltered(ctx context.Context, actor *domain.User, lakeID, q, state, nodeType string, limit int) ([]domain.NodeSearchResult, error) {
	if err := s.assertReadable(ctx, actor, lakeID); err != nil {
		return nil, err
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	// 校验可选枚举值（防注入）
	if state != "" && !domain.NodeState(state).IsValid() {
		return nil, domain.ErrInvalidInput
	}
	if nodeType != "" && !domain.NodeType(nodeType).IsValid() {
		return nil, domain.ErrInvalidInput
	}
	q = escapeLuceneQuery(q)
	return s.nodes.SearchFiltered(ctx, lakeID, q, state, nodeType, limit)
}

// FindRelated P18-A：取目标节点所在湖内的相关节点（基于全文索引）。
// 使用节点内容的前 50 字作为关键词查询。
func (s *NodeService) FindRelated(ctx context.Context, actor *domain.User, nodeID string, limit int) ([]domain.NodeSearchResult, error) {
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
	// 取内容前 50 字作为关键词（截取 rune 避免乱码）
	runes := []rune(strings.TrimSpace(n.Content))
	if len(runes) > 50 {
		runes = runes[:50]
	}
	keyword := escapeLuceneQuery(string(runes))
	if keyword == "" {
		return []domain.NodeSearchResult{}, nil
	}
	return s.nodes.FindRelated(ctx, nodeID, n.LakeID, keyword, limit)
}

// escapeLuceneQuery 转义 Lucene 查询特殊字符，防止解析错误。
// 受影响字符：+ - && || ! ( ) { } [ ] ^ " ~ * ? : \ /
func escapeLuceneQuery(q string) string {
	const special = `+-&|!(){}[]^"~*?:/\`
	var buf strings.Builder
	buf.Grow(len(q) * 2)
	for _, c := range q {
		if strings.ContainsRune(special, c) {
			buf.WriteByte('\\')
		}
		buf.WriteRune(c)
	}
	return buf.String()
}

// --- 批量导入（P12-A）---

const maxBatchImport = 1000
const maxNodeContentRunes = 10000

// BatchImportItem 单个批量导入项。
type BatchImportItem struct {
	Content string
	Type    domain.NodeType
}

// BatchImportResult 批量导入结果。
type BatchImportResult struct {
	Created int
	Nodes   []*domain.Node
}

// BatchImportNodes 批量创建节点到指定湖（P12-A）。
// 权限：至少 NAVIGATOR（批量写操作不对 PASSENGER 开放）。
// 空 content 节点自动跳过；content 超长时截断至 10000 字符。
func (s *NodeService) BatchImportNodes(ctx context.Context, actor *domain.User, lakeID string, items []BatchImportItem) (*BatchImportResult, error) {
	if lakeID == "" {
		return nil, fmt.Errorf("%w: lake_id required", domain.ErrInvalidInput)
	}
	role, err := s.memberships.GetRole(ctx, actor.ID, lakeID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrPermissionDenied
		}
		return nil, err
	}
	if !role.AtLeast(domain.RoleNavigator) {
		return nil, domain.ErrPermissionDenied
	}
	if len(items) == 0 {
		return &BatchImportResult{}, nil
	}
	if len(items) > maxBatchImport {
		return nil, fmt.Errorf("%w: too many nodes (max %d)", domain.ErrInvalidInput, maxBatchImport)
	}

	now := time.Now().UTC()
	nodes := make([]*domain.Node, 0, len(items))
	for _, item := range items {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		runes := []rune(content)
		if len(runes) > maxNodeContentRunes {
			content = string(runes[:maxNodeContentRunes])
		}
		nodeType := item.Type
		if !nodeType.IsValid() {
			nodeType = domain.NodeTypeText
		}
		nodes = append(nodes, &domain.Node{
			ID:        platform.NewID(),
			LakeID:    lakeID,
			OwnerID:   actor.ID,
			Content:   content,
			Type:      nodeType,
			State:     domain.StateDrop,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	if len(nodes) == 0 {
		return &BatchImportResult{}, nil
	}
	if err := s.checkNodeQuota(ctx, lakeID, int64(len(nodes))); err != nil {
		return nil, err
	}
	if err := s.nodes.BatchCreate(ctx, nodes); err != nil {
		return nil, err
	}
	return &BatchImportResult{Created: len(nodes), Nodes: nodes}, nil
}

func (s *NodeService) checkNodeQuota(ctx context.Context, lakeID string, delta int64) error {
	if s.orgs == nil || delta <= 0 {
		return nil
	}
	lake, err := s.lakes.GetByID(ctx, lakeID)
	if err != nil {
		return err
	}
	if lake.OrgID == "" {
		return nil
	}
	counter, ok := s.nodes.(nodeOrgCounter)
	if !ok {
		return fmt.Errorf("%w: node quota counter not configured", domain.ErrInvalidInput)
	}
	used, err := counter.CountByOrg(ctx, lake.OrgID)
	if err != nil {
		return err
	}
	return s.orgs.CheckQuota(ctx, lake.OrgID, domain.OrgQuotaNodes, used, delta)
}

// BatchOperate P14-C：对多个节点执行同一操作（evaporate / condense）。
// node_ids 最多 200 个，且必须全部属于 lakeID（由每个单节点操作内部校验）。
const batchOperateMaxNodes = 200

// BatchOperateResult 批量操作结果。
type BatchOperateResult struct {
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// Erase 手动彻底删除（软删除）一个节点，将其状态设为 ERASED。
// 权限：节点 owner 本人 OR 湖 NAVIGATOR+（管理员级别可清理他人节点）。
func (s *NodeService) Erase(ctx context.Context, actor *domain.User, nodeID string) (*domain.Node, error) {
	n, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	// 权限：节点 owner 本人，或 lake 成员 NAVIGATOR+
	if n.OwnerID != actor.ID {
		if n.LakeID == "" {
			return nil, domain.ErrPermissionDenied
		}
		role, rerr := s.memberships.GetRole(ctx, actor.ID, n.LakeID)
		if rerr != nil {
			if errors.Is(rerr, domain.ErrNotFound) {
				return nil, domain.ErrPermissionDenied
			}
			return nil, rerr
		}
		if !role.AtLeast(domain.RoleNavigator) {
			return nil, domain.ErrPermissionDenied
		}
	}
	if err := n.Erase(time.Now().UTC()); err != nil {
		return nil, err
	}
	if err := s.nodes.UpdateState(ctx, n); err != nil {
		return nil, err
	}
	s.publish(ctx, n.LakeID, "node.erased", n)
	return n, nil
}

// BatchOperate 对节点列表执行批量操作。支持 "evaporate" / "condense" / "erase"。
func (s *NodeService) BatchOperate(ctx context.Context, actor *domain.User, lakeID, action string, nodeIDs []string) (*BatchOperateResult, error) {
	if len(nodeIDs) == 0 {
		return &BatchOperateResult{}, nil
	}
	if lakeID == "" {
		return nil, fmt.Errorf("%w: lake_id required", domain.ErrInvalidInput)
	}
	if len(nodeIDs) > batchOperateMaxNodes {
		return nil, fmt.Errorf("%w: too many nodes in batch (max %d)", domain.ErrInvalidInput, batchOperateMaxNodes)
	}
	if action != "evaporate" && action != "condense" && action != "erase" {
		return nil, fmt.Errorf("%w: action must be evaporate, condense or erase", domain.ErrInvalidInput)
	}
	// 路由是 /lakes/{id}/nodes/batch_op，所有节点必须属于该 lake。
	for _, id := range nodeIDs {
		n, err := s.nodes.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if n.LakeID != lakeID {
			return nil, domain.ErrPermissionDenied
		}
	}
	res := &BatchOperateResult{}
	for _, id := range nodeIDs {
		var err error
		switch action {
		case "evaporate":
			_, err = s.Evaporate(ctx, actor, id)
		case "condense":
			_, err = s.Condense(ctx, actor, id, lakeID)
		case "erase":
			_, err = s.Erase(ctx, actor, id)
		}
		if err != nil {
			res.Failed++
		} else {
			res.Succeeded++
		}
	}
	return res, nil
}
