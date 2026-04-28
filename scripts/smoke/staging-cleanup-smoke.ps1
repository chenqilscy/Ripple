param(
  [string]$RemoteHost = $env:RIPPLE_STAGING_SSH_HOST,
  [string]$RemoteUser = $env:RIPPLE_STAGING_SSH_USER,
  [string]$PgContainer = 'ripple-staging-postgres',
  [string]$Neo4jContainer = 'ripple-staging-neo4j',
  [string]$PgUser = 'ripple',
  [string]$PgDatabase = 'ripple',
  [string[]]$EmailPrefixes = @('ws_smoke_', 'phase13+', 'phase14_smoke_'),
  [string[]]$LakeNamePrefixes = @('ws-smoke-', 'ws-curl-', 'ws-origin-', 'phase13-smoke-'),
  [string]$Neo4jPassword = $env:RIPPLE_STAGING_NEO4J_PASSWORD,
  [switch]$Apply
)

$ErrorActionPreference = 'Stop'

if (-not $RemoteHost -or -not $RemoteUser) {
  throw 'RemoteHost / RemoteUser must be provided via param or RIPPLE_STAGING_SSH_HOST / RIPPLE_STAGING_SSH_USER'
}

$mode = if ($Apply) { 'APPLY' } else { 'DRY-RUN' }
Write-Host ('== Ripple staging smoke cleanup [{0}] ==' -f $mode)
Write-Host ('Remote: {0}@{1}' -f $RemoteUser, $RemoteHost)
Write-Host ('Email prefixes: {0}' -f ($EmailPrefixes -join ', '))
Write-Host ('Lake name prefixes: {0}' -f ($LakeNamePrefixes -join ', '))

$sshTarget = '{0}@{1}' -f $RemoteUser, $RemoteHost

function Invoke-RemoteStdin {
  param([string]$Cmd, [string]$StdinText)
  $StdinText | & ssh -o StrictHostKeyChecking=accept-new $sshTarget $Cmd
  if ($LASTEXITCODE -ne 0) {
    throw ('remote command (stdin) failed (exit={0}): {1}' -f $LASTEXITCODE, $Cmd)
  }
}

$emailLikes = ($EmailPrefixes | ForEach-Object { "email LIKE '$_%'" }) -join ' OR '
$pgSelect = 'SELECT id, email FROM users WHERE ' + $emailLikes + ';'
# Restrict-FK preflight: clear graylist_entries.created_by, node_revisions.editor_id, organizations.owner_id, audit_events (NO ACTION)
# Other FKs to users use CASCADE/SET NULL and are handled automatically by DELETE FROM users.
$subSelect = 'SELECT id FROM users WHERE ' + $emailLikes
$pgDelete = "BEGIN;`n" +
  "DELETE FROM graylist_entries WHERE " + $emailLikes + ";`n" +
  "DELETE FROM graylist_entries WHERE created_by IN ($subSelect);`n" +
  "DELETE FROM audit_events WHERE actor_id IN ($subSelect);`n" +
  "DELETE FROM node_revisions WHERE editor_id IN ($subSelect);`n" +
  "DELETE FROM organizations WHERE owner_id IN ($subSelect);`n" +
  "DELETE FROM users WHERE " + $emailLikes + ";`n" +
  "COMMIT;`n"

Write-Host ''
Write-Host '-- PG: list candidates --'
$pgListCmd = 'docker exec -i {0} psql -U {1} -d {2} -A -t' -f $PgContainer, $PgUser, $PgDatabase
Invoke-RemoteStdin $pgListCmd $pgSelect

if ($Apply) {
  Write-Host '-- PG: deleting --'
  $pgRunCmd = 'docker exec -i {0} psql -U {1} -d {2}' -f $PgContainer, $PgUser, $PgDatabase
  Invoke-RemoteStdin $pgRunCmd $pgDelete
} else {
  Write-Host '(dry-run) skip PG delete; pass -Apply to execute'
}

if (-not $Neo4jPassword) {
  Write-Host ''
  Write-Host '[skip Neo4j] RIPPLE_STAGING_NEO4J_PASSWORD not provided'
} else {
  $whereParts = $LakeNamePrefixes | ForEach-Object { "l.name STARTS WITH '$_'" }
  $where = $whereParts -join ' OR '
  $cypherSelect = 'MATCH (l:Lake) WHERE ' + $where + ' RETURN l.id AS id, l.name AS name;'
  $cypherDelete = 'MATCH (l:Lake) WHERE ' + $where + ' DETACH DELETE l RETURN count(l) AS deleted;'

  Write-Host ''
  Write-Host '-- Neo4j: list candidates --'
  $cypherCmd = "docker exec -i {0} cypher-shell -u neo4j -p '{1}'" -f $Neo4jContainer, $Neo4jPassword
  Invoke-RemoteStdin $cypherCmd $cypherSelect

  if ($Apply) {
    Write-Host '-- Neo4j: deleting --'
    Invoke-RemoteStdin $cypherCmd $cypherDelete
  } else {
    Write-Host '(dry-run) skip Neo4j delete; pass -Apply to execute'
  }
}

Write-Host ''
Write-Host ('== cleanup done ({0}) ==' -f $mode)