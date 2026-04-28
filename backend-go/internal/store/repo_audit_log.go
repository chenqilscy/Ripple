package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ─────────────────────────────────────────────────────────────────────────────
// Interface
// ─────────────────────────────────────────────────────────────────────────────

// AuditLogRepository P10-B 审计日志仓库。
type AuditLogRepository interface {
	// Write 异步写入（建议调用方在 goroutine 内调用，不阻塞响应）。
	Write(ctx context.Context, log *domain.AuditLog) error
	// ListByResource 查询特定资源的审计记录，倒序时间，最多 limit 条。
	ListByResource(ctx context.Context, resourceType, resourceID string, limit int) ([]*domain.AuditLog, error)
	// PruneOlderThan 删除 cutoff 之前的记录（启动时调用，保留 30 天）。
	PruneOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// PG implementation
// ─────────────────────────────────────────────────────────────────────────────

type auditLogRepoPG struct{ pool *pgxpool.Pool }

// NewAuditLogRepository 创建 PG 审计日志仓库。
func NewAuditLogRepository(pool *pgxpool.Pool) AuditLogRepository {
	return &auditLogRepoPG{pool: pool}
}

const sqlInsertAuditLog = `
INSERT INTO audit_logs (id, actor_id, action, resource_type, resource_id, detail, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

func (r *auditLogRepoPG) Write(ctx context.Context, log *domain.AuditLog) error {
	detailJSON, err := json.Marshal(log.Detail)
	if err != nil {
		detailJSON = []byte("{}")
	}
	_, err = r.pool.Exec(ctx, sqlInsertAuditLog,
		log.ID, log.ActorID, log.Action,
		log.ResourceType, log.ResourceID,
		detailJSON, log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("audit_log write: %w", err)
	}
	return nil
}

const sqlListAuditLogsByResource = `
SELECT id, actor_id, action, resource_type, resource_id, detail, created_at
FROM   audit_logs
WHERE  resource_type = $1 AND resource_id = $2
ORDER  BY created_at DESC
LIMIT  $3
`

func (r *auditLogRepoPG) ListByResource(ctx context.Context, resourceType, resourceID string, limit int) ([]*domain.AuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, sqlListAuditLogsByResource, resourceType, resourceID, limit)
	if err != nil {
		return nil, fmt.Errorf("audit_log list: %w", err)
	}
	defer rows.Close()
	var out []*domain.AuditLog
	for rows.Next() {
		l := &domain.AuditLog{}
		var detailJSON []byte
		if err := rows.Scan(&l.ID, &l.ActorID, &l.Action, &l.ResourceType, &l.ResourceID, &detailJSON, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("audit_log scan: %w", err)
		}
		if err := json.Unmarshal(detailJSON, &l.Detail); err != nil {
			l.Detail = map[string]any{}
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

const sqlListLatestAuditLogsByResources = `
SELECT id, actor_id, action, resource_type, resource_id, detail, created_at
FROM (
  SELECT id, actor_id, action, resource_type, resource_id, detail, created_at,
         row_number() OVER (PARTITION BY resource_id ORDER BY created_at DESC) AS rn
  FROM audit_logs
  WHERE resource_type = $1 AND resource_id = ANY($2::text[])
) ranked
WHERE rn <= $3
ORDER BY resource_id ASC, created_at DESC
`

func (r *auditLogRepoPG) ListLatestByResources(ctx context.Context, resourceType string, resourceIDs []string, limitPerResource int) (map[string][]*domain.AuditLog, error) {
	out := make(map[string][]*domain.AuditLog, len(resourceIDs))
	if len(resourceIDs) == 0 || limitPerResource <= 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, sqlListLatestAuditLogsByResources, resourceType, resourceIDs, limitPerResource)
	if err != nil {
		return nil, fmt.Errorf("audit_log list latest resources: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		l := &domain.AuditLog{}
		if err := rows.Scan(&l.ID, &l.ActorID, &l.Action, &l.ResourceType, &l.ResourceID, &l.Detail, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("audit_log latest scan: %w", err)
		}
		out[l.ResourceID] = append(out[l.ResourceID], l)
	}
	return out, rows.Err()
}

const sqlPruneAuditLogs = `
DELETE FROM audit_logs WHERE created_at < $1
`

func (r *auditLogRepoPG) PruneOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, sqlPruneAuditLogs, cutoff)
	if err != nil {
		return 0, fmt.Errorf("audit_log prune: %w", err)
	}
	return tag.RowsAffected(), nil
}
