# SeaTunnelX 重启脚本：可选构建 + 可选前端，前端 PM2 名为 seatunnelx-ui
# 用法：
#   .\scripts\restart.ps1                    # 默认：构建前后端并启动
#   .\scripts\restart.ps1 -NoBuild           # 仅重启，不构建
#   .\scripts\restart.ps1 -NoFrontend        # 仅后端（可配合 -NoBuild 仅重启后端）
#   .\scripts\restart.ps1 -StopFrontend     # 仅停止前端 (pm2 stop seatunnelx-ui)

param(
    [switch]$NoBuild,      # 不构建，仅重启已有进程/二进制
    [switch]$NoFrontend,   # 不构建、不启动前端
    [switch]$StopFrontend  # 只停止前端并退出
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
if (-not (Test-Path (Join-Path $ProjectRoot "go.mod"))) {
    Write-Error "未在项目根找到 go.mod，请于项目根目录执行: .\scripts\restart.ps1"
    exit 1
}

Set-Location $ProjectRoot
$PM2_UI = "seatunnelx-ui"

if ($StopFrontend) {
    Write-Host "停止前端 (pm2 stop $PM2_UI) ..." -ForegroundColor Cyan
    pm2 stop $PM2_UI 2>$null; pm2 status
    Write-Host "完成." -ForegroundColor Green
    exit 0
}

$step = 0
$total = if ($NoBuild) { 3 } else { 5 }

if (-not $NoBuild) {
    $step++; Write-Host "[$step/$total] 构建 seatunnelx ..." -ForegroundColor Cyan
    go build -o seatunnelx .
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    Write-Host "      seatunnelx 构建完成." -ForegroundColor Green

    $step++; Write-Host "[$step/$total] 构建 seatunnelx-agent ..." -ForegroundColor Cyan
    Set-Location (Join-Path $ProjectRoot "agent")
    go build -o seatunnelx-agent ./cmd/main.go
    if ($LASTEXITCODE -ne 0) { Set-Location $ProjectRoot; exit $LASTEXITCODE }
    Set-Location $ProjectRoot
    Write-Host "      seatunnelx-agent 构建完成." -ForegroundColor Green

    $LibAgent = Join-Path $ProjectRoot "lib\agent"
    if (Test-Path $LibAgent) {
        $agentBin = Join-Path $ProjectRoot "agent\seatunnelx-agent"
        if ($env:OS -eq "Windows_NT") { $agentBin = $agentBin + ".exe" }
        if (Test-Path $agentBin) {
            $dest = Join-Path $LibAgent "seatunnelx-agent-windows-amd64.exe"
            if ($env:OS -ne "Windows_NT") { $dest = Join-Path $LibAgent "seatunnelx-agent-linux-amd64" }
            Copy-Item $agentBin $dest -Force -ErrorAction SilentlyContinue
            Write-Host "      已同步 agent 到 lib/agent." -ForegroundColor Gray
        }
    }
}

$step++; Write-Host "[$step/$total] 停止已有 seatunnelx 进程 ..." -ForegroundColor Cyan
Get-Process -Name "seatunnelx" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1
Write-Host "      已停止." -ForegroundColor Green

$step++; Write-Host "[$step/$total] 启动 seatunnelx api ..." -ForegroundColor Cyan
$binName = "seatunnelx"
if ($env:OS -eq "Windows_NT") { $binName = "seatunnelx.exe" }
$bin = Join-Path $ProjectRoot $binName
if (-not (Test-Path $bin)) {
    Write-Error "未找到 $binName，请先执行一次无 -NoBuild 的 restart 或手动构建"
    exit 1
}
Start-Process -FilePath $bin -ArgumentList "api" -WorkingDirectory $ProjectRoot -NoNewWindow
Write-Host "      已启动 (API 默认 http://localhost:8000)." -ForegroundColor Green

if (-not $NoFrontend) {
    $step++; Write-Host "[$step/$total] 前端 ..." -ForegroundColor Cyan
    $frontendDir = Join-Path $ProjectRoot "frontend"
    if (-not (Test-Path (Join-Path $frontendDir "package.json"))) {
        Write-Host "      未找到 frontend/package.json，跳过前端." -ForegroundColor Yellow
    } else {
        Set-Location $frontendDir
        if (-not $NoBuild) {
            pnpm run build
            if ($LASTEXITCODE -ne 0) { Set-Location $ProjectRoot; exit $LASTEXITCODE }
        }
        pm2 delete $PM2_UI 2>$null
        pm2 start "pnpm start -- -p 80" --name $PM2_UI
        pm2 status
        Set-Location $ProjectRoot
        Write-Host "      前端已启动 (PM2: $PM2_UI，端口 80)." -ForegroundColor Green
    }
} else {
    Write-Host "      跳过前端 (NoFrontend)." -ForegroundColor Gray
}

Write-Host "完成." -ForegroundColor Green
