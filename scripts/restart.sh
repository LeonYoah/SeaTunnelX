#!/usr/bin/env bash
###
 # @Author: Leon Yoah 1733839298@qq.com
 # @Date: 2026-02-23 12:41:03
 # @LastEditors: Leon Yoah 1733839298@qq.com
 # @LastEditTime: 2026-02-23 17:51:46
 # @FilePath: \SeaTunnelX\scripts\restart.sh
 # @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
###
# SeaTunnelX 重启脚本：可选构建 + 可选前端，前端 PM2 名为 seatunnelx-ui
# 用法：
#   ./scripts/restart.sh                  # 默认：构建前后端并启动
#   ./scripts/restart.sh --no-build       # 仅重启，不构建
#   ./scripts/restart.sh --no-frontend    # 仅后端（可配合 --no-build 仅重启后端）
#   ./scripts/restart.sh --stop-frontend # 仅停止前端 (pm2 stop seatunnelx-ui)

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

NO_BUILD=false
NO_FRONTEND=false
STOP_FRONTEND=false
for arg in "$@"; do
    case "$arg" in
        --no-build)      NO_BUILD=true ;;
        --no-frontend)   NO_FRONTEND=true ;;
        --stop-frontend) STOP_FRONTEND=true ;;
    esac
done

PM2_UI="seatunnelx-ui"

if [[ ! -f go.mod ]]; then
    echo "未在项目根找到 go.mod，请于项目根目录执行: ./scripts/restart.sh"
    exit 1
fi

if $STOP_FRONTEND; then
    echo "停止前端 (pm2 stop $PM2_UI) ..."
    pm2 stop "$PM2_UI" 2>/dev/null || true
    pm2 status
    echo "完成."
    exit 0
fi

if $NO_BUILD; then total=3; else total=5; fi
step=0

if ! $NO_BUILD; then
    step=$((step+1)); echo "[$step/$total] 构建 seatunnelx ..."
    go build -o seatunnelx .
    echo "      seatunnelx 构建完成."

    step=$((step+1)); echo "[$step/$total] 构建 seatunnelx-agent ..."
    cd agent
    go build -o seatunnelx-agent ./cmd/main.go
    cd "$PROJECT_ROOT"
    echo "      seatunnelx-agent 构建完成."

    if [[ -d lib/agent ]] && [[ -f agent/seatunnelx-agent ]]; then
        cp -f agent/seatunnelx-agent lib/agent/seatunnelx-agent-linux-amd64
        echo "      已同步 agent 到 lib/agent."
    fi
fi

step=$((step+1)); echo "[$step/$total] 停止已有 seatunnelx 进程 ..."
pkill -f seatunnelx || true
sleep 1
echo "      已停止."

step=$((step+1)); echo "[$step/$total] 启动 seatunnelx api ..."
if [[ ! -f ./seatunnelx ]]; then
    echo "未找到 ./seatunnelx，请先执行一次无 --no-build 的 restart 或手动构建"
    exit 1
fi
nohup ./seatunnelx api > seatunnelx.log 2>&1 &
echo "      已启动 (API 默认 http://localhost:8000，日志 seatunnelx.log)."

if $NO_FRONTEND; then
    echo "      跳过前端 (--no-frontend)."
else
    step=$((step+1)); echo "[$step/$total] 前端 ..."
    if [[ ! -f frontend/package.json ]]; then
        echo "      未找到 frontend/package.json，跳过前端."
    else
        cd frontend
        if ! $NO_BUILD; then
            pnpm run build
        fi
        pm2 delete "$PM2_UI" 2>/dev/null || true
        pm2 start "pnpm start -- -p 80" --name "$PM2_UI"
        pm2 status
        cd "$PROJECT_ROOT"
        echo "      前端已启动 (PM2: $PM2_UI，端口 80)."
    fi
fi

echo "完成."
