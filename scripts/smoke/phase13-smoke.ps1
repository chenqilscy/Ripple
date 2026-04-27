param(
  [string]$Base = "http://127.0.0.1:18000"
)

$ErrorActionPreference = "Stop"

function Invoke-Json($Method, $Uri, $Body, $Token) {
  $headers = @{}
  if ($Token) {
    $headers["Authorization"] = "Bearer $Token"
  }
  if ($null -ne $Body) {
    return Invoke-RestMethod -Method $Method -Uri $Uri -Headers $headers -ContentType "application/json" -Body ($Body | ConvertTo-Json -Depth 8)
  }
  return Invoke-RestMethod -Method $Method -Uri $Uri -Headers $headers
}

$ts = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$email = "phase13+$ts@ripple.local"
$password = "Phase13-password-123"

Write-Host "== health =="
$health = Invoke-RestMethod -Method Get -Uri "$Base/healthz"
if ($health.status -ne "ok") {
  throw "healthz not ok"
}

Write-Host "== register =="
$null = Invoke-Json POST "$Base/api/v1/auth/register" @{
  email = $email
  password = $password
  display_name = "Phase13 Smoke"
} $null

Write-Host "== login =="
$login = Invoke-Json POST "$Base/api/v1/auth/login" @{
  email = $email
  password = $password
} $null
$token = $login.access_token

Write-Host "== create lake =="
$lake = Invoke-Json POST "$Base/api/v1/lakes" @{
  name = "Phase13 Lake"
  description = "staging smoke"
  is_public = $false
} $token

Start-Sleep -Seconds 3

Write-Host "== create node =="
$node = Invoke-Json POST "$Base/api/v1/nodes" @{
  lake_id = $lake.id
  type = "TEXT"
  content = "phase13 smoke node"
  position = @{ x = 0; y = 0; z = 0 }
} $token

Write-Host "== search =="
$search = Invoke-RestMethod -Method Get -Uri "$Base/api/v1/search?q=phase13&lake_id=$($lake.id)&limit=5" -Headers @{ Authorization = "Bearer $token" }
if (-not $search.nodes -or $search.nodes.Count -lt 1) {
  throw "search returned no nodes"
}

Write-Host "== batch import =="
$batch = Invoke-Json POST "$Base/api/v1/lakes/$($lake.id)/nodes/batch" @{
  items = @(
    @{ type = "TEXT"; content = "batch-a"; position = @{ x = 1; y = 1; z = 0 } },
    @{ type = "TEXT"; content = "batch-b"; position = @{ x = 2; y = 2; z = 0 } }
  )
} $token
if ($batch.created -lt 2) {
  throw "batch import did not create expected nodes"
}

Write-Host "== api key =="
$apiKey = Invoke-Json POST "$Base/api/v1/api_keys" @{
  name = "phase13-smoke"
  scopes = @("lakes:read")
} $token
if ([string]::IsNullOrWhiteSpace($apiKey.raw_key)) {
  throw "api key raw_key missing"
}

Write-Host "== org create + invite by email =="
$org = Invoke-Json POST "$Base/api/v1/organizations" @{
  name = "Phase13 Org"
  slug = "phase13-$ts"
  description = "staging smoke"
} $token

$invite = Invoke-Json POST "$Base/api/v1/organizations/$($org.id)/members/by_email" @{
  email = $email
  role = "MEMBER"
} $token

Write-Host "== audit logs =="
$audit = Invoke-RestMethod -Method Get -Uri "$Base/api/v1/audit_logs?limit=10" -Headers @{ Authorization = "Bearer $token" }
if ($null -eq $audit.items) {
  throw "audit logs missing"
}

Write-Host "OK phase13 smoke passed" -ForegroundColor Green
Write-Host "lake_id=$($lake.id) node_id=$($node.id) org_id=$($org.id) invited_user=$($invite.user_id)"