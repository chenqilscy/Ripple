package domain

import "time"

// User 是系统中的人。
// ID 用 string 存储 UUID v4（实际生成在 platform/ids.go）。
type User struct {
	ID           string
	Email        string
	PasswordHash string
	DisplayName  string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
