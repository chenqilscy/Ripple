package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ─────────────────────────────────────────────────────────────────────────────
// Interface
// ─────────────────────────────────────────────────────────────────────────────

// APIKeyRepository P10-A API Key 仓库。
type APIKeyRepository interface {
	// Create 持久化新 key（KeyPrefix / KeyHash / KeySalt 由调用方已填充）。
	Create(ctx context.Context, key *domain.APIKey) error
	// GetByPrefix 按前缀查找 key（用于鉴权路径，O(1) 索引查找）。
	GetByPrefix(ctx context.Context, prefix string) (*domain.APIKey, error)
	// ListByOwner 列出 owner 的所有未删除 key（倒序创建时间）。
	ListByOwner(ctx context.Context, ownerID string) ([]*domain.APIKey, error)
	// Revoke 软删除：设置 revoked_at = now()；ownerID 用于防止越权撤销。
	Revoke(ctx context.Context, id, ownerID string) error
	// UpdateLastUsed 异步更新最近使用时间（不影响主请求响应）。
	UpdateLastUsed(ctx context.Context, id string, t time.Time) error
}

// ─────────────────────────────────────────────────────────────────────────────
// Key generation helpers（暴露为包级函数供 handler 调用）
// ─────────────────────────────────────────────────────────────────────────────

// GenerateAPIKey 生成三元组 (rawKey, prefix, salt, hash)。
//
//	rawKey  = "rpl_<prefix>.<secret>"
//	prefix  = 16 hex chars (8 random bytes)，明文存储
//	secret  = 64 hex chars (32 random bytes)，不存储
//	salt    = 32 hex chars (16 random bytes)，明文存储
//	hash    = hex(SHA-256(salt + secret))，存储
func GenerateAPIKey() (rawKey, prefix, salt, hash string, err error) {
	prefixBytes := make([]byte, 8)
	secretBytes := make([]byte, 32)
	saltBytes := make([]byte, 16)

	if _, err = rand.Read(prefixBytes); err != nil {
		return
	}
	if _, err = rand.Read(secretBytes); err != nil {
		return
	}
	if _, err = rand.Read(saltBytes); err != nil {
		return
	}

	prefix = hex.EncodeToString(prefixBytes)
	secret := hex.EncodeToString(secretBytes)
	salt = hex.EncodeToString(saltBytes)
	hash = hashAPIKeySecret(salt, secret)
	rawKey = "rpl_" + prefix + "." + secret
	return
}

// VerifyAPIKey 校验 hash(salt + secret) == storedHash。
// prefix 已由调用方从 rawKey 中提取出来，对应 DB 中的 key_prefix。
// secret 从 rawKey 中提取（点号后的部分）。
func VerifyAPIKey(rawKey, storedSalt, storedHash string) bool {
	// rawKey = "rpl_<prefix>.<secret>"
	const pfxLen = 4 + 16 + 1 // "rpl_" + 16 hex + "."
	if len(rawKey) <= pfxLen {
		return false
	}
	secret := rawKey[pfxLen:]
	return hashAPIKeySecret(storedSalt, secret) == storedHash
}

// ExtractPrefix 从 rawKey 中提取 prefix（不含 "rpl_"）。
func ExtractPrefix(rawKey string) (string, bool) {
	// rawKey = "rpl_XXXXXXXXXXXXXXXX.YYYY..."
	if len(rawKey) < 4+16+1 || rawKey[:4] != "rpl_" {
		return "", false
	}
	prefix := rawKey[4 : 4+16]
	if rawKey[4+16] != '.' {
		return "", false
	}
	return prefix, true
}

func hashAPIKeySecret(salt, secret string) string {
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte(secret))
	return hex.EncodeToString(h.Sum(nil))
}

// ─────────────────────────────────────────────────────────────────────────────
// PG implementation
// ─────────────────────────────────────────────────────────────────────────────

type apiKeyRepoPG struct{ pool *pgxpool.Pool }

// NewAPIKeyRepository 创建 PG API Key 仓库。
func NewAPIKeyRepository(pool *pgxpool.Pool) APIKeyRepository {
	return &apiKeyRepoPG{pool: pool}
}

const sqlInsertAPIKey = `
INSERT INTO api_keys (id, owner_id, org_id, name, key_prefix, key_hash, key_salt, scopes, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
`

func (r *apiKeyRepoPG) Create(ctx context.Context, key *domain.APIKey) error {
	_, err := r.pool.Exec(ctx, sqlInsertAPIKey,
		key.ID, key.OwnerID, key.OrgID, key.Name,
		key.KeyPrefix, key.KeyHash, key.KeySalt,
		key.Scopes, key.ExpiresAt, key.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("api_key create: %w", err)
	}
	return nil
}

const sqlSelectAPIKeyByPrefix = `
SELECT id, owner_id, coalesce(org_id, '') AS org_id, name, key_prefix, key_hash, key_salt,
       scopes, last_used_at, expires_at, revoked_at, created_at
FROM   api_keys
WHERE  key_prefix = $1
`

func (r *apiKeyRepoPG) GetByPrefix(ctx context.Context, prefix string) (*domain.APIKey, error) {
	row := r.pool.QueryRow(ctx, sqlSelectAPIKeyByPrefix, prefix)
	return scanAPIKey(row)
}

const sqlListAPIKeysByOwner = `
SELECT id, owner_id, coalesce(org_id, '') AS org_id, name, key_prefix, key_hash, key_salt,
       scopes, last_used_at, expires_at, revoked_at, created_at
FROM   api_keys
WHERE  owner_id = $1 AND revoked_at IS NULL
ORDER  BY created_at DESC
`

func (r *apiKeyRepoPG) ListByOwner(ctx context.Context, ownerID string) ([]*domain.APIKey, error) {
	rows, err := r.pool.Query(ctx, sqlListAPIKeysByOwner, ownerID)
	if err != nil {
		return nil, fmt.Errorf("api_keys list: %w", err)
	}
	defer rows.Close()
	var out []*domain.APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

const sqlCountAPIKeysByOrg = `
SELECT COUNT(*) FROM api_keys WHERE org_id = $1 AND revoked_at IS NULL
`

func (r *apiKeyRepoPG) CountByOrg(ctx context.Context, orgID string) (int64, error) {
	var n int64
	if err := r.pool.QueryRow(ctx, sqlCountAPIKeysByOrg, orgID).Scan(&n); err != nil {
		return 0, fmt.Errorf("api_keys count by org: %w", err)
	}
	return n, nil
}

const sqlRevokeAPIKey = `
UPDATE api_keys SET revoked_at = now()
WHERE  id = $1 AND owner_id = $2 AND revoked_at IS NULL
`

func (r *apiKeyRepoPG) Revoke(ctx context.Context, id, ownerID string) error {
	tag, err := r.pool.Exec(ctx, sqlRevokeAPIKey, id, ownerID)
	if err != nil {
		return fmt.Errorf("api_key revoke: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

const sqlUpdateLastUsed = `
UPDATE api_keys SET last_used_at = $2 WHERE id = $1
`

func (r *apiKeyRepoPG) UpdateLastUsed(ctx context.Context, id string, t time.Time) error {
	_, err := r.pool.Exec(ctx, sqlUpdateLastUsed, id, t)
	return err
}

// scanAPIKey 从行扫描 *domain.APIKey。
// 接受 pgx.Row 和 pgx.Rows（两者都实现 pgx.Row interface via Scan）。
func scanAPIKey(row interface{ Scan(...any) error }) (*domain.APIKey, error) {
	k := &domain.APIKey{}
	err := row.Scan(
		&k.ID, &k.OwnerID, &k.OrgID, &k.Name,
		&k.KeyPrefix, &k.KeyHash, &k.KeySalt,
		&k.Scopes, &k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt, &k.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("api_key scan: %w", err)
	}
	return k, nil
}
