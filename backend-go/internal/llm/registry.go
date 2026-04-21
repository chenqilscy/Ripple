// Package llm · 多 provider 注册器。
//
// 从配置出发组装 Router 的 providers 列表。
// 顺序决定 Router 路由优先级（DefaultRouter 取第一个 Supports 的）。
//
// 用法：
//
//	cfg := config.Load()
//	providers := llm.BuildProvidersFromConfig(cfg)
//	router := llm.NewDefaultRouter(providers, llm.Policy{EnableFallback: true}, recorder)
//
// 已配置 API key 的 provider 才注册；未配置静默忽略，方便本地 dev 仅开一两家。

package llm

import (
	"strings"
)

// ProviderConfig 业务侧不感知 envconfig，由调用方组装。
type ProviderConfig struct {
	// 各 provider 的 key/model，留空则该 provider 不注册
	ZhipuKey   string
	ZhipuModel string

	OpenAIKey      string
	OpenAIModel    string
	OpenAIEndpoint string

	DeepSeekKey   string
	DeepSeekModel string

	VolcKey   string
	VolcModel string

	MiniMaxKey   string
	MiniMaxModel string

	OpenAICompatKey      string
	OpenAICompatModel    string
	OpenAICompatEndpoint string
	OpenAICompatName     string

	// Order 控制注册顺序，逗号分隔。空 = 默认顺序。
	// 合法值：zhipu,openai,deepseek,volc,minimax,openai-compat
	Order string
}

// BuildProviders 按 cfg 顺序构造 Provider 切片。
// 不会 panic；缺 key 的 provider 跳过。
func BuildProviders(cfg ProviderConfig) []Provider {
	order := parseOrder(cfg.Order)
	out := make([]Provider, 0, len(order))
	seen := map[string]bool{}
	for _, name := range order {
		if seen[name] {
			continue
		}
		seen[name] = true
		if p := buildOne(name, cfg); p != nil {
			out = append(out, p)
		}
	}
	return out
}

func parseOrder(raw string) []string {
	def := []string{"zhipu", "deepseek", "openai", "volc", "minimax", "openai-compat"}
	if raw == "" {
		return def
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

func buildOne(name string, cfg ProviderConfig) Provider {
	switch name {
	case "zhipu":
		if cfg.ZhipuKey == "" {
			return nil
		}
		return NewZhipuClient(cfg.ZhipuKey, cfg.ZhipuModel, "").AsProvider()
	case "openai":
		if cfg.OpenAIKey == "" {
			return nil
		}
		return NewOpenAICompatClient(OpenAICompatConfig{
			Name: "openai", APIKey: cfg.OpenAIKey, Model: cfg.OpenAIModel, Endpoint: cfg.OpenAIEndpoint,
		})
	case "deepseek":
		if cfg.DeepSeekKey == "" {
			return nil
		}
		return NewOpenAICompatClient(OpenAICompatConfig{
			Name: "deepseek", APIKey: cfg.DeepSeekKey, Model: cfg.DeepSeekModel,
		})
	case "volc", "doubao":
		if cfg.VolcKey == "" {
			return nil
		}
		return NewOpenAICompatClient(OpenAICompatConfig{
			Name: "volc", APIKey: cfg.VolcKey, Model: cfg.VolcModel,
		})
	case "minimax":
		if cfg.MiniMaxKey == "" {
			return nil
		}
		return NewOpenAICompatClient(OpenAICompatConfig{
			Name: "minimax", APIKey: cfg.MiniMaxKey, Model: cfg.MiniMaxModel,
		})
	case "openai-compat":
		if cfg.OpenAICompatKey == "" || cfg.OpenAICompatEndpoint == "" || cfg.OpenAICompatModel == "" {
			return nil
		}
		name := cfg.OpenAICompatName
		if name == "" {
			name = "openai-compat"
		}
		return NewOpenAICompatClient(OpenAICompatConfig{
			Name: name, APIKey: cfg.OpenAICompatKey, Model: cfg.OpenAICompatModel,
			Endpoint: cfg.OpenAICompatEndpoint,
		})
	}
	return nil
}
