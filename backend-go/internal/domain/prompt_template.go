package domain

import "time"

// PromptTemplateScope 是模板可见性范围。
type PromptTemplateScope string

const (
	PromptScopePrivate PromptTemplateScope = "private"
	PromptScopeOrg     PromptTemplateScope = "org"
)

// PromptTemplate 是 AI 节点触发所使用的 Prompt 模板（Phase 15-C）。
type PromptTemplate struct {
	ID          string
	Name        string
	Description string
	Template    string
	Scope       PromptTemplateScope
	OrgID       string // scope=org 时有值
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PromptTemplateUpdate 支持 PATCH 的可变字段。
type PromptTemplateUpdate struct {
	Name        *string
	Description *string
	Template    *string
	Scope       *PromptTemplateScope
}
