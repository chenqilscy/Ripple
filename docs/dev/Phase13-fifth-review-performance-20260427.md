# Phase 13 第五轮深度代码审查：性能与资源管理（2026-04-27）

## 范围

- 公开分享端点 `/api/v1/share/{token}`
- HTTP handler 输入校验后的资源保护路径
- 无鉴权入口的内存占用与 DB 压力

## 发现与修复

### Public share 公开端点缺少限流

- 文件：`backend-go/internal/api/http/handlers_share.go`
- 风险：该端点无需鉴权，若持续请求随机 token，即使上一轮已增加格式短路，合法格式随机 token 仍会进入 repository 查询，对 DB 与 handler goroutine 造成不必要压力。
- 修复：新增按客户端 IP 的轻量 token bucket：
  - 默认 `2 rps`，`burst=20`；
  - 返回 `429 too many share requests`；
  - limiter entry 带 `lastSeen` 与定期清理，避免长时间运行后 map 无界增长；
  - 使用 `RemoteAddr` 解析客户端 IP，不信任业务 header。

## 新增测试

- `TestShareRateLimiter_AllowAndRefill`
- `TestShareRateLimiter_CleansStaleEntries`
- `TestClientIP`

## 验证命令

```powershell
$env:PATH = "C:\Users\chenq\go-sdk\go\bin;" + $env:PATH
$env:GOTOOLCHAIN = "local"
$env:GOPROXY = "https://goproxy.cn,direct"
cd backend-go

go test ./internal/api/http -run "Test(ShareRateLimiter|ClientIP|IsShareTokenFormat|APIKeyHandlers_Revoke_RejectsInvalidID)" -count=1 -v
go test ./internal/api/http -count=1
```

结果：

```text
=== RUN   TestAPIKeyHandlers_Revoke_RejectsInvalidID
--- PASS: TestAPIKeyHandlers_Revoke_RejectsInvalidID (0.00s)
=== RUN   TestIsShareTokenFormat
--- PASS: TestIsShareTokenFormat (0.00s)
=== RUN   TestShareRateLimiter_AllowAndRefill
--- PASS: TestShareRateLimiter_AllowAndRefill (0.00s)
=== RUN   TestShareRateLimiter_CleansStaleEntries
--- PASS: TestShareRateLimiter_CleansStaleEntries (0.00s)
=== RUN   TestClientIP
--- PASS: TestClientIP (0.00s)
PASS
ok github.com/chenqilscy/ripple/backend-go/internal/api/http 0.250s

ok github.com/chenqilscy/ripple/backend-go/internal/api/http 0.328s
```

## 结论

第五轮性能与资源管理审查已完成。公开分享端点现在具备格式短路、TTL 约束和 IP 级限流，能降低无鉴权入口被刷时的 DB 压力与内存增长风险。
