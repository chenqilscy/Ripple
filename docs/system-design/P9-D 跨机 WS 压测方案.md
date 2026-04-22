# P9-D · 跨机 WS 1000 并发压测方案（TD-04）

> 状态：**文档就绪，待生产环境执行**  
> 依赖：`backend-go/cmd/loadtest/ws_connect/ws_connect.go`（已有）

---

## 背景

- P7-C 验证了同机 300 并发（Windows 端口耗尽上限）
- P9-A 完成 Redis Pub/Sub 多实例支持后，需验证 1000+ 并发跨实例广播
- TD-04 要求至少 1 台独立客户端机器向目标服务器发起 1000 连接

---

## 先决条件

| 项目 | 要求 |
|------|------|
| 服务端 | yjs-bridge（:7790）+ 主 HTTP API（:8000）启动，`YJS_BRIDGE_REDIS_URL` 已配置 |
| 客户端机 | 与服务端网络互通（同 LAN 或 VPN），Go 1.23+ 已安装 |
| JWT | 有效的 RIPPLE_JWT_SECRET 签发的 Bearer token |
| 压测目标 | `/api/v1/lakes/<lakeID>/ws`（Lake WS）或 `/yjs?node=<nodeID>&token=<jwt>`（yjs-bridge） |

---

## 步骤

### 1. 准备 JWT token

在服务端或任意机器上运行：

```bash
# 获取 token（替换 user/pass）
curl -s -X POST http://<SERVER>:8000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@ripple.io","password":"<pass>"}' \
  | jq -r .access_token
```

### 2. 准备测试用 Lake

```bash
# 创建测试湖
curl -s -X POST http://<SERVER>:8000/api/v1/lakes \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"loadtest-lake","is_public":false}' | jq -r .id
```

### 3. 编译压测工具（在客户端机器上）

```bash
# 克隆或复制 backend-go/cmd/loadtest/ws_connect/
cd backend-go
go build -o ws_connect ./cmd/loadtest/ws_connect/
```

### 4. 运行 1000 并发压测

```bash
./ws_connect \
  -url "ws://<SERVER>:8000/api/v1/lakes/<LAKE_ID>/ws" \
  -token "$TOKEN" \
  -conc 1000 \
  -hold 60s \
  -dial-timeout 15s
```

### 5. 多实例验证（2 个 yjs-bridge 实例）

在服务端分别启动两个 bridge 实例（不同端口），连接同一 Redis：

```bash
# 实例 A（:7790）
YJS_BRIDGE_ADDR=:7790 YJS_BRIDGE_REDIS_URL=redis://:<PASS>@<REDIS>:16379 \
  go run ./cmd/yjs-bridge/ &

# 实例 B（:7791）
YJS_BRIDGE_ADDR=:7791 YJS_BRIDGE_REDIS_URL=redis://:<PASS>@<REDIS>:16379 \
  go run ./cmd/yjs-bridge/ &
```

分别对 :7790 和 :7791 发起 500 并发，验证 Redis Pub/Sub 跨实例广播：

```bash
./ws_connect \
  -url "ws://<SERVER>:7790/yjs?node=<NODE_ID>&token=$TOKEN&lake=<LAKE_ID>" \
  -conc 500 -hold 30s

./ws_connect \
  -url "ws://<SERVER>:7791/yjs?node=<NODE_ID>&token=$TOKEN&lake=<LAKE_ID>" \
  -conc 500 -hold 30s
```

---

## 预期通过标准

| 指标 | 目标 |
|------|------|
| 成功建连率 | ≥ 99%（失败 ≤ 10） |
| 握手 p95 延迟 | ≤ 500ms |
| 60s 存活率 | ≥ 98% |
| 服务端 goroutine 无泄漏 | `SIGINT` 停止后 goroutine 数量回落至 < 50 |
| Redis 发布消息正常 | `MONITOR` 命令观察到 yjs-bridge Pub/Sub 消息 |

---

## 系统参数调优（生产服务器 Linux）

```bash
# 最大文件描述符
ulimit -n 65536

# 内核网络参数
sysctl -w net.core.somaxconn=65536
sysctl -w net.ipv4.tcp_max_syn_backlog=65536

# TCP TIME_WAIT 复用
sysctl -w net.ipv4.tcp_tw_reuse=1
```

---

## 问题排查

| 现象 | 原因 | 处理 |
|------|------|------|
| `dial timeout` | 服务端 CORS/防火墙拦截或端口未开放 | 检查防火墙规则，确认 :8000 / :7790 可达 |
| `connection refused` | 服务未启动 | 检查 systemd/pm2 进程状态 |
| 成功率 < 90% | 服务端 goroutine 资源不足 | 调 `GOMAXPROCS`，检查内存、`ulimit` |
| Redis 断连 | 网络抖动 | `subscribeRoom` 重连逻辑（2s 延迟）自动恢复，监控报警阈值 |
