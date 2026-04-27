param(
  [Parameter(Mandatory = $true)]
  [ValidateSet("redis", "neo4j", "yjs-bridge")]
  [string]$Scenario,
  [int]$DurationSeconds = 15,
  [switch]$NoRecover,
  [string]$ComposeFile = "docker-compose.staging.yml"
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$composePath = Join-Path $repoRoot $ComposeFile

if (-not (Test-Path $composePath)) {
  throw "compose file not found: $composePath"
}

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
  throw "docker command not found; run this script on a machine with Docker installed"
}

function Get-ServiceName([string]$name) {
  switch ($name) {
    "redis" { return "redis" }
    "neo4j" { return "neo4j" }
    "yjs-bridge" { return "yjs-bridge" }
    default { throw "unsupported scenario: $name" }
  }
}

function Get-ContainerName([string]$name) {
  switch ($name) {
    "redis" { return "ripple-staging-redis" }
    "neo4j" { return "ripple-staging-neo4j" }
    "yjs-bridge" { return "ripple-staging-yjs-bridge" }
    default { throw "unsupported scenario: $name" }
  }
}

$service = Get-ServiceName $Scenario
$container = Get-ContainerName $Scenario

Push-Location $repoRoot
try {
  Write-Host "Stopping $service ..." -ForegroundColor Yellow
  docker compose -f $ComposeFile stop $service | Out-Host

  if ($NoRecover) {
    Write-Host "NoRecover enabled; service remains stopped." -ForegroundColor Red
    return
  }

  Write-Host "Waiting $DurationSeconds seconds before recovery ..." -ForegroundColor Cyan
  Start-Sleep -Seconds $DurationSeconds

  Write-Host "Starting $service ..." -ForegroundColor Green
  docker compose -f $ComposeFile up -d $service | Out-Host

  for ($i = 0; $i -lt 20; $i++) {
    $state = docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' $container 2>$null
    if ($state -eq 'healthy' -or $state -eq 'running') {
      break
    }
    Start-Sleep -Seconds 2
  }

  Write-Host "Recovery triggered. Check healthz and smoke results manually." -ForegroundColor Green
} finally {
  Pop-Location
}