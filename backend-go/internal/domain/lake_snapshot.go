package domain

import "time"

// LakeSnapshot 图谱布局快照（P18-D）。
// Layout 是 JSON 对象：{"nodeId": {"x": float, "y": float}, ...}
// GraphState 是 JSON 对象（P21）：{"nodes": [{id, title, type}], "edges": [{id, src, dst, kind}]}
// 存于 PG（lake_snapshots 表）。
type LakeSnapshot struct {
	ID         string
	LakeID     string
	Name       string
	Layout     []byte // JSONB — 节点坐标映射
	GraphState []byte // JSONB — 图谱内容快照（P21，可为 nil）
	CreatedBy  string // user UUID
	CreatedAt  time.Time
}
