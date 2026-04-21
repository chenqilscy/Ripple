package domain

import "time"

// NodeState 是节点生命周期状态。
// 转移图（约束规约 §2.2）：
//
//	MIST   (TTL 7d)  --condense--> DROP
//	DROP             --freeze----> FROZEN
//	DROP/FROZEN      --evaporate-> VAPOR (TTL 30d)
//	VAPOR            --restore---> DROP
//	VAPOR            --expire----> ERASED
//	任何态           --cross-lake-> GHOST（保留跨湖引用）
type NodeState string

const (
	StateMist   NodeState = "MIST"
	StateDrop   NodeState = "DROP"
	StateFrozen NodeState = "FROZEN"
	StateVapor  NodeState = "VAPOR"
	StateErased NodeState = "ERASED"
	StateGhost  NodeState = "GHOST"
)

// IsValid 校验状态字符串。
func (s NodeState) IsValid() bool {
	switch s {
	case StateMist, StateDrop, StateFrozen, StateVapor, StateErased, StateGhost:
		return true
	}
	return false
}

// CanEvaporate 判断当前状态是否允许蒸发。
func (s NodeState) CanEvaporate() bool {
	return s == StateDrop || s == StateFrozen
}

// CanRestore 判断当前状态是否允许还原。
func (s NodeState) CanRestore() bool {
	return s == StateVapor
}

// CanCondense 判断当前状态（一定是 MIST）是否允许凝露。
func (s NodeState) CanCondense() bool {
	return s == StateMist
}

// Position 是节点在湖面 3D 空间中的位置。
type Position struct {
	X, Y, Z float64
}

// NodeType 是节点的内容类型。
type NodeType string

const (
	NodeTypeText  NodeType = "TEXT"
	NodeTypeImage NodeType = "IMAGE"
	NodeTypeLink  NodeType = "LINK"
	NodeTypeAudio NodeType = "AUDIO"
)

// IsValid 校验节点类型是否合法。
func (t NodeType) IsValid() bool {
	switch t {
	case NodeTypeText, NodeTypeImage, NodeTypeLink, NodeTypeAudio:
		return true
	}
	return false
}

// Node 是青萍系统的核心实体（涟漪 / 浮萍）。
// Node 实体存于 Neo4j；不在 PG。
type Node struct {
	ID        string
	LakeID    string // 可空：MIST 状态尚未归湖
	OwnerID   string // user UUID 字符串
	Content   string
	Type      NodeType
	State     NodeState
	Position  *Position // 可空
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time // VAPOR 时填充
	TTLAt     *time.Time // MIST/VAPOR 到期时间
}

// Evaporate 把节点切换到 VAPOR 态。
// 调用前必须由 service 层校验权限。
func (n *Node) Evaporate(now time.Time, ttl time.Duration) error {
	if !n.State.CanEvaporate() {
		return ErrInvalidStateTransition
	}
	n.State = StateVapor
	n.DeletedAt = &now
	exp := now.Add(ttl)
	n.TTLAt = &exp
	n.UpdatedAt = now
	return nil
}

// Restore 把 VAPOR 节点还原回 DROP。
func (n *Node) Restore(now time.Time) error {
	if !n.State.CanRestore() {
		return ErrInvalidStateTransition
	}
	n.State = StateDrop
	n.DeletedAt = nil
	n.TTLAt = nil
	n.UpdatedAt = now
	return nil
}

// Condense 把 MIST 节点凝露到目标湖。
func (n *Node) Condense(now time.Time, lakeID string) error {
	if !n.State.CanCondense() {
		return ErrInvalidStateTransition
	}
	if lakeID == "" {
		return ErrInvalidInput
	}
	n.State = StateDrop
	n.LakeID = lakeID
	n.TTLAt = nil
	n.UpdatedAt = now
	return nil
}
