package domain

import "time"

// PermaNode 凝结后的"晶体节点"。
//
// 数据分布：
//   - PG: perma_nodes 表（本结构体）— 存索引、source 列表、摘要、provider 审计
//   - Neo4j: :Perma 节点 — 仅存 ID 引用 + 与原 mist 的关系
//
// 创建路径：mist 列表 → LLM 总结 → 创建 PG 行 + Neo4j 节点（saga）
type PermaNode struct {
	ID            string
	LakeID        string
	OwnerID       string
	Title         string
	Summary       string
	SourceNodeIDs []string
	LLMProvider   string
	LLMCostTokens int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
