package store

import (
	"context"
	"errors"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Attachment 附件元数据。
type Attachment struct {
	ID        string
	UserID    string
	OrgID     string
	NodeID    string // 可空
	MIME      string
	SizeBytes int64
	FilePath  string
	SHA256    string
	CreatedAt time.Time
}

// AttachmentRepository M4-B 附件元数据存储。
type AttachmentRepository interface {
	Insert(ctx context.Context, a *Attachment) error
	GetByID(ctx context.Context, id string) (*Attachment, error)
	GetBySHA(ctx context.Context, userID, sha string) (*Attachment, error)
	ListByNode(ctx context.Context, nodeID string) ([]Attachment, error)
	Delete(ctx context.Context, id string) error
}

type attachmentRepoPG struct{ pool *pgxpool.Pool }

// NewAttachmentRepository 构造 PG 实现。
func NewAttachmentRepository(pool *pgxpool.Pool) AttachmentRepository {
	return &attachmentRepoPG{pool: pool}
}

func (r *attachmentRepoPG) Insert(ctx context.Context, a *Attachment) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO attachments (id, user_id, org_id, node_id, mime, size_bytes, file_path, sha256, created_at)
		VALUES ($1, $2, $3, NULLIF($4,''), $5, $6, $7, $8, $9)`,
		a.ID, a.UserID, a.OrgID, a.NodeID, a.MIME, a.SizeBytes, a.FilePath, a.SHA256, a.CreatedAt,
	)
	return err
}

func (r *attachmentRepoPG) GetByID(ctx context.Context, id string) (*Attachment, error) {
	a := Attachment{}
	var nodeID *string
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, coalesce(org_id, '') AS org_id, node_id, mime, size_bytes, file_path, sha256, created_at
		FROM attachments WHERE id = $1`, id).
		Scan(&a.ID, &a.UserID, &a.OrgID, &nodeID, &a.MIME, &a.SizeBytes, &a.FilePath, &a.SHA256, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if nodeID != nil {
		a.NodeID = *nodeID
	}
	return &a, nil
}

func (r *attachmentRepoPG) GetBySHA(ctx context.Context, userID, sha string) (*Attachment, error) {
	a := Attachment{}
	var nodeID *string
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, coalesce(org_id, '') AS org_id, node_id, mime, size_bytes, file_path, sha256, created_at
		FROM attachments WHERE user_id = $1 AND sha256 = $2`, userID, sha).
		Scan(&a.ID, &a.UserID, &a.OrgID, &nodeID, &a.MIME, &a.SizeBytes, &a.FilePath, &a.SHA256, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if nodeID != nil {
		a.NodeID = *nodeID
	}
	return &a, nil
}

func (r *attachmentRepoPG) ListByNode(ctx context.Context, nodeID string) ([]Attachment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, coalesce(org_id, '') AS org_id, node_id, mime, size_bytes, file_path, sha256, created_at
		FROM attachments WHERE node_id = $1 ORDER BY created_at DESC`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attachment
	for rows.Next() {
		a := Attachment{}
		var nID *string
		if err := rows.Scan(&a.ID, &a.UserID, &a.OrgID, &nID, &a.MIME, &a.SizeBytes, &a.FilePath, &a.SHA256, &a.CreatedAt); err != nil {
			return nil, err
		}
		if nID != nil {
			a.NodeID = *nID
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *attachmentRepoPG) CountByOrg(ctx context.Context, orgID string) (int64, error) {
	var n int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM attachments WHERE org_id = $1`, orgID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *attachmentRepoPG) CountByOrgIDs(ctx context.Context, orgIDs []string) (map[string]int64, error) {
	out := make(map[string]int64, len(orgIDs))
	if len(orgIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
SELECT org_id, COUNT(*)
FROM attachments
WHERE org_id = ANY($1::text[])
GROUP BY org_id
`, orgIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var orgID string
		var count int64
		if err := rows.Scan(&orgID, &count); err != nil {
			return nil, err
		}
		out[orgID] = count
	}
	return out, rows.Err()
}

func (r *attachmentRepoPG) SumSizeByOrg(ctx context.Context, orgID string) (int64, error) {
	var n int64
	if err := r.pool.QueryRow(ctx, `SELECT coalesce(SUM(size_bytes), 0) FROM attachments WHERE org_id = $1`, orgID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *attachmentRepoPG) SumSizeByOrgIDs(ctx context.Context, orgIDs []string) (map[string]int64, error) {
	out := make(map[string]int64, len(orgIDs))
	if len(orgIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
SELECT org_id, coalesce(SUM(size_bytes), 0)
FROM attachments
WHERE org_id = ANY($1::text[])
GROUP BY org_id
`, orgIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var orgID string
		var size int64
		if err := rows.Scan(&orgID, &size); err != nil {
			return nil, err
		}
		out[orgID] = size
	}
	return out, rows.Err()
}

func (r *attachmentRepoPG) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM attachments WHERE id = $1`, id)
	return err
}
