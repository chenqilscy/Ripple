# Phase 13 第四轮深度代码审查：安全性与数据隔离（2026-04-27）

## 范围

- `backend-go/internal/api/http`
- `backend-go/internal/service`
- `backend-go/internal/store`

重点检查：公开端点、管理端点输入校验、跨用户/跨湖/跨组织数据隔离、token/API key 相关路径。

## 发现与修复

### 1. Public share token 缺少格式短路校验

- 文件：`backend-go/internal/api/http/handlers_share.go`
- 风险：公开端点 `/api/v1/share/{token}` 无需鉴权；任意长度/字符的 token 都会进入仓库查询路径，容易放大无效请求对 DB 的压力。
- 修复：新增 `isShareTokenFormat`，仅接受当前生成器产出的 43 字符 URL-safe base64 token；非法格式统一返回 `404 share not found`。
- 验证：新增 `TestIsShareTokenFormat`。

### 2. Share TTL 缺少范围限制

- 文件：`backend-go/internal/api/http/handlers_share.go`
- 风险：`ttl_hours` 可传入负数或过大整数，导致语义不清或时间计算风险。
- 修复：限制 `ttl_hours` 范围为 `0..8760`，其中 `0` 表示永不过期。

### 3. API Key revoke 缺少 ID 格式校验

- 文件：`backend-go/internal/api/http/handlers_api_key.go`
- 风险：`DELETE /api/v1/api_keys/{id}` 对明显非法 ID 仍进入 repo 层，增加无效 DB 压力，也不利于统一错误语义。
- 修复：handler 层先用 `uuid.Parse` 校验，非法 ID 返回 `400 invalid api key id`，且不调用 repo。
- 验证：新增 `TestAPIKeyHandlers_Revoke_RejectsInvalidID`。

## 验证命令

```powershell
$env:PATH = "C:\Users\chenq\go-sdk\go\bin;" + $env:PATH
$env:GOTOOLCHAIN = "local"
$env:GOPROXY = "https://goproxy.cn,direct"
cd backend-go

go test ./internal/api/http -run "Test(IsShareTokenFormat|APIKeyHandlers_Revoke_RejectsInvalidID)" -count=1 -v
go test ./internal/api/http -count=1
```

结果：

```text
=== RUN   TestAPIKeyHandlers_Revoke_RejectsInvalidID
--- PASS: TestAPIKeyHandlers_Revoke_RejectsInvalidID (0.00s)
=== RUN   TestIsShareTokenFormat
--- PASS: TestIsShareTokenFormat (0.00s)
PASS
ok github.com/chenqilscy/ripple/backend-go/internal/api/http 2.064s

ok github.com/chenqilscy/ripple/backend-go/internal/api/http 0.421s
```

## 结论

第四轮安全性与数据隔离审查已完成并修复本轮确认问题。后续第五轮建议继续聚焦性能与资源管理，特别是公开端点限流、WebSocket 资源释放和后台 goroutine 生命周期。
