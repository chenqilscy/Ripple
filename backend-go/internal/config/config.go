// Package config 在启动期加载与校验所有配置。
// 任何关键字段缺失立即 panic，不允许进入运行态。
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

const envPrefix = "RIPPLE"

// Config 是服务配置全集。环境变量前缀 RIPPLE_。
type Config struct {
	// 服务
	Env         string `envconfig:"ENV" default:"dev"`
	HTTPAddr    string `envconfig:"HTTP_ADDR" default:":8000"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`
	CORSOrigins string `envconfig:"CORS_ORIGINS" default:"http://localhost:5173"`

	// PG
	PGURL         string        `envconfig:"PG_URL" required:"true"`
	PGMaxConns    int32         `envconfig:"PG_MAX_CONNS" default:"20"`
	PGMinConns    int32         `envconfig:"PG_MIN_CONNS" default:"2"`
	PGConnTimeout time.Duration `envconfig:"PG_CONN_TIMEOUT" default:"5s"`

	// Neo4j
	Neo4jURI      string `envconfig:"NEO4J_URI" required:"true"`
	Neo4jUser     string `envconfig:"NEO4J_USER" required:"true"`
	Neo4jPass     string `envconfig:"NEO4J_PASS" required:"true"`
	Neo4jDatabase string `envconfig:"NEO4J_DATABASE" default:"neo4j"`

	// Redis
	RedisAddr string `envconfig:"REDIS_ADDR" required:"true"`
	RedisPass string `envconfig:"REDIS_PASS"`
	RedisDB   int    `envconfig:"REDIS_DB" default:"0"`

	// Broker（实时广播）：memory（单实例） | redis（多实例）
	BrokerKind string `envconfig:"BROKER_KIND" default:"memory"`

	// JWT
	JWTSecret    string        `envconfig:"JWT_SECRET" required:"true"`
	JWTExpiresIn time.Duration `envconfig:"JWT_EXPIRES_IN" default:"24h"`

	// LLM
	// 多 provider 支持：zhipu / openai / deepseek / volc(豆包) / minimax / 任意 openai-compat。
	// 留空则该 provider 不注册。
	// 优先级 = LLM_PROVIDER_ORDER 字符串里的位置（如 "zhipu,deepseek,openai"），
	// 留空则按代码内置默认顺序：zhipu → deepseek → openai → volc → minimax → openai-compat。
	LLMProviderOrder string `envconfig:"LLM_PROVIDER_ORDER" default:""`
	LLMFallback      bool   `envconfig:"LLM_FALLBACK" default:"true"`

	// Zhipu
	ZhipuAPIKey string `envconfig:"ZHIPU_API_KEY"`
	ZhipuModel  string `envconfig:"ZHIPU_MODEL" default:"glm-4-flash"`

	// OpenAI 官方
	OpenAIAPIKey   string `envconfig:"OPENAI_API_KEY"`
	OpenAIModel    string `envconfig:"OPENAI_MODEL" default:"gpt-4o-mini"`
	OpenAIEndpoint string `envconfig:"OPENAI_ENDPOINT"` // 留空走默认

	// DeepSeek
	DeepSeekAPIKey string `envconfig:"DEEPSEEK_API_KEY"`
	DeepSeekModel  string `envconfig:"DEEPSEEK_MODEL" default:"deepseek-chat"`

	// 火山豆包
	VolcAPIKey string `envconfig:"VOLC_API_KEY"`
	VolcModel  string `envconfig:"VOLC_MODEL" default:"doubao-1-5-lite-32k-250115"`

	// MiniMax
	MiniMaxAPIKey string `envconfig:"MINIMAX_API_KEY"`
	MiniMaxModel  string `envconfig:"MINIMAX_MODEL" default:"abab6.5s-chat"`

	// 通用 OpenAI 兼容（如本地 Ollama / vLLM）。同时配 KEY+ENDPOINT+MODEL 才注册。
	OpenAICompatKey      string `envconfig:"OPENAI_COMPAT_KEY"`
	OpenAICompatModel    string `envconfig:"OPENAI_COMPAT_MODEL"`
	OpenAICompatEndpoint string `envconfig:"OPENAI_COMPAT_ENDPOINT"`
	OpenAICompatName     string `envconfig:"OPENAI_COMPAT_NAME" default:"openai-compat"`
}

// Load 从环境变量加载配置。
func Load() (*Config, error) {
	var c Config
	if err := envconfig.Process(envPrefix, &c); err != nil {
		return nil, fmt.Errorf("config: load env: %w", err)
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// validate 启动期硬性校验。审查报告 L4-06。
func (c *Config) validate() error {
	var errs []string
	if c.Env == "prod" {
		if len(c.JWTSecret) < 32 {
			errs = append(errs, "JWT_SECRET 在 prod 下长度必须 >= 32")
		}
		if strings.Contains(c.JWTSecret, "dev") || strings.Contains(c.JWTSecret, "change") {
			errs = append(errs, "JWT_SECRET 在 prod 下不允许使用开发占位符")
		}
	}
	if c.PGMaxConns < c.PGMinConns {
		errs = append(errs, "PG_MAX_CONNS 必须 >= PG_MIN_CONNS")
	}
	if len(errs) > 0 {
		return errors.New("config: " + strings.Join(errs, "; "))
	}
	return nil
}

// CORSOriginList 返回切片化的 CORS 列表。
func (c *Config) CORSOriginList() []string {
	out := make([]string, 0)
	for _, s := range strings.Split(c.CORSOrigins, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
