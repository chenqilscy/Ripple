package domain

import "time"

// APIKey 是用于服务端鉴权的长效 token（P10-A）。
// 原始密钥格式：rpl_<prefix>.<secret>
// - prefix（16 hex chars）：明文存储，用于快速前缀查找。
// - secret（64 hex chars）：SHA-256(key_salt + secret) 后存储，只在创建时返回一次。
type APIKey struct {
	ID         string
	OwnerID    string
	OrgID      string
	Name       string
	KeyPrefix  string   // 16 hex chars，明文存储（用于 DB 查找）
	KeyHash    string   // hex(SHA-256(KeySalt + rawSecret))
	KeySalt    string   // 32 hex chars（16 random bytes）
	Scopes     []string // e.g. ["read_lake","write_node"]
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

// IsValid 判断 key 是否有效（未撤销、未过期）。
func (k *APIKey) IsValid() bool {
	if k.RevokedAt != nil {
		return false
	}
	if k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt) {
		return false
	}
	return true
}

// HasScope 检查 key 是否拥有指定 scope。
// scope="*" 表示全权限（手动授予超级 key 时使用）。
func (k *APIKey) HasScope(scope string) bool {
	for _, s := range k.Scopes {
		if s == scope || s == "*" {
			return true
		}
	}
	return false
}
