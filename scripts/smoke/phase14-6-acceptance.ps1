param(
  [string]$RepoRoot = (Resolve-Path "$PSScriptRoot/../..").Path,
  [string]$BackendDir = "backend-go",
  [string]$FrontendDir = "frontend",
  [string]$GoToolchain = "local",

  # staging（可选；not provided则跳过）
  [string]$StagingBase = $env:RIPPLE_STAGING_BASE,
  [string]$StagingFrontendBase = $env:RIPPLE_STAGING_FRONTEND_BASE,

  # 跳过开关（用于细粒度调试）
  [switch]$SkipBackend,
  [switch]$SkipFrontend,
  [switch]$SkipE2E,
  [switch]$SkipStaging,

  # staging 数据清理（可选；默认仅 dry-run；需先 export RIPPLE_STAGING_SSH_HOST/USER 与 RIPPLE_STAGING_NEO4J_PASSWORD）
  [switch]$IncludeStagingCleanup,
  [switch]$StagingCleanupApply,
  [string]$StagingPgContainer = 'ripple-staging-postgres',
  [string]$StagingNeo4jContainer = 'ripple-staging-neo4j'
)

$ErrorActionPreference = "Stop"
$results = New-Object System.Collections.Generic.List[object]

function Invoke-Step {
  param([string]$Name, [scriptblock]$Body)
  Write-Host "`n=== [$Name] ==="
  $sw = [System.Diagnostics.Stopwatch]::StartNew()
  $status = "PASS"
  $err = $null
  try {
    & $Body
  } catch {
    $status = "FAIL"
    $err = $_.Exception.Message
    Write-Host "[$Name] FAIL: $err"
  }
  $sw.Stop()
  $results.Add([pscustomobject]@{
    Step = $Name
    Status = $status
    Seconds = [math]::Round($sw.Elapsed.TotalSeconds, 1)
    Error = $err
  }) | Out-Null
}

Push-Location $RepoRoot
try {
  $env:GOTOOLCHAIN = $GoToolchain

  if (-not $SkipBackend) {
    Invoke-Step "backend: go vet" {
      Push-Location $BackendDir
      try { & go vet ./...; if ($LASTEXITCODE -ne 0) { throw "go vet failed" } } finally { Pop-Location }
    }
    Invoke-Step "backend: go test -race -count=1" {
      Push-Location $BackendDir
      try { & go test -race -count=1 ./...; if ($LASTEXITCODE -ne 0) { throw "go test failed" } } finally { Pop-Location }
    }
  }

  if (-not $SkipFrontend) {
    Invoke-Step "frontend: lint" {
      Push-Location $FrontendDir
      try { & npm.cmd run lint; if ($LASTEXITCODE -ne 0) { throw "lint failed" } } finally { Pop-Location }
    }
    Invoke-Step "frontend: vitest platformAdminRegression" {
      Push-Location $FrontendDir
      try { & npm.cmd test -- --run tests/platformAdminRegression.test.ts; if ($LASTEXITCODE -ne 0) { throw "vitest failed" } } finally { Pop-Location }
    }
    Invoke-Step "frontend: build" {
      Push-Location $FrontendDir
      try { & npm.cmd run build; if ($LASTEXITCODE -ne 0) { throw "build failed" } } finally { Pop-Location }
    }
  }

  if (-not $SkipE2E) {
    Invoke-Step "frontend: playwright settings-tabs" {
      Push-Location $FrontendDir
      try {
        & npm.cmd run e2e -- e2e/settings-tabs.spec.ts --reporter=list
        if ($LASTEXITCODE -ne 0) { throw "e2e failed" }
      } finally { Pop-Location }
    }
  }

  if (-not $SkipStaging -and $StagingBase) {
    Invoke-Step "staging: backend healthz" {
      $r = Invoke-RestMethod -Method Get -Uri "$StagingBase/healthz"
      if ($r.status -ne "ok") { throw "healthz not ok: $($r | ConvertTo-Json -Compress)" }
    }
    if ($StagingFrontendBase) {
      Invoke-Step "staging: /yjs returns 400 (lake required)" {
        try {
          $resp = Invoke-WebRequest -Method Get -Uri "$StagingFrontendBase/yjs" -UseBasicParsing -ErrorAction Stop
          throw "expected 400, got $($resp.StatusCode)"
        } catch [System.Net.WebException] {
          $code = [int]$_.Exception.Response.StatusCode
          if ($code -ne 400) { throw "expected 400, got $code" }
        } catch {
          # Invoke-WebRequest 在 PS7 抛 HttpRequestException
          if ($_.ErrorDetails.Message -notmatch "lake required") {
            if ($_.Exception.Response.StatusCode.value__ -ne 400) { throw $_ }
          }
        }
      }
    }
  } elseif (-not $SkipStaging) {
    Write-Host "[skip staging] RIPPLE_STAGING_BASE not provided"
  }

  if ($IncludeStagingCleanup) {
    Invoke-Step "staging: cleanup smoke data" {
      $cleanupScript = Join-Path $PSScriptRoot 'staging-cleanup-smoke.ps1'
      if (-not (Test-Path $cleanupScript)) { throw "cleanup script missing: $cleanupScript" }
      $cleanupArgs = @('-PgContainer', $StagingPgContainer, '-Neo4jContainer', $StagingNeo4jContainer)
      if ($StagingCleanupApply) { $cleanupArgs += '-Apply' }
      & powershell -ExecutionPolicy Bypass -File $cleanupScript @cleanupArgs
      if ($LASTEXITCODE -ne 0) { throw "cleanup failed (exit=$LASTEXITCODE)" }
    }
  }

} finally {
  Pop-Location
}

Write-Host "`n=== Phase 14.6 acceptance summary ==="
$results | Format-Table -AutoSize | Out-String | Write-Host

$failed = @($results | Where-Object { $_.Status -ne "PASS" })
if ($failed.Count -gt 0) {
  Write-Host "FAILED steps: $($failed.Count)"
  exit 1
}
Write-Host "ALL PASS"
