package service

import (
	"html"
	"regexp"
	"strings"
)

const defaultMaxOutputLen = 4000

// PromptRenderer 负责渲染 Prompt 模板（Phase 15-C）。
// 变量替换：{{var_name}} → 对应值（HTML 转义，防 XSS）。
// 长度上限：渲染后 > MaxOutputLen 字符则硬截断并追加 "\n[内容已截断]"。
type PromptRenderer struct {
	MaxOutputLen int
}

// NewPromptRenderer 构造，使用默认最大长度 4000。
func NewPromptRenderer() *PromptRenderer {
	return &PromptRenderer{MaxOutputLen: defaultMaxOutputLen}
}

var varPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// Render 将 tmpl 中的 {{var}} 替换为 vars 对应值。
// 未知变量保留原文（便于用户自定义扩展）。
// 所有 var value 做 HTML 转义（防 XSS）。
func (r *PromptRenderer) Render(tmpl string, vars map[string]string) string {
	result := varPattern.ReplaceAllStringFunc(tmpl, func(match string) string {
		key := strings.TrimSpace(match[2 : len(match)-2])
		if val, ok := vars[key]; ok {
			return html.EscapeString(val)
		}
		return match // 未知变量保留原文
	})

	maxLen := r.MaxOutputLen
	if maxLen <= 0 {
		maxLen = defaultMaxOutputLen
	}
	if len([]rune(result)) > maxLen {
		runes := []rune(result)[:maxLen]
		result = string(runes) + "\n[内容已截断]"
	}
	return result
}

// StripHTML 移除 HTML 标签，返回纯文本（用于 {{node_content}} 变量填充）。
func StripHTML(s string) string {
	const tagPattern = `<[^>]*>`
	re := regexp.MustCompile(tagPattern)
	return re.ReplaceAllString(s, "")
}
