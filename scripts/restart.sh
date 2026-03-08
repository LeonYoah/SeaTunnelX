#!/usr/bin/env bash
# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# SeaTunnelX 重启脚本：
# - 后端使用 PM2 启动（seatunnelx-api）
# - 前端默认使用 Next.js standalone 产物 + PM2 启动（seatunnelx-ui）
# - 可选通过 --frontend-dev 改为 pnpm run dev + PM2 启动，便于前端开发时避免重复构建
# - 启动前会检测并清理同名 PM2 进程，最后执行 pm2 save
#
# 用法：
#   ./scripts/restart.sh                  # 默认：构建前后端并重启
#   ./scripts/restart.sh --no-build       # 不构建，直接重启（需已有可用产物）
#   ./scripts/restart.sh --frontend-dev   # 前端改用 pnpm run dev 启动（跳过前端 build）
#   ./scripts/restart.sh --no-frontend    # 仅重启后端
#   ./scripts/restart.sh --stop-frontend  # 仅停止前端 PM2 进程

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

print_help() {
  cat <<'EOF'
SeaTunnelX 重启脚本

用法:
  ./scripts/restart.sh [选项]

选项:
  --no-build       不构建，直接重启（需已有可用产物）
  --frontend-dev   前端改用 pnpm run dev 启动（跳过前端 build）
  --no-frontend    仅重启后端
  --stop-frontend  仅停止前端 PM2 进程并退出
  -h, --help       显示本帮助

环境变量:
  PM2_API                        后端 PM2 进程名，默认 seatunnelx-api
  PM2_UI                         前端 PM2 进程名，默认 seatunnelx-ui
  CONFIG_PATH                    后端配置文件路径，默认 ./config.yaml
  APP_EXTERNAL_URL               写入 config.yaml 的 app.external_url，默认 http://127.0.0.1:8000
  FRONTEND_PORT                  前端端口，默认 80
  NEXT_PUBLIC_BACKEND_BASE_URL   前端访问后端的基础地址，默认 http://127.0.0.1:8000
EOF
}

NO_BUILD=false
NO_FRONTEND=false
STOP_FRONTEND=false
FRONTEND_DEV=false
for arg in "$@"; do
  case "$arg" in
    -h|--help)
      print_help
      exit 0
      ;;
    --no-build) NO_BUILD=true ;;
    --frontend-dev) FRONTEND_DEV=true ;;
    --no-frontend) NO_FRONTEND=true ;;
    --stop-frontend) STOP_FRONTEND=true ;;
    *)
      echo "未知参数: $arg"
      echo
      print_help
      exit 1
      ;;
  esac
done

PM2_API="${PM2_API:-seatunnelx-api}"
PM2_UI="${PM2_UI:-seatunnelx-ui}"
CONFIG_PATH="${CONFIG_PATH:-$PROJECT_ROOT/config.yaml}"
APP_EXTERNAL_URL="${APP_EXTERNAL_URL:-http://127.0.0.1:8000}"
FRONTEND_PORT="${FRONTEND_PORT:-80}"
NEXT_PUBLIC_BACKEND_BASE_URL="${NEXT_PUBLIC_BACKEND_BASE_URL:-http://127.0.0.1:8000}"
FRONTEND_DIR="$PROJECT_ROOT/frontend"
FRONTEND_STANDALONE_DIR="$FRONTEND_DIR/dist-standalone"
FRONTEND_ENTRY=""
FRONTEND_RUNTIME_DIR="$FRONTEND_STANDALONE_DIR"

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "缺少命令: $cmd"
    exit 1
  fi
}

pm2_name_count() {
  local name="$1"
  pm2 pid "$name" 2>/dev/null | awk '
    /^[[:space:]]*[0-9]+[[:space:]]*$/ && $1 != "0" { count++ }
    END { print count + 0 }
  '
}

pm2_delete_if_exists() {
  local name="$1"
  local count
  count="$(pm2_name_count "$name" 2>/dev/null || echo 0)"
  count="$(echo "$count" | tr -dc '0-9')"
  count="${count:-0}"
  if [[ "$count" -gt 0 ]]; then
    echo "      检测到 PM2 中已有 $name (${count} 个)，先清理..."
    pm2 delete "$name" >/dev/null 2>&1 || true
  fi
}

ensure_config_external_url() {
  local config_path="$1"
  local external_url="$2"
  local temp_path=""

  if [[ ! -f "$config_path" ]]; then
    echo "未找到配置文件 $config_path，跳过 external_url 同步."
    return 0
  fi

  temp_path="$(mktemp)"
  awk -v external_url="$external_url" '
    function indent_len(line, matched) {
      match(line, /^[[:space:]]*/)
      return RLENGTH
    }
    function leading_indent(line) {
      match(line, /^[[:space:]]*/)
      return substr(line, 1, RLENGTH)
    }
    function ltrim(line) {
      sub(/^[[:space:]]+/, "", line)
      return line
    }
    BEGIN {
      app_found = 0
      external_updated = 0
      app_indent = ""
      app_indent_len = -1
    }
    {
      line = $0
      trimmed = ltrim(line)

      if (!app_found && line ~ /^[[:space:]]*app:[[:space:]]*$/) {
        app_found = 1
        app_indent = leading_indent(line)
        app_indent_len = indent_len(line)
        print line
        next
      }

      if (app_found && !external_updated) {
        current_indent_len = indent_len(line)

        if (trimmed ~ /^external_url:[[:space:]]*/) {
          print app_indent "  external_url: \"" external_url "\""
          external_updated = 1
          next
        }

        if (trimmed != "" && trimmed !~ /^#/ && current_indent_len <= app_indent_len) {
          print app_indent "  external_url: \"" external_url "\""
          external_updated = 1
        }
      }

      print line
    }
    END {
      if (app_found && !external_updated) {
        print app_indent "  external_url: \"" external_url "\""
      }
    }
  ' "$config_path" > "$temp_path"
  mv "$temp_path" "$config_path"

  echo "      已同步 app.external_url = $external_url"
}

prepare_frontend_standalone() {
  local next_standalone_dir="$FRONTEND_DIR/.next/standalone"
  local next_standalone_entry=""
  local entry_relative_path=""
  local runtime_relative_dir=""

  if [[ ! -f "$FRONTEND_DIR/package.json" ]]; then
    echo "未找到 frontend/package.json，跳过前端"
    return 1
  fi

  cd "$FRONTEND_DIR"

  if ! $NO_BUILD; then
    echo "      构建前端（next build）..."
    pnpm run build
  fi

  if [[ -f "$next_standalone_dir/server.js" ]]; then
    next_standalone_entry="$next_standalone_dir/server.js"
  else
    next_standalone_entry="$(find "$next_standalone_dir" -maxdepth 3 -type f -name 'server.js' | sort | head -n 1)"
  fi

  if [[ -z "$next_standalone_entry" || ! -f "$next_standalone_entry" ]]; then
    echo "未找到 .next/standalone 下的 server.js，请确认 next.config.ts 已配置 output: 'standalone'"
    return 1
  fi

  entry_relative_path="${next_standalone_entry#"$next_standalone_dir"/}"
  runtime_relative_dir="$(dirname "$entry_relative_path")"

  echo "      组装 standalone 运行目录 (entry: $entry_relative_path)..."
  rm -rf "$FRONTEND_STANDALONE_DIR"
  mkdir -p "$FRONTEND_STANDALONE_DIR"
  cp -a "$FRONTEND_DIR/.next/standalone/." "$FRONTEND_STANDALONE_DIR/"
  FRONTEND_ENTRY="$FRONTEND_STANDALONE_DIR/$entry_relative_path"
  if [[ "$runtime_relative_dir" == "." ]]; then
    FRONTEND_RUNTIME_DIR="$FRONTEND_STANDALONE_DIR"
  else
    FRONTEND_RUNTIME_DIR="$FRONTEND_STANDALONE_DIR/$runtime_relative_dir"
  fi
  if [[ -d "$FRONTEND_DIR/.next/static" ]]; then
    mkdir -p "$FRONTEND_RUNTIME_DIR/.next"
    cp -a "$FRONTEND_DIR/.next/static" "$FRONTEND_RUNTIME_DIR/.next/static"
  fi
  if [[ -d "$FRONTEND_DIR/public" ]]; then
    cp -a "$FRONTEND_DIR/public" "$FRONTEND_RUNTIME_DIR/public"
  fi
  cd "$PROJECT_ROOT"

  if [[ ! -f "$FRONTEND_ENTRY" ]]; then
    echo "standalone 产物不完整: $FRONTEND_ENTRY 不存在"
    return 1
  fi
  return 0
}

start_frontend_dev() {
  if [[ ! -f "$FRONTEND_DIR/package.json" ]]; then
    echo "未找到 frontend/package.json，跳过前端"
    return 1
  fi

  pm2_delete_if_exists "$PM2_UI"
  HOSTNAME="0.0.0.0" PORT="$FRONTEND_PORT" NEXT_PUBLIC_BACKEND_BASE_URL="$NEXT_PUBLIC_BACKEND_BASE_URL" \
    pm2 start pnpm --name "$PM2_UI" --cwd "$FRONTEND_DIR" --update-env -- exec next dev --turbopack --hostname 0.0.0.0 --port "$FRONTEND_PORT"
  echo "      前端开发模式已启动 (http://127.0.0.1:$FRONTEND_PORT, command: pnpm run dev)."
  return 0
}

require_cmd go
require_cmd pm2
require_cmd pnpm

if [[ ! -f go.mod ]]; then
  echo "未在项目根找到 go.mod，请于项目根目录执行: ./scripts/restart.sh"
  exit 1
fi

ensure_config_external_url "$CONFIG_PATH" "$APP_EXTERNAL_URL"

if $STOP_FRONTEND; then
  echo "停止前端 (PM2: $PM2_UI)..."
  pm2_delete_if_exists "$PM2_UI"
  pm2 save >/dev/null 2>&1 || true
  pm2 status
  echo "完成."
  exit 0
fi

total=1
if ! $NO_BUILD; then total=$((total + 2)); fi
if ! $NO_FRONTEND; then total=$((total + 1)); fi
step=0

if ! $NO_BUILD; then
  step=$((step + 1)); echo "[$step/$total] 构建 seatunnelx ..."
  go build -o seatunnelx .
  echo "      seatunnelx 构建完成."

  step=$((step + 1)); echo "[$step/$total] 构建 seatunnelx-agent ..."
  (cd agent && go build -o seatunnelx-agent ./cmd)
  echo "      seatunnelx-agent 构建完成."

  if [[ -d lib/agent ]] && [[ -f agent/seatunnelx-agent ]]; then
    cp -f agent/seatunnelx-agent lib/agent/seatunnelx-agent-linux-amd64
    echo "      已同步 agent 到 lib/agent."
  fi
fi

step=$((step + 1)); echo "[$step/$total] 启动后端 (PM2: $PM2_API) ..."
if [[ ! -f "$PROJECT_ROOT/seatunnelx" ]]; then
  echo "未找到 $PROJECT_ROOT/seatunnelx，请先执行一次不带 --no-build 的重启"
  exit 1
fi
pm2_delete_if_exists "$PM2_API"
# 兜底：清理非 PM2 拉起的旧后端进程
pkill -f "$PROJECT_ROOT/seatunnelx api" >/dev/null 2>&1 || true
CONFIG_PATH="$CONFIG_PATH" pm2 start "$PROJECT_ROOT/seatunnelx" --name "$PM2_API" --cwd "$PROJECT_ROOT" --interpreter none -- api
echo "      后端已启动 (API: http://127.0.0.1:8000)."

if $NO_FRONTEND; then
  echo "      跳过前端 (--no-frontend)."
else
  step=$((step + 1))
  if $FRONTEND_DEV; then
    echo "[$step/$total] 启动前端开发模式 (PM2: $PM2_UI) ..."
    if start_frontend_dev; then
      :
    else
      echo "      前端启动已跳过."
    fi
  else
    echo "[$step/$total] 启动前端 standalone (PM2: $PM2_UI) ..."
    if prepare_frontend_standalone; then
      pm2_delete_if_exists "$PM2_UI"
      HOSTNAME="0.0.0.0" PORT="$FRONTEND_PORT" NEXT_PUBLIC_BACKEND_BASE_URL="$NEXT_PUBLIC_BACKEND_BASE_URL" \
        pm2 start "$FRONTEND_ENTRY" --name "$PM2_UI" --cwd "$FRONTEND_RUNTIME_DIR" --update-env
      echo "      前端已启动 (http://127.0.0.1:$FRONTEND_PORT)."
    else
      echo "      前端启动已跳过."
    fi
  fi
fi

echo "[*] 保存 PM2 进程列表 (pm2 save) ..."
pm2 save
pm2 status
echo "完成."
