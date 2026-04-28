package domain

import "time"

// OrgRole 是用户在组织内的角色。
type OrgRole string

const (
	OrgRoleOwner  OrgRole = "OWNER"
	OrgRoleAdmin  OrgRole = "ADMIN"
	OrgRoleMember OrgRole = "MEMBER"
)

// Rank 返回权限数值，越高权限越大。
func (r OrgRole) Rank() int {
	switch r {
	case OrgRoleOwner:
		return 2
	case OrgRoleAdmin:
		return 1
	case OrgRoleMember:
		return 0
	default:
		return -1
	}
}

// AtLeast 判断当前角色是否 >= 目标角色。
func (r OrgRole) AtLeast(min OrgRole) bool {
	return r.Rank() >= min.Rank()
}

// IsValid 校验角色字符串是否合法。
func (r OrgRole) IsValid() bool {
	return r.Rank() >= 0
}

// Organization 是多租户组织（P12-C）。
// 组织数据存于 PG；向后兼容，不修改已有 Lake/Space 表。
type Organization struct {
	ID          string
	Name        string
	Slug        string // lowercase alphanumeric+hyphen, 3-40 chars
	Description string
	OwnerID     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// OrgMember 组织成员。
type OrgMember struct {
	OrgID    string
	UserID   string
	Role     OrgRole
	JoinedAt time.Time
}

// OrgUsage 组织的真实资源用量（Phase 16）。
type OrgUsage struct {
	Members int64 `json:"members"` // 当前成员数（含 OWNER）
	Lakes   int64 `json:"lakes"`   // 当前归属该组织的湖数
	Nodes   int64 `json:"nodes"`   // 当前归属该组织湖中的非已删除节点数
}

