# TD-04 · WS 1000 跨机压测一键脚本（PowerShell）
#
# 用途：
#   1. 在压测客户端机器（不与后端同机）上一键完成：
#      注册测试账号 → 登录拿 JWT → 建湖 → 1000 并发 WS 建连 -hold 30s
#   2. 采集后端 /metrics 与 ws_connect 自报指标，输出 markdown 报告。
#
# 前置：
#   - 后端已部署并暴露 http://${BASE}/healthz 与 ws://${BASE}/api/v1/lakes/{id}/ws
#   - 压测机能连通后端
#   - Go SDK 在 PATH（或 prepend C:\Users\chenq\go-sdk\go\bin）
#
# 用法：
#   ./scripts/loadtest/td04-ws-1000.ps1 -Base http://fn.cky:8000 -Conc 1000 -Hold 30s
#
# 输出：
#   docs/dev/TD-04-WS压测报告-<timestamp>.md

[CmdletBinding()]
param(
    [string]$Base = "http://fn.cky:8000",
    [int]$Conc = 1000,
    [string]$Hold = "30s"
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path "$PSScriptRoot/../.."
$ts = Get-Date -Format "yyyyMMdd-HHmmss"
$report = Join-Path $repoRoot "docs/dev/TD-04-WS压测报告-$ts.md"

function Probe-Backend {
    param([string]$Url)
    try {
        $r = Invoke-WebRequest -Uri "$Url/healthz" -TimeoutSec 5 -UseBasicParsing
        return $r.StatusCode -eq 200
    } catch { return $false }
}

if (-not (Probe-Backend $Base)) {
    Write-Error "后端 $Base/healthz 不可达，请先启动 backend-go 或纠正 -Base 参数"
    exit 1
}

# 1. 注册并登录测试账号
$email = "td04+$ts@ripple.local"
$pwd = "Test1234!"
$register = @{ email=$email; password=$pwd; display_name="td04" } | ConvertTo-Json -Compress
Invoke-RestMethod -Uri "$Base/api/v1/auth/register" -Method Post -Body $register -ContentType "application/json" | Out-Null
$login = @{ email=$email; password=$pwd } | ConvertTo-Json -Compress
$resp = Invoke-RestMethod -Uri "$Base/api/v1/auth/login" -Method Post -Body $login -ContentType "application/json"
$token = $resp.access_token
if (-not $token) { Write-Error "登录失败"; exit 1 }

# 2. 建湖
$lakeBody = @{ name="td04-$ts"; description="TD-04 1000 WS 压测湖" } | ConvertTo-Json -Compress
$lake = Invoke-RestMethod -Uri "$Base/api/v1/lakes" -Method Post -Body $lakeBody -ContentType "application/json" -Headers @{Authorization="Bearer $token"}
$lakeId = $lake.id
if (-not $lakeId) { Write-Error "建湖失败"; exit 1 }

# 3. 抓基线 /metrics
$metricsBefore = (Invoke-WebRequest -Uri "$Base/metrics" -UseBasicParsing).Content
$metricsBeforeFile = Join-Path $env:TEMP "td04-metrics-before-$ts.txt"
$metricsBefore | Set-Content -Path $metricsBeforeFile -Encoding UTF8

# 4. 启动 ws_connect（同步等待结束，捕获 stdout）
Push-Location (Join-Path $repoRoot "backend-go")
$wsBase = $Base -replace "^http://","ws://" -replace "^https://","wss://"
$wsUrl = "$wsBase/api/v1/lakes/$lakeId/ws"
$startedAt = Get-Date
$wsOutput = & go run ./cmd/loadtest/ws_connect -url $wsUrl -token $token -conc $Conc -hold $Hold 2>&1 | Out-String
$elapsed = (Get-Date) - $startedAt
Pop-Location

# 5. 抓尾基线 /metrics
$metricsAfter = (Invoke-WebRequest -Uri "$Base/metrics" -UseBasicParsing).Content
$metricsAfterFile = Join-Path $env:TEMP "td04-metrics-after-$ts.txt"
$metricsAfter | Set-Content -Path $metricsAfterFile -Encoding UTF8

# 6. 写报告
$md = @"
# TD-04 · 1000 WS 跨机压测报告

- 时间：$ts
- Base：$Base
- WS URL：$wsUrl
- 并发：$Conc
- 持续：$Hold
- 实际耗时：$([int]$elapsed.TotalSeconds) s

## ws_connect 输出

\`\`\`
$wsOutput
\`\`\`

## /metrics 基线（前后对比）

- before：$metricsBeforeFile
- after：$metricsAfterFile

提示：使用 \`Compare-Object\` 或导入 Grafana 对照查看。

## 验收

- [ ] 1000 连接全部 Dial 成功（错误数 0）
- [ ] 后端 RSS 增长 < 500MB（依据机器配置调整阈值）
- [ ] CPU 峰值 < 80%
- [ ] /metrics 中 \`ripple_active_ws_connections\`（如有）峰值 ≥ 1000
"@

New-Item -ItemType Directory -Path (Split-Path $report) -Force | Out-Null
$md | Set-Content -Path $report -Encoding UTF8
Write-Host "报告已生成：$report"
