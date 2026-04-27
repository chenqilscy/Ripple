package domain

import (
	"fmt"
	"time"
)

// OrgQuotaKey 标识组织配额维度。
type OrgQuotaKey string

const (
	OrgQuotaMembers     OrgQuotaKey = "members"
	OrgQuotaLakes       OrgQuotaKey = "lakes"
	OrgQuotaNodes       OrgQuotaKey = "nodes"
	OrgQuotaAttachments OrgQuotaKey = "attachments"
	OrgQuotaAPIKeys     OrgQuotaKey = "api_keys"
	OrgQuotaStorageMB   OrgQuotaKey = "storage_mb"
)

// OrgQuota 是组织级资源配额。0 表示该维度不允许继续新增，不表示无限制。
type OrgQuota struct {
	OrgID          string
	MaxMembers     int64
	MaxLakes       int64
	MaxNodes       int64
	MaxAttachments int64
	MaxAPIKeys     int64
	MaxStorageMB   int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// DefaultOrgQuota 返回免费层默认配额。
func DefaultOrgQuota(orgID string) *OrgQuota {
	now := time.Now().UTC()
	return &OrgQuota{
		OrgID:          orgID,
		MaxMembers:     20,
		MaxLakes:       50,
		MaxNodes:       10000,
		MaxAttachments: 1000,
		MaxAPIKeys:     10,
		MaxStorageMB:   1024,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// LimitFor 返回指定维度的配额上限。
func (q *OrgQuota) LimitFor(key OrgQuotaKey) (int64, error) {
	if q == nil {
		return 0, fmt.Errorf("%w: quota is nil", ErrInvalidInput)
	}
	switch key {
	case OrgQuotaMembers:
		return q.MaxMembers, nil
	case OrgQuotaLakes:
		return q.MaxLakes, nil
	case OrgQuotaNodes:
		return q.MaxNodes, nil
	case OrgQuotaAttachments:
		return q.MaxAttachments, nil
	case OrgQuotaAPIKeys:
		return q.MaxAPIKeys, nil
	case OrgQuotaStorageMB:
		return q.MaxStorageMB, nil
	default:
		return 0, fmt.Errorf("%w: unknown quota key", ErrInvalidInput)
	}
}

// OrgUsageSnapshot 记录某个时刻的组织资源使用量。
type OrgUsageSnapshot struct {
	ID              string
	OrgID           string
	MembersUsed     int64
	LakesUsed       int64
	NodesUsed       int64
	AttachmentsUsed int64
	APIKeysUsed     int64
	StorageMBUsed   int64
	CapturedAt      time.Time
}
