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

	// JWT
	JWTSecret    string        `envconfig:"JWT_SECRET" required:"true"`
	JWTExpiresIn time.Duration `envconfig:"JWT_EXPIRES_IN" default:"24h"`

	// LLM
	ZhipuAPIKey string `envconfig:"ZHIPU_API_KEY"`
	ZhipuModel  string `envconfig:"ZHIPU_MODEL" default:"glm-4-flash"`
	MiniMaxKey  string `envconfig:"MINIMAX_API_KEY"`
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
