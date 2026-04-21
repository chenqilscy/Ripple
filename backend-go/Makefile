# Ripple Go Backend · Makefile 骨架
# 真正的目标在收到中间件信息后逐项实现。

.PHONY: help tidy build run test lint migrate-up migrate-down

help:
	@echo "Ripple Go backend · 可用目标："
	@echo "  tidy           - go mod tidy"
	@echo "  build          - 构建 cmd/server"
	@echo "  run            - 运行 cmd/server (需 .env)"
	@echo "  test           - 跑全部测试"
	@echo "  lint           - golangci-lint"
	@echo "  migrate-up     - 升级 PG schema (需 atlas/migrate 工具)"
	@echo "  migrate-down   - 回滚 PG schema"

tidy:
	go mod tidy

build:
	go build -o bin/ripple-server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -race -cover

lint:
	golangci-lint run ./...

migrate-up:
	@echo "TODO: 等中间件信息到位后实现"

migrate-down:
	@echo "TODO: 等中间件信息到位后实现"
