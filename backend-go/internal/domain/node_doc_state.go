package domain

import "time"

// NodeDocState 存储 Y.Doc 的编码状态快照（P8-A）。
// 一个节点对应一条记录；存储 Yjs encodeStateAsUpdate 二进制（≤1MB）。
type NodeDocState struct {
	NodeID    string    // 对应 Neo4j 节点 ID
	State     []byte    // Y.Doc encodeStateAsUpdate 二进制
	Version   int64     // 乐观锁版本号（每次 PUT +1）
	UpdatedAt time.Time
}
