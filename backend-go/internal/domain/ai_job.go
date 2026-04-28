package domain

import "time"

// AiJobStatus 是 AI 任务状态。
type AiJobStatus string

const (
	AiJobPending    AiJobStatus = "pending"
	AiJobProcessing AiJobStatus = "processing"
	AiJobDone       AiJobStatus = "done"
	AiJobFailed     AiJobStatus = "failed"
)

// IsActive 是否仍在进行中（不可重触发）。
func (s AiJobStatus) IsActive() bool {
	return s == AiJobPending || s == AiJobProcessing
}

// AiJob 是 AI 节点填充任务（Phase 15-C）。
type AiJob struct {
	ID                 string
	NodeID             string
	LakeID             string
	PromptTemplateID   string // 可空（模板被删除后 SET NULL）
	Status             AiJobStatus
	Priority           int    // Phase 15.2：越大越优先，默认 0
	ProgressPct        int
	InputNodeIDs       []string
	OverrideVars       map[string]string
	StartedAt          *time.Time
	FinishedAt         *time.Time
	Error              string
	CreatedBy          string
	CreatedAt          time.Time
}
