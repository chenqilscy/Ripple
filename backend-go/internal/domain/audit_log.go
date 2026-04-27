package domain

import "time"

// AuditLog 记录一次有意义的资源变更操作（P10-B）。
type AuditLog struct {
	ID           string
	ActorID      string         // 操作者 user_id
	Action       string         // node.create / node.delete / edge.create / lake_member.update ...
	ResourceType string         // node / edge / lake_member / lake / api_key
	ResourceID   string         // 受影响资源的 ID
	Detail       map[string]any // 操作相关补充信息，如 {"prev_state":"MIST","next_state":"DROP"}
	CreatedAt    time.Time
}

// 预定义 Action 常量，避免字符串硬编码。
const (
	AuditNodeCreate       = "node.create"
	AuditNodeDelete       = "node.delete"
	AuditNodeUpdate       = "node.update"
	AuditNodeCondense     = "node.condense"
	AuditNodeEvaporate    = "node.evaporate"
	AuditEdgeCreate       = "edge.create"
	AuditEdgeDelete       = "edge.delete"
	AuditLakeMemberAdd    = "lake_member.add"
	AuditLakeMemberUpdate = "lake_member.update"
	AuditLakeMemberRemove = "lake_member.remove"
	AuditAPIKeyCreate     = "api_key.create"
	AuditAPIKeyRevoke     = "api_key.revoke"
	AuditOrgQuotaUpdate   = "org_quota.update"
)
