param(
  [switch]$KeepVolumes,
  [string]$ComposeFile = "docker-compose.staging.yml"
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$composePath = Join-Path $repoRoot $ComposeFile

if (-not (Test-Path $composePath)) {
  throw "compose file not found: $composePath"
}

Push-Location $repoRoot
try {
  $args = @("compose", "-f", $ComposeFile, "down", "--remove-orphans")
  if (-not $KeepVolumes) {
    $args += "-v"
  }
  docker @args | Out-Host
} finally {
  Pop-Location
}