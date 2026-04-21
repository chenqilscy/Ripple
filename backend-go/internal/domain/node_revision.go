package domain

import "time"

// NodeRevision 节点一次内容变更的完整快照。
// 存于 PG 而非 Neo4j：revision 是审计/回溯数据，以范围分页查询为主，关系型存储更合适。
type NodeRevision struct {
	ID         string
	NodeID     string
	RevNumber  int // 在单节点内单调递增（从 1 开始）
	Content    string
	Title      string
	EditorID   string
	EditReason string
	CreatedAt  time.Time
}
