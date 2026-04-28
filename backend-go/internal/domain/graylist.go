package domain

import "time"

// GraylistEntry 是灰度准入邮箱名单项。
type GraylistEntry struct {
	ID        string
	Email     string
	Note      string
	CreatedBy string
	CreatedAt time.Time
}