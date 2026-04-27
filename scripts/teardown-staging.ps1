param(
  [switch]$KeepVolumes,
  [switch]$DryRun,
  [string]$ComposeFile = "docker-compose.staging.yml"
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$composePath = Join-Path $repoRoot $ComposeFile

if (-not (Test-Path $composePath)) {
  throw "compose file not found: $composePath"
}

if (-not $DryRun -and -not (Get-Command docker -ErrorAction SilentlyContinue)) {
  throw "docker command not found; run this script on a machine with Docker installed"
}

Push-Location $repoRoot
try {
  $args = @("compose", "-f", $ComposeFile, "down", "--remove-orphans")
  if (-not $KeepVolumes) {
    $args += "-v"
  }
  if ($DryRun) {
    Write-Host "DRY RUN: docker $($args -join ' ')"
    return
  }
  docker @args | Out-Host
} finally {
  Pop-Location
}