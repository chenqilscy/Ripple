# pprof 快照脚本
#
# 用法:
#   .\scripts\loadtest\pprof-snapshot.ps1 -OutputDir .\pprof-out
#
# 前提：后端启动时设置 RIPPLE_PPROF_ADDR=:6060

param(
    [string]$PProfBase = "http://localhost:6060",
    [string]$OutputDir = ".\pprof-out"
)

if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir | Out-Null
}

$ts = Get-Date -Format "yyyyMMdd-HHmmss"

Write-Host "→ 抓取 heap 快照..."
Invoke-WebRequest -Uri "$PProfBase/debug/pprof/heap" -OutFile "$OutputDir\heap-$ts.pb.gz"

Write-Host "→ 抓取 goroutine 列表..."
Invoke-WebRequest -Uri "$PProfBase/debug/pprof/goroutine?debug=2" -OutFile "$OutputDir\goroutine-$ts.txt"

Write-Host "→ 抓取 30s CPU profile..."
Invoke-WebRequest -Uri "$PProfBase/debug/pprof/profile?seconds=30" -OutFile "$OutputDir\cpu-$ts.pb.gz" -TimeoutSec 60

Write-Host "完成。文件位于 $OutputDir"
Write-Host "分析示例: go tool pprof $OutputDir\heap-$ts.pb.gz"
