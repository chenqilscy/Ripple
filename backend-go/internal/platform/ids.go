package platform

import "github.com/google/uuid"

// NewID 生成一个新的 UUID v4 字符串。
func NewID() string {
	return uuid.NewString()
}
