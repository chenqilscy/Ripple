# Ripple Go Backend · Makefile
# 用法：在 backend-go/ 目录下执行 `make <target>`。
# 大部分目标会读取 .env（gitignored）作为环境变量。

.PHONY: help tidy build run test test-cover lint healthcheck \
        migrate-up migrate-down neo4j-init docker-build docker-up docker-down

# 默认目标：列举可用命令
help:
	@echo "Ripple Go backend · 可用目标："
	@echo ""
	@echo "  tidy           - go mod tidy（拉依赖）"
	@echo "  build          - 构建 cmd/server 到 bin/ripple-server"
	@echo "  run            - 加载 .env 后运行 cmd/server"
	@echo "  test           - 跑全部单元测试 (含 race)"
	@echo "  test-cover     - 跑测试并打印覆盖率"
	@echo "  lint           - golangci-lint"
	@echo "  healthcheck    - 探测本地 /healthz (调用编译后二进制)"
	@echo ""
	@echo "  migrate-up     - 应用 PG migrations/0001_init.up.sql"
	@echo "  migrate-down   - 回滚 PG migrations/0001_init.down.sql (危险!)"
	@echo "  neo4j-init     - 在 Neo4j 创建 unique constraint 与 index"
	@echo ""
	@echo "  docker-build   - 构建 ripple-go:dev 镜像"
	@echo "  docker-up      - docker compose -f docker-compose.dev.yml up"
	@echo "  docker-down    - docker compose -f docker-compose.dev.yml down"

# === Go ===

tidy:
	go mod tidy

build:
	go build -trimpath -o bin/ripple-server ./cmd/server

run:
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
	go run ./cmd/server

test:
	go test ./... -race

test-cover:
	go test ./... -race -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -n 1

lint:
	golangci-lint run ./...

healthcheck:
	./bin/ripple-server --healthcheck

# === Schema ===
# PG migrations 用 psql 直接跑，避免引入额外工具链。
# .env 提供 PG_URL；为兼容 psql，转换 RIPPLE_PG_URL=postgres://... 为 PGURL 变量。

migrate-up:
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
	psql "$$RIPPLE_PG_URL" -v ON_ERROR_STOP=1 -f migrations/0001_init.up.sql

migrate-down:
	@echo "⚠️  这会删除所有数据，确认请在 5 秒内 Ctrl+C 取消..."
	@sleep 5
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
	psql "$$RIPPLE_PG_URL" -v ON_ERROR_STOP=1 -f migrations/0001_init.down.sql

neo4j-init:
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
	cypher-shell -a "$$RIPPLE_NEO4J_URI" -u "$$RIPPLE_NEO4J_USER" -p "$$RIPPLE_NEO4J_PASS" \
	  -d "$$RIPPLE_NEO4J_DATABASE" -f scripts/neo4j_constraints.cypher

# === Docker ===

docker-build:
	docker build -t ripple-go:dev .

docker-up:
	docker compose -f docker-compose.dev.yml up --build

docker-down:
	docker compose -f docker-compose.dev.yml down
