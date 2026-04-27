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
	_ "net/http/pprof" // 副作用：注册 /debug/pprof/* 到 http.DefaultServeMux（仅当启用 pprof 监听时暴露）
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	httpapi "github.com/chenqilscy/ripple/backend-go/internal/api/http"
	"github.com/chenqilscy/ripple/backend-go/internal/config"
	"github.com/chenqilscy/ripple/backend-go/internal/llm"
	"github.com/chenqilscy/ripple/backend-go/internal/metrics"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/presence"
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
	edges := store.NewEdgeRepository(neo, cfg.Neo4jDatabase)
	invites := store.NewInviteRepository(pg)
	nodeRevs := store.NewNodeRevisionRepository(pg)
	spaceRepo := store.NewSpaceRepository(pg)
	permaRepo := store.NewPermaNodeRepository(pg)
	docStateRepo := store.NewNodeDocStateRepository(pg) // P8-A/B/C
	apiKeyRepo := store.NewAPIKeyRepository(pg)         // P10-A
	auditLogRepo := store.NewAuditLogRepository(pg)     // P10-B
	orgRepo := store.NewOrgRepository(pg)               // P12-C
	orgQuotaRepo := store.NewOrgQuotaRepository(pg)     // P14-A

	// P10-B：启动时清理 30 天以前的审计日志（非阻塞）
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cutoff := time.Now().UTC().AddDate(0, 0, -30)
		if n, err := auditLogRepo.PruneOlderThan(ctx, cutoff); err != nil {
			logger.Warn().Err(err).Msg("audit_log prune failed")
		} else if n > 0 {
			logger.Info().Int64("pruned", n).Msg("audit_logs pruned")
		}
	}()

	// P11-D：DB 连接池指标采集（每 30s 刷新一次）
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			stat := pg.Stat()
			metrics.DBPoolAcquired.Set(int64(stat.AcquiredConns()))
			metrics.DBPoolTotal.Set(int64(stat.TotalConns()))
		}
	}()

	authSvc := service.NewAuthService(users, jwt)
	lakeSvc := service.NewLakeService(lakes, memberships, outbox, txRunner)
	spaceSvc := service.NewSpaceService(spaceRepo)
	orgSvc := service.NewOrgService(orgRepo).
		WithQuotaRepository(orgQuotaRepo).
		WithAuditLogRepository(auditLogRepo) // P12-C / P14-A
	notifRepo := store.NewNotificationRepository(pg) // P13-B
	tagRepo := store.NewTagRepository(pg)            // P13-C
	// P18：新仓库
	nodeTemplateRepo := store.NewNodeTemplateRepository(pg) // P18-C
	lakeSnapshotRepo := store.NewLakeSnapshotRepository(pg) // P18-D
	nodeShareRepo := store.NewNodeShareRepository(pg)       // P18-B

	// P18-C：插入系统内置模板（幂等）
	if err := store.EnsureSystemTemplates(bootCtx, pg); err != nil {
		logger.Warn().Err(err).Msg("ensure system templates failed (non-fatal)")
	}

	broker := newBroker(cfg, rds, logger)
	defer func() { _ = broker.Close() }()

	notifSvc := service.NewNotificationService(notifRepo, broker) // P13-B / P14-A

	presenceSvc := presence.NewService(rds, broker, 0)

	nodeSvc := service.NewNodeService(nodes, memberships, lakes, broker).WithRevisions(nodeRevs)
	edgeSvc := service.NewEdgeService(edges, nodes, memberships, lakes, broker)
	inviteSvc := service.NewInviteService(invites, memberships, lakes)
	cloudSvc := service.NewCloudService(cloudTasks, nodes, lakes)
	tagSvc := service.NewTagService(tagRepo, memberships, nodes) // P13-C

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
		ClaudeCodeEnabled:    cfg.ClaudeCodeEnabled,
		ClaudeCodeCLIPath:    cfg.ClaudeCodeCLIPath,
		ClaudeCodeModel:      cfg.ClaudeCodeModel,
		RPS:                  cfg.LLMRPS,
		Burst:                cfg.LLMBurst,
		Order:                cfg.LLMProviderOrder,
	})
	// M4-T5：fake provider（压测专用，避免真付费）
	if cfg.LLMFake {
		fp := llm.NewFakeProvider(cfg.LLMFakeSleepMS, cfg.LLMFakeTextLen)
		providers = append([]llm.Provider{fp}, providers...) // 放最前，优先命中
		logger.Warn().Msg("LLM_FAKE=true：fake provider 已启用，所有 TEXT 生成不会调用真实厂商")
	}
	// M4-S2：占位图片 provider（多模态 IMAGE 通路）
	if cfg.LLMImageStub {
		ip := llm.NewPlaceholderImageProvider("image-stub", cfg.LLMImageStubSleepMS)
		providers = append(providers, ip)
		logger.Warn().Msg("LLM_IMAGE_STUB=true：image-stub provider 已启用（IMAGE 模态）")
	}
	if len(providers) == 0 {
		logger.Warn().Msg("no LLM provider configured; cloud generation will fail")
	} else {
		names := make([]string, 0, len(providers))
		for _, p := range providers {
			names = append(names, p.Name())
		}
		logger.Info().Strs("providers", names).Bool("fallback", cfg.LLMFallback).Msg("llm router assembled")
	}
	// Claude Code CLI 侦测（非阻塞，仅日志）。订阅制 provider，单独于 token-based router。
	go func() {
		pctx, pcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pcancel()
		r := llm.ProbeClaudeCodeCLI(pctx, cfg.ClaudeCodeCLIPath)
		if r.Available {
			logger.Info().Str("path", r.Path).Str("version", r.Version).Msg("claude code cli detected")
		} else if cfg.ClaudeCodeCLIPath != "" {
			// 显式配置了路径却不可用，报 warn
			logger.Warn().Err(r.Err).Str("configured_path", cfg.ClaudeCodeCLIPath).Msg("claude code cli not available")
		}
	}()
	llmRouter := llm.NewDefaultRouter(providers, llm.Policy{EnableFallback: cfg.LLMFallback}, llmRecorder)
	weaver := service.NewAIWeaver(cloudTasks, nodes, llmRouter, broker, logger, 3)
	weaverCtx, weaverCancel := context.WithCancel(context.Background())
	defer weaverCancel()
	go weaver.Run(weaverCtx)

	crystallizeSvc := service.NewCrystallizeService(permaRepo, nodes, memberships, llmRouter)

	feedbackRepo := store.NewFeedbackRepository(pg)
	recommenderSvc := service.NewRecommenderService(feedbackRepo, rds).
		WithSpaceSignal(permaRepo, memberships)

	// M4-B：本地 FS 附件（UploadDir 非空时启用）
	var attachmentH *httpapi.AttachmentHandlers
	if cfg.UploadDir != "" {
		attachRepo := store.NewAttachmentRepository(pg)
		ah, err := httpapi.NewAttachmentHandlers(attachRepo, cfg.UploadDir, cfg.UploadMaxMB, cfg.UploadAllowMIME)
		if err != nil {
			logger.Fatal().Err(err).Msg("attachment handlers init failed")
		}
		attachmentH = ah
	}

	wsH := &httpapi.WSHandlers{
		Lakes:    lakeSvc,
		Broker:   broker,
		Presence: presenceSvc,
		Origins:  cfg.CORSOriginList(),
	}

	// P7-B：ws-only 短期 token 签发（5 分钟有效，purpose="ws"）
	wsTokenH := &httpapi.WsTokenHandlers{JWT: jwt}

	router := httpapi.NewRouter(httpapi.Deps{
		Auth:           authSvc,
		Lakes:          lakeSvc,
		Nodes:          nodeSvc,
		Edges:          edgeSvc,
		Invites:        inviteSvc,
		Clouds:         cloudSvc,
		Spaces:         spaceSvc,
		Crystallize:    crystallizeSvc,
		Recommender:    recommenderSvc,
		Feedback:       feedbackRepo,
		Attachments:    attachmentH,
		Presence:       presenceSvc,
		WS:             wsH,
		WsToken:        wsTokenH,
		DocStates:      docStateRepo,
		APIKeys:        apiKeyRepo,
		AuditLogs:      auditLogRepo,
		Orgs:           orgSvc,
		Notifications:  notifSvc,
		Tags:           tagSvc,
		LLMRouter:      llmRouter,
		CORSOrigins:    cfg.CORSOriginList(),
		MetricsEnabled: cfg.MetricsEnabled,
		// P18
		NodeTemplates: nodeTemplateRepo,
		LakeSnapshots: lakeSnapshotRepo,
		NodeShares:    nodeShareRepo,
		Memberships:   memberships,
		Users:         users, // P12-C: Org by_email 邀请
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// pprof 独立监听（生产建议绑内网）。配置为空则不启动。
	if cfg.PProfAddr != "" {
		go func() {
			logger.Info().Str("addr", cfg.PProfAddr).Msg("pprof listening")
			pprofMux := http.NewServeMux()
			// net/http/pprof 在 init 时注册到 http.DefaultServeMux；这里用 default 即可
			pprofMux.Handle("/", http.DefaultServeMux)
			if err := http.ListenAndServe(cfg.PProfAddr, pprofMux); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error().Err(err).Msg("pprof server error")
			}
		}()
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
