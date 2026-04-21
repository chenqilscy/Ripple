// Package main · Ripple Go 后端入口。
//
// 装配顺序：config -> logger -> store(pg/neo4j/redis) -> platform(jwt) ->
// service(auth) -> api/http -> http.Server。
//
// 优雅关停：捕获 SIGINT/SIGTERM，先关 HTTP 再关数据库连接。
//
// 命令行 flag：
//   --healthcheck : 仅探测 http://127.0.0.1:$HTTP_ADDR/healthz，
//                   返回 0 表示健康，1 表示异常。供 Dockerfile HEALTHCHECK 使用。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	httpapi "github.com/chenqilscy/ripple/backend-go/internal/api/http"
	"github.com/chenqilscy/ripple/backend-go/internal/config"
	"github.com/chenqilscy/ripple/backend-go/internal/llm"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/realtime"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "仅 HTTP GET /healthz，0=ok 1=fail")
	flag.Parse()
	if *healthcheck {
		os.Exit(runHealthCheck())
	}

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	logger := platform.NewLogger(cfg.LogLevel, cfg.Env)
	logger.Info().Str("addr", cfg.HTTPAddr).Msg("ripple-go starting")

	bootCtx, cancelBoot := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelBoot()

	pg, err := store.NewPGPool(bootCtx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("pg connect failed")
	}
	defer pg.Close()

	neo, err := store.NewNeo4jDriver(bootCtx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("neo4j connect failed")
	}
	defer func() { _ = neo.Close(context.Background()) }()

	rds, err := store.NewRedis(bootCtx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("redis connect failed")
	}
	defer func() { _ = rds.Close() }()

	logger.Info().Msg("all middleware connected")

	jwt := platform.NewJWTSigner(cfg.JWTSecret, cfg.JWTExpiresIn)
	users := store.NewUserRepository(pg)
	memberships := store.NewMembershipRepository(pg)
	outbox := store.NewOutboxRepository(pg)
	txRunner := store.NewTxRunner(pg)
	cloudTasks := store.NewCloudTaskRepository(pg)
	lakes := store.NewLakeRepository(neo, cfg.Neo4jDatabase)
	nodes := store.NewNodeRepository(neo, cfg.Neo4jDatabase)

	authSvc := service.NewAuthService(users, jwt)
	lakeSvc := service.NewLakeService(lakes, memberships, outbox, txRunner)

	broker := newBroker(cfg, rds, logger)
	defer func() { _ = broker.Close() }()

	nodeSvc := service.NewNodeService(nodes, memberships, lakes, broker)
	cloudSvc := service.NewCloudService(cloudTasks, nodes, lakes)

	// Outbox dispatcher 在单独 goroutine 中运行
	dispatcher := service.NewOutboxDispatcher(outbox, lakes, logger)
	dispatcherCtx, dispatcherCancel := context.WithCancel(context.Background())
	defer dispatcherCancel()
	go dispatcher.Run(dispatcherCtx)

	// AI Weaver worker pool（造云）
	llmRecorder := store.NewPGCallRecorder(pg, 512)
	defer llmRecorder.Close()
	providers := llm.BuildProviders(llm.ProviderConfig{
		ZhipuKey: cfg.ZhipuAPIKey, ZhipuModel: cfg.ZhipuModel,
		OpenAIKey: cfg.OpenAIAPIKey, OpenAIModel: cfg.OpenAIModel, OpenAIEndpoint: cfg.OpenAIEndpoint,
		DeepSeekKey: cfg.DeepSeekAPIKey, DeepSeekModel: cfg.DeepSeekModel,
		VolcKey: cfg.VolcAPIKey, VolcModel: cfg.VolcModel,
		MiniMaxKey: cfg.MiniMaxAPIKey, MiniMaxModel: cfg.MiniMaxModel,
		OpenAICompatKey:      cfg.OpenAICompatKey,
		OpenAICompatModel:    cfg.OpenAICompatModel,
		OpenAICompatEndpoint: cfg.OpenAICompatEndpoint,
		OpenAICompatName:     cfg.OpenAICompatName,
		Order:                cfg.LLMProviderOrder,
	})
	if len(providers) == 0 {
		logger.Warn().Msg("no LLM provider configured; cloud generation will fail")
	} else {
		names := make([]string, 0, len(providers))
		for _, p := range providers {
			names = append(names, p.Name())
		}
		logger.Info().Strs("providers", names).Bool("fallback", cfg.LLMFallback).Msg("llm router assembled")
	}
	llmRouter := llm.NewDefaultRouter(providers, llm.Policy{EnableFallback: cfg.LLMFallback}, llmRecorder)
	weaver := service.NewAIWeaver(cloudTasks, nodes, llmRouter, broker, logger, 3)
	weaverCtx, weaverCancel := context.WithCancel(context.Background())
	defer weaverCancel()
	go weaver.Run(weaverCtx)

	wsH := &httpapi.WSHandlers{
		Lakes:   lakeSvc,
		Broker:  broker,
		Origins: cfg.CORSOriginList(),
	}

	router := httpapi.NewRouter(httpapi.Deps{
		Auth:        authSvc,
		Lakes:       lakeSvc,
		Nodes:       nodeSvc,
		Clouds:      cloudSvc,
		WS:          wsH,
		CORSOrigins: cfg.CORSOriginList(),
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info().Msg("shutdown signal received")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal().Err(err).Msg("http server error")
	}
	logger.Info().Msg("ripple-go stopped")
}

// runHealthCheck 探测本机 /healthz；3s 超时；非 200 返 1。
func runHealthCheck() int {
	addr := os.Getenv("RIPPLE_HTTP_ADDR")
	if addr == "" {
		addr = ":8000"
	}
	host := "127.0.0.1"
	if !strings.HasPrefix(addr, ":") {
		host = ""
	}
	url := fmt.Sprintf("http://%s%s/healthz", host, addr)

	cli := &http.Client{Timeout: 3 * time.Second}
	resp, err := cli.Get(url) //nolint:noctx // healthcheck 临时进程
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck failed:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck status:", resp.StatusCode)
		return 1
	}
	return 0
}


// newBroker 按 cfg.BrokerKind 选择 memory 或 redis 实现。
func newBroker(cfg *config.Config, rds *redis.Client, logger zerolog.Logger) realtime.Broker {
	switch cfg.BrokerKind {
	case "redis":
		logger.Info().Str("kind", "redis").Msg("broker selected")
		return realtime.NewRedisBroker(rds, 128)
	default:
		logger.Info().Str("kind", "memory").Msg("broker selected")
		return realtime.NewMemoryBroker(128)
	}
}
