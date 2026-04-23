package domain

import "time"

// LakeSnapshot 图谱布局快照（P18-D）。
// Layout 是 JSON 对象：{"nodeId": {"x": float, "y": float}, ...}
// 存于 PG（lake_snapshots 表）。
type LakeSnapshot struct {
	ID        string
	LakeID    string
	Name      string
	Layout    []byte // JSONB — 节点坐标映射
	CreatedBy string // user UUID
	CreatedAt time.Time
}
