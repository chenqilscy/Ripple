# TD-04 WS cross-machine load test helper.
#
# Flow:
#   1. Register a temporary user and log in.
#   2. Create one lake.
#   3. Poll GET /api/v1/lakes/{id} until the lake projection becomes readable.
#   4. Capture /metrics before the run.
#   5. Run ws_connect with the requested concurrency and hold time.
#   6. Capture /metrics after the run and write a markdown report.

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

function Test-BackendHealth {
    param([string]$Url)
    try {
        $r = Invoke-WebRequest -Uri "${Url}/healthz" -TimeoutSec 5 -UseBasicParsing
        return $r.StatusCode -eq 200
    } catch { return $false }
}

function Wait-LakeReady {
    param(
        [string]$Url,
        [string]$LakeId,
        [string]$Token,
        [int]$MaxAttempts = 30,
        [int]$DelayMs = 200
    )

    for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
        try {
            $resp = Invoke-WebRequest -Uri "${Url}/api/v1/lakes/$LakeId" -Headers @{ Authorization = "Bearer $Token" } -TimeoutSec 5 -UseBasicParsing
            if ($resp.StatusCode -eq 200) {
                return $attempt
            }
        } catch {
            if ($attempt -eq $MaxAttempts) {
                throw
            }
        }
        Start-Sleep -Milliseconds $DelayMs
    }

    throw "Lake $LakeId did not become readable within $MaxAttempts attempts."
}

if (-not (Test-BackendHealth $Base)) {
    Write-Error "Backend ${Base}/healthz is unreachable. Start the backend or fix -Base."
    exit 1
}

# 1. 注册并登录测试账号
$email = "td04+$ts@ripple.local"
$password = "Test1234!"
$register = @{ email=$email; password=$password; display_name="td04" } | ConvertTo-Json -Compress
Invoke-RestMethod -Uri "${Base}/api/v1/auth/register" -Method Post -Body $register -ContentType "application/json" | Out-Null
$login = @{ email=$email; password=$password } | ConvertTo-Json -Compress
$resp = Invoke-RestMethod -Uri "${Base}/api/v1/auth/login" -Method Post -Body $login -ContentType "application/json"
$token = $resp.access_token
if (-not $token) { Write-Error "Login failed"; exit 1 }

# 2. 建湖
$lakeBody = @{ name="td04-$ts"; description="TD-04 WS load test lake" } | ConvertTo-Json -Compress
$lake = Invoke-RestMethod -Uri "${Base}/api/v1/lakes" -Method Post -Body $lakeBody -ContentType "application/json" -Headers @{Authorization="Bearer $token"}
$lakeId = $lake.id
if (-not $lakeId) { Write-Error "Lake creation failed"; exit 1 }
$lakeReadyAttempts = Wait-LakeReady -Url $Base -LakeId $lakeId -Token $token

# 3. 抓基线 /metrics
$metricsBefore = (Invoke-WebRequest -Uri "${Base}/metrics" -UseBasicParsing).Content
$metricsBeforeFile = Join-Path $env:TEMP "td04-metrics-before-$ts.txt"
$metricsBefore | Set-Content -Path $metricsBeforeFile -Encoding UTF8

# 4. 启动 ws_connect（同步等待结束，捕获 stdout）
Push-Location (Join-Path $repoRoot "backend-go")
$wsBase = $Base -replace "^http://","ws://" -replace "^https://","wss://"
$wsUrl = "${wsBase}/api/v1/lakes/$lakeId/ws"
$wsBinary = Join-Path $env:TEMP "ws_connect-$ts.exe"
$prevGoos = $env:GOOS
$prevGoarch = $env:GOARCH
$startedAt = Get-Date
try {
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    & go build -o $wsBinary ./cmd/loadtest/ws_connect
    if ($LASTEXITCODE -ne 0) {
        throw "go build ./cmd/loadtest/ws_connect failed"
    }
    $wsOutput = & $wsBinary -url $wsUrl -token $token -conc $Conc -hold $Hold 2>&1 | Out-String
} finally {
    if ($null -ne $prevGoos) {
        $env:GOOS = $prevGoos
    } else {
        Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    }
    if ($null -ne $prevGoarch) {
        $env:GOARCH = $prevGoarch
    } else {
        Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    }
    Remove-Item $wsBinary -Force -ErrorAction SilentlyContinue
}
$elapsed = (Get-Date) - $startedAt
Pop-Location

# 5. 抓尾基线 /metrics
$metricsAfter = (Invoke-WebRequest -Uri "${Base}/metrics" -UseBasicParsing).Content
$metricsAfterFile = Join-Path $env:TEMP "td04-metrics-after-$ts.txt"
$metricsAfter | Set-Content -Path $metricsAfterFile -Encoding UTF8

# 6. 写报告
$report = Join-Path $repoRoot "docs/dev/TD-04-WS-loadtest-report-$ts.md"
$md = @"
# TD-04 WS Cross-Machine Load Test Report

- Time: $ts
- Base: $Base
- WS URL: $wsUrl
- Concurrency: $Conc
- Hold: $Hold
- Elapsed: $([int]$elapsed.TotalSeconds) s
- Lake ready after: $lakeReadyAttempts poll(s)

## ws_connect output

\`\`\`
$wsOutput
\`\`\`

## Metrics snapshots

- before: $metricsBeforeFile
- after: $metricsAfterFile

Use Compare-Object or your metrics dashboard to inspect the delta.

## Checklist

- [ ] All dial attempts succeeded
- [ ] Backend RSS growth stayed within budget
- [ ] Peak CPU stayed within budget
- [ ] /metrics shows ripple_ws_connections near the target concurrency
"@

New-Item -ItemType Directory -Path (Split-Path $report) -Force | Out-Null
$md | Set-Content -Path $report -Encoding UTF8
Write-Host "Report written: $report"
