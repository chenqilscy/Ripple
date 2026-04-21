package domain

import "time"

// CloudTaskStatus 造云任务状态。
type CloudTaskStatus string

const (
	CloudStatusQueued  CloudTaskStatus = "queued"
	CloudStatusRunning CloudTaskStatus = "running"
	CloudStatusDone    CloudTaskStatus = "done"
	CloudStatusFailed  CloudTaskStatus = "failed"
)

// CloudTask 造云任务。AI Weaver 异步消费。
//
// 生命周期：queued → running → (done | failed)
type CloudTask struct {
	ID            string
	OwnerID       string
	LakeID        string // 可空：节点保持 MIST 不归湖
	Prompt        string
	N             int
	NodeType      NodeType
	Status        CloudTaskStatus
	RetryCount    int
	LastError     string
	ResultNodeIDs []string
	CreatedAt     time.Time
	StartedAt     *time.Time
	CompletedAt   *time.Time
}

// MaxCloudN 造云请求 n 上限（防成本失控）。
const MaxCloudN = 10
