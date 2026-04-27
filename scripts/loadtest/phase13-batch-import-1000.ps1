[CmdletBinding()]
param(
    [string]$Base = "http://127.0.0.1:8000",
    [int]$NodeCount = 1000,
    [double]$ThresholdSeconds = 5.0
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Invoke-Json {
    param(
        [string]$Method,
        [string]$Url,
        [object]$Body,
        [string]$Token
    )

    $headers = @{}
    if ($Token) {
        $headers["Authorization"] = "Bearer $Token"
    }

    if ($null -eq $Body) {
        return Invoke-RestMethod -Method $Method -Uri $Url -Headers $headers -TimeoutSec 30
    }

    $json = $Body | ConvertTo-Json -Depth 8 -Compress
    return Invoke-RestMethod -Method $Method -Uri $Url -Headers $headers -Body $json -ContentType "application/json" -TimeoutSec 60
}

$repoRoot = Resolve-Path "$PSScriptRoot/../.."
$ts = Get-Date -Format "yyyyMMdd-HHmmss"
$reportPath = Join-Path $repoRoot "docs/dev/Phase13-batch-import-1000-baseline-$ts.md"

$email = "batch1000+$ts@ripple.local"
$password = "Test1234!"

$null = Invoke-Json -Method "POST" -Url "$Base/api/v1/auth/register" -Body @{
    email = $email
    password = $password
    display_name = "batch-1000"
} -Token ""

$login = Invoke-Json -Method "POST" -Url "$Base/api/v1/auth/login" -Body @{
    email = $email
    password = $password
} -Token ""
$token = $login.access_token
if (-not $token) {
    throw "Login failed"
}

$lake = Invoke-Json -Method "POST" -Url "$Base/api/v1/lakes" -Body @{
    name = "batch1000-$($ts.Substring(0, 8))"
    is_public = $false
} -Token $token

$lakeId = $lake.id
if (-not $lakeId) {
    throw "Create lake failed"
}

for ($i = 0; $i -lt 30; $i++) {
    try {
        $got = Invoke-Json -Method "GET" -Url "$Base/api/v1/lakes/$lakeId" -Body $null -Token $token
        if ($got.id -eq $lakeId) {
            break
        }
    } catch {
        if ($i -eq 29) { throw }
    }
    Start-Sleep -Milliseconds 200
}

$nodes = @()
for ($i = 1; $i -le $NodeCount; $i++) {
    $nodes += @{ type = "TEXT"; content = "phase13-batch1000-item-$i" }
}

$body = @{ nodes = $nodes }

$sw = [System.Diagnostics.Stopwatch]::StartNew()
$result = Invoke-Json -Method "POST" -Url "$Base/api/v1/lakes/$lakeId/nodes/batch" -Body $body -Token $token
$sw.Stop()

$elapsedSec = [Math]::Round($sw.Elapsed.TotalSeconds, 4)
$created = 0
if ($null -ne $result.created) {
    $created = [int]$result.created
}
$pass = ($created -eq $NodeCount) -and ($elapsedSec -le $ThresholdSeconds)

$md = @"
# Phase13 Batch Import 1000 Baseline

- Time: $ts
- Base: $Base
- Lake ID: $lakeId
- Node count: $NodeCount
- Created: $created
- Elapsed: ${elapsedSec}s
- Threshold: ${ThresholdSeconds}s
- Result: $(if ($pass) { "PASS" } else { "FAIL" })

## Request

- Endpoint: POST /api/v1/lakes/{id}/nodes/batch
- Payload nodes: $NodeCount
- Type: TEXT

## Acceptance

- created == $NodeCount
- elapsed <= ${ThresholdSeconds}s
"@

New-Item -ItemType Directory -Path (Split-Path $reportPath) -Force | Out-Null
Set-Content -Path $reportPath -Value $md -Encoding UTF8

Write-Host "Report: $reportPath"
if (-not $pass) {
    throw "Batch import baseline failed: created=$created elapsed=${elapsedSec}s threshold=${ThresholdSeconds}s"
}
