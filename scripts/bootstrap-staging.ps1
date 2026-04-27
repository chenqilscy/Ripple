param(
  [switch]$NoBuild,
  [switch]$SkipSmoke,
  [string]$ComposeFile = "docker-compose.staging.yml"
)

$ErrorActionPreference = "Stop"

function Write-Step($message) {
  Write-Host "`n== $message ==" -ForegroundColor Cyan
}

function Get-EnvOrDefault($name, $defaultValue) {
  $value = [Environment]::GetEnvironmentVariable($name, "Process")
  if ([string]::IsNullOrWhiteSpace($value)) {
    return $defaultValue
  }
  return $value
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$composePath = Join-Path $repoRoot $ComposeFile
$backendPort = Get-EnvOrDefault "STAGING_BACKEND_PORT" "18000"
$frontendPort = Get-EnvOrDefault "STAGING_FRONTEND_PORT" "14173"
$yjsPort = Get-EnvOrDefault "STAGING_YJS_PORT" "17790"

if (-not (Test-Path $composePath)) {
  throw "compose file not found: $composePath"
}

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
  throw "docker command not found; run this script on a machine with Docker installed"
}

$requiredEnv = @("PG_PASSWORD", "NEO4J_PASSWORD", "REDIS_PASSWORD", "JWT_SECRET")
$missing = @()
foreach ($name in $requiredEnv) {
  if ([string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($name, "Process"))) {
    $missing += $name
  }
}
if ($missing.Count -gt 0) {
  throw "missing env vars: $($missing -join ', ')"
}

Push-Location $repoRoot
try {
  Write-Step "docker compose pull"
  docker compose -f $ComposeFile pull postgres neo4j redis | Out-Host

  $upArgs = @("compose", "-f", $ComposeFile, "up", "-d")
  if ($NoBuild) {
    $upArgs += "--no-build"
  } else {
    $upArgs += "--build"
  }

  Write-Step "docker compose up"
  docker @upArgs | Out-Host

  Write-Step "wait backend healthz"
  $healthUrl = "http://127.0.0.1:$backendPort/healthz"
  $ok = $false
  for ($i = 0; $i -lt 40; $i++) {
    try {
      $resp = Invoke-RestMethod -Uri $healthUrl -Method Get -TimeoutSec 3
      if ($resp.status -eq "ok") {
        $ok = $true
        break
      }
    } catch {
    }
    Start-Sleep -Seconds 3
  }
  if (-not $ok) {
    throw "backend healthcheck failed: $healthUrl"
  }

  if (-not $SkipSmoke) {
    Write-Step "phase13 smoke"
    & (Join-Path $repoRoot "scripts\smoke\phase13-smoke.ps1") -Base "http://127.0.0.1:$backendPort" | Out-Host
  }

  Write-Step "done"
  Write-Host "frontend: http://127.0.0.1:$frontendPort" -ForegroundColor Green
  Write-Host "backend:  http://127.0.0.1:$backendPort" -ForegroundColor Green
  Write-Host "yjs:      ws://127.0.0.1:$yjsPort/yjs" -ForegroundColor Green
} finally {
  Pop-Location
}
