package domain

import "time"

// NodeShare 节点只读外链分享（P18-B）。
// 存于 PG（node_shares 表）。
type NodeShare struct {
	ID        string
	NodeID    string
	Token     string    // URL-safe random token（32 字节 base64）
	ExpiresAt time.Time // 零值 = 永不过期
	Revoked   bool
	CreatedBy string
	CreatedAt time.Time
}
