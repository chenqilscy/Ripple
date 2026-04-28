package domain

import "time"

type PlatformAdminRole string

const (
	PlatformAdminRoleAdmin PlatformAdminRole = "ADMIN"
	PlatformAdminRoleOwner PlatformAdminRole = "OWNER"
)

func (r PlatformAdminRole) IsValid() bool {
	return r == PlatformAdminRoleAdmin || r == PlatformAdminRoleOwner
}

type PlatformAdmin struct {
	UserID    string
	Role      PlatformAdminRole
	Note      string
	CreatedBy string
	CreatedAt time.Time
	RevokedAt *time.Time
}
