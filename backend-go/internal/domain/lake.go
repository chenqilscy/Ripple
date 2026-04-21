package domain

import "time"

// Role 是用户在某个湖泊里的角色。
// 顺序按照权限从高到低；rank 用于比较。
type Role string

const (
	RoleOwner      Role = "OWNER"
	RoleNavigator  Role = "NAVIGATOR"
	RolePassenger  Role = "PASSENGER"
	RoleObserver   Role = "OBSERVER"
)

// Rank 返回权限数值，越高权限越大。
func (r Role) Rank() int {
	switch r {
	case RoleOwner:
		return 3
	case RoleNavigator:
		return 2
	case RolePassenger:
		return 1
	case RoleObserver:
		return 0
	default:
		return -1
	}
}

// AtLeast 判断当前角色是否 >= 目标角色。
func (r Role) AtLeast(min Role) bool {
	return r.Rank() >= min.Rank()
}

// IsValid 校验角色字符串是否合法。
func (r Role) IsValid() bool {
	return r.Rank() >= 0
}

// Lake 是节点的容器。Lake 实体存于 Neo4j；成员关系存于 PG。
type Lake struct {
	ID          string
	Name        string
	Description string
	IsPublic    bool
	OwnerID     string // user UUID 字符串
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// LakeMembership 表示某用户对某湖的角色。
type LakeMembership struct {
	UserID    string
	LakeID    string
	Role      Role
	CreatedAt time.Time
	UpdatedAt time.Time
}
