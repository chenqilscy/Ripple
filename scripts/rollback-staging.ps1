param(
  [Parameter(Mandatory = $true)]
  [string]$Ref,
  [switch]$SkipSmoke,
  [switch]$AllowDirty,
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

if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
  throw "git command not found"
}

Push-Location $repoRoot
try {
  if (-not $AllowDirty) {
    $status = git status --porcelain
    if (-not [string]::IsNullOrWhiteSpace($status)) {
      throw "working tree is dirty; commit/stash first, or pass -AllowDirty explicitly"
    }
  }

  $currentRef = (git rev-parse --abbrev-ref HEAD).Trim()
  if ([string]::IsNullOrWhiteSpace($currentRef)) {
    throw "failed to determine current git ref"
  }

  git fetch --all --tags --prune | Out-Host
  git checkout $Ref | Out-Host

  & (Join-Path $repoRoot "scripts\teardown-staging.ps1")
  if ($SkipSmoke) {
    & (Join-Path $repoRoot "scripts\bootstrap-staging.ps1") -SkipSmoke
  } else {
    & (Join-Path $repoRoot "scripts\bootstrap-staging.ps1")
  }

  Write-Host "rollback drill completed on ref: $Ref" -ForegroundColor Green
  Write-Host "to return to previous ref, run: git checkout $currentRef" -ForegroundColor Yellow
} finally {
  Pop-Location
}