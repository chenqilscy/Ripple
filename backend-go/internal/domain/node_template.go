package domain

import "time"

// NodeTemplate 是节点内容模板。
// 存于 PG（template_nodes 表）。
type NodeTemplate struct {
	ID          string
	Name        string
	Description string
	Content     string
	Tags        []string
	CreatedBy   string // user UUID
	IsSystem    bool   // 内置模板；不可删除
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
