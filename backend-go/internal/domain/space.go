// Package domain · Space（M3 工作空间）领域模型。
//
// Space 与 Lake 的区别：
//   - Lake：知识容器（节点+边的图），存 Neo4j
//   - Space：组织/团队工作空间，承载多个 Lake，管 LLM 配额（M3-S2 起）
//
// 角色独立于 Lake 角色：Space.OWNER / EDITOR / VIEWER。
package domain

import "time"

// SpaceRole 用户在 Space 内的角色。
type SpaceRole string

const (
	SpaceRoleOwner  SpaceRole = "OWNER"
	SpaceRoleEditor SpaceRole = "EDITOR"
	SpaceRoleViewer SpaceRole = "VIEWER"
)

// Rank 数值越大权限越大。
func (r SpaceRole) Rank() int {
	switch r {
	case SpaceRoleOwner:
		return 2
	case SpaceRoleEditor:
		return 1
	case SpaceRoleViewer:
		return 0
	default:
		return -1
	}
}

// AtLeast 判断当前角色是否 >= 目标角色。
func (r SpaceRole) AtLeast(min SpaceRole) bool {
	return r.Rank() >= min.Rank()
}

// IsValid 校验角色字符串是否合法。
func (r SpaceRole) IsValid() bool {
	return r.Rank() >= 0
}

// Space 工作空间。
type Space struct {
	ID                   string
	OwnerID              string
	Name                 string
	Description          string
	LLMQuotaMonthly      int
	LLMUsedCurrentMonth  int
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// SpaceMember 空间成员关系。
type SpaceMember struct {
	SpaceID   string
	UserID    string
	Role      SpaceRole
	JoinedAt  time.Time
	UpdatedAt time.Time
}
