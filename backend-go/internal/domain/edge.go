// Package domain · Edge 实体（节点之间的有向关系）。
//
// 设计：
//   - 边存在 Neo4j（与节点同库），不入 PG（避免跨库 Saga）。
//   - kind 枚举 + custom 二选一：custom 时 label 必填非空。
//   - 软删（DeletedAt），与节点蒸发语义对齐。
//   - 必须同 lake：跨湖关联在 M2 不支持。
package domain

import "time"

// EdgeKind 节点关系类型。
type EdgeKind string

const (
	EdgeKindRelates EdgeKind = "relates" // 弱关联
	EdgeKindDerives EdgeKind = "derives" // 派生（src→dst 由前者推出）
	EdgeKindOpposes EdgeKind = "opposes" // 反对
	EdgeKindRefines EdgeKind = "refines" // 细化
	EdgeKindGroups     EdgeKind = "groups"     // 归类
	EdgeKindSummarizes EdgeKind = "summarizes" // 摘要（摘要节点→源节点，P20-B）
	EdgeKindCustom     EdgeKind = "custom"     // 自定义（必须配 Label）
)

// IsValid 判断 kind 合法。
func (k EdgeKind) IsValid() bool {
	switch k {
	case EdgeKindRelates, EdgeKindDerives, EdgeKindOpposes,
		EdgeKindRefines, EdgeKindGroups, EdgeKindSummarizes, EdgeKindCustom:
		return true
	}
	return false
}

// Edge 节点之间的有向关系。
type Edge struct {
	ID        string
	LakeID    string // 冗余：方便按湖列表 + 同湖校验
	SrcNodeID string
	DstNodeID string
	Kind      EdgeKind
	Label     string // kind=custom 时必填；其他 kind 可选展示文本
	OwnerID   string
	CreatedAt time.Time
	DeletedAt *time.Time
}
