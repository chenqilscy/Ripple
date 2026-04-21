// Package main · Ripple Go 后端入口。
//
// 装配顺序：config -> logger -> store(pg/neo4j/redis) -> platform(jwt) ->
// service(auth) -> api/http -> http.Server。
//
// 优雅关停：捕获 SIGINT/SIGTERM，先关 HTTP 再关数据库连接。
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpapi "github.com/chenqilscy/ripple/backend-go/internal/api/http"
	"github.com/chenqilscy/ripple/backend-go/internal/config"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

func main() {
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
	lakes := store.NewLakeRepository(neo, cfg.Neo4jDatabase)
	nodes := store.NewNodeRepository(neo, cfg.Neo4jDatabase)

	authSvc := service.NewAuthService(users, jwt)
	lakeSvc := service.NewLakeService(lakes, memberships)
	nodeSvc := service.NewNodeService(nodes, memberships, lakes)

	router := httpapi.NewRouter(httpapi.Deps{
		Auth:        authSvc,
		Lakes:       lakeSvc,
		Nodes:       nodeSvc,
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
