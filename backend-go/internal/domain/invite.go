package domain

import "time"

// Invite 是加入湖的邀请凭证。
// 存于 PG；token 由 crypto/rand 32B → base64url 编码。
type Invite struct {
	ID         string
	LakeID     string
	Token      string
	CreatedBy  string // user UUID
	Role       Role   // 目标加入角色（不能是 OWNER）
	MaxUses    int
	UsedCount  int
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

// IsAlive 判断邀请是否仍可使用（未撤销 + 未过期 + 余额大于 0）。
func (i *Invite) IsAlive(now time.Time) bool {
	if i.RevokedAt != nil {
		return false
	}
	if now.After(i.ExpiresAt) {
		return false
	}
	if i.UsedCount >= i.MaxUses {
		return false
	}
	return true
}
