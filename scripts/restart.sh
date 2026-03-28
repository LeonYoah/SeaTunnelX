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

# SeaTunnelX 构建/重启脚本：
# - 后端使用 PM2 启动（seatunnelx-api）
# - 前端默认使用 Next.js standalone 产物 + PM2 启动（seatunnelx-ui）
# - 支持统一的目标（前端/后端）与动作（构建/重启）
# - 启动前会检测并清理同名 PM2 进程，最后执行 pm2 save

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

print_help() {
  cat <<'EOF'
SeaTunnelX 构建/重启脚本

用法:
  ./scripts/restart.sh [选项]

选项:
  --build-only     仅构建（不重启）
  --restart-only   仅重启（不构建）
  --frontend-only  仅处理前端
  --backend-only   仅处理后端
  --no-build       兼容旧参数，等价于 --restart-only
  --no-frontend    兼容旧参数，跳过前端
  --no-backend     跳过后端
  --frontend-dev   前端用 pnpm run dev 启动（仅重启前端时生效）
  --restart-java-proxy
                   构建/重启后，重启本机 seatunnelx-java-proxy
  --stop-frontend  仅停止前端 PM2 进程并退出
  -h, --help       显示本帮助

示例:
  ./scripts/restart.sh                               # 默认：构建并重启前后端
  ./scripts/restart.sh --restart-only --frontend-only # 仅重启前端
  ./scripts/restart.sh --build-only --backend-only    # 仅构建后端
  ./scripts/restart.sh --backend-only --restart-java-proxy
  ./scripts/restart.sh --build-only                   # 仅构建前后端，不重启

环境变量:
  PM2_API                        后端 PM2 进程名，默认 seatunnelx-api
  PM2_UI                         前端 PM2 进程名，默认 seatunnelx-ui
  CONFIG_PATH                    后端配置文件路径，默认 ./config.yaml
  APP_EXTERNAL_URL               写入 config.yaml 的 app.external_url，默认 http://127.0.0.1:8000
  FRONTEND_PORT                  前端端口，默认 80
  NEXT_PUBLIC_BACKEND_BASE_URL   前端访问后端的基础地址，默认 http://127.0.0.1:8000
  LOCAL_SEATUNNEL_HOME           本机 SeaTunnel 安装目录，默认 /opt/seatunnel-2.3.13-new
  LOCAL_JAVA_PROXY_PORT          本机 seatunnelx-java-proxy 端口，默认 18080
  CONTROL_PLANE_BASE_URL         控制面地址，默认 http://127.0.0.1:8000
  CONTROL_PLANE_USERNAME         登录用户名，默认 admin
  CONTROL_PLANE_PASSWORD         登录密码，默认 admin123
EOF
}

BUILD_ONLY=false
RESTART_ONLY=false
NO_BUILD=false
NO_FRONTEND=false
NO_BACKEND=false
FRONTEND_ONLY=false
BACKEND_ONLY=false
STOP_FRONTEND=false
FRONTEND_DEV=false
RESTART_JAVA_PROXY=false
while [[ $# -gt 0 ]]; do
  arg="$1"
  case "$arg" in
    -h|--help)
      print_help
      exit 0
      ;;
    --build-only) BUILD_ONLY=true ;;
    --restart-only) RESTART_ONLY=true ;;
    --frontend-only) FRONTEND_ONLY=true ;;
    --backend-only) BACKEND_ONLY=true ;;
    --no-build) NO_BUILD=true ;;
    --frontend-dev) FRONTEND_DEV=true ;;
    --no-frontend) NO_FRONTEND=true ;;
    --no-backend) NO_BACKEND=true ;;
    --stop-frontend) STOP_FRONTEND=true ;;
    --restart-java-proxy) RESTART_JAVA_PROXY=true ;;
    *)
      echo "未知参数: $arg"
      echo
      print_help
      exit 1
      ;;
  esac
  shift
done

if $BUILD_ONLY && $RESTART_ONLY; then
  echo "参数冲突: --build-only 与 --restart-only 不能同时使用"
  exit 1
fi

if $FRONTEND_ONLY && $BACKEND_ONLY; then
  echo "参数冲突: --frontend-only 与 --backend-only 不能同时使用"
  exit 1
fi

DO_BUILD=true
DO_RESTART=true
if $BUILD_ONLY; then
  DO_RESTART=false
fi
if $RESTART_ONLY || $NO_BUILD; then
  DO_BUILD=false
fi

RUN_BACKEND=true
RUN_FRONTEND=true
if $FRONTEND_ONLY; then
  RUN_BACKEND=false
fi
if $BACKEND_ONLY; then
  RUN_FRONTEND=false
fi
if $NO_FRONTEND; then
  RUN_FRONTEND=false
fi
if $NO_BACKEND; then
  RUN_BACKEND=false
fi

if ! $RUN_BACKEND && ! $RUN_FRONTEND; then
  echo "没有可执行目标：前后端都被禁用了"
  exit 1
fi

PM2_API="${PM2_API:-seatunnelx-api}"
PM2_UI="${PM2_UI:-seatunnelx-ui}"
CONFIG_PATH="${CONFIG_PATH:-$PROJECT_ROOT/config.yaml}"
APP_EXTERNAL_URL="${APP_EXTERNAL_URL:-http://127.0.0.1:8000}"
FRONTEND_PORT="${FRONTEND_PORT:-80}"
NEXT_PUBLIC_BACKEND_BASE_URL="${NEXT_PUBLIC_BACKEND_BASE_URL:-http://127.0.0.1:8000}"
CAPABILITY_PROXY_DEFAULT_VERSION="${CAPABILITY_PROXY_DEFAULT_VERSION:-2.3.13}"
AGENT_HOME="${AGENT_HOME:-/usr/local/lib/seatunnelx-agent}"
AGENT_PROXY_LIB_DIR="${AGENT_PROXY_LIB_DIR:-$AGENT_HOME/lib}"
LOCAL_SEATUNNEL_HOME="${LOCAL_SEATUNNEL_HOME:-/opt/seatunnel-2.3.13-new}"
LOCAL_JAVA_PROXY_PORT="${LOCAL_JAVA_PROXY_PORT:-18080}"
CONTROL_PLANE_BASE_URL="${CONTROL_PLANE_BASE_URL:-http://127.0.0.1:8000}"
CONTROL_PLANE_USERNAME="${CONTROL_PLANE_USERNAME:-admin}"
CONTROL_PLANE_PASSWORD="${CONTROL_PLANE_PASSWORD:-admin123}"
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

restart_local_java_proxy() {
  local install_dir="$LOCAL_SEATUNNEL_HOME"
  local port="$LOCAL_JAVA_PROXY_PORT"
  local script_path=""
  local jar_path="$AGENT_PROXY_LIB_DIR/seatunnelx-java-proxy-${CAPABILITY_PROXY_DEFAULT_VERSION}.jar"
  local state_dir="$install_dir/.seatunnelx/seatunnelx-java-proxy"
  local log_path="$state_dir/service.log"
  local pid_path="$state_dir/service.pid"
  local port_path="$state_dir/service.port"
  local old_pid=""

  for candidate in \
    "$AGENT_HOME/scripts/seatunnelx-java-proxy.sh" \
    "$PROJECT_ROOT/scripts/seatunnelx-java-proxy.sh" \
    "$PROJECT_ROOT/tools/seatunnelx-java-proxy/bin/seatunnelx-java-proxy.sh"
  do
    if [[ -f "$candidate" ]]; then
      script_path="$candidate"
      break
    fi
  done

  if [[ -z "$script_path" ]]; then
    echo "      未找到本机 seatunnelx-java-proxy 启动脚本，跳过重启."
    return 1
  fi
  if [[ ! -f "$jar_path" ]]; then
    echo "      未找到本机 seatunnelx-java-proxy jar: $jar_path"
    return 1
  fi
  if [[ ! -f "$install_dir/starter/seatunnel-starter.jar" ]]; then
    echo "      未找到本机 SeaTunnel 安装目录: $install_dir"
    return 1
  fi

  mkdir -p "$state_dir"
  touch "$log_path"

  old_pid="$(ss -lntp 2>/dev/null | awk -v port=":$port" '$4 ~ port"$" {print $NF}' | sed -n 's/.*pid=\([0-9]\+\).*/\1/p' | head -n1 || true)"
  if [[ -n "$old_pid" ]]; then
    echo "[*] 停止本机 seatunnelx-java-proxy (pid=$old_pid, port=$port) ..."
    kill -TERM "$old_pid" 2>/dev/null || true
    for _ in {1..20}; do
      if ss -lntp 2>/dev/null | rg -q ":$port"; then
        sleep 1
      else
        break
      fi
    done
    if ss -lntp 2>/dev/null | rg -q ":$port"; then
      kill -KILL "$old_pid" 2>/dev/null || true
      sleep 1
    fi
  fi

  echo "[*] 启动本机 seatunnelx-java-proxy ..."
  local pid
  pid="$(
    SEATUNNEL_HOME="$install_dir" \
    SEATUNNEL_PROXY_JAR="$jar_path" \
    SEATUNNELX_JAVA_PROXY_VERSION="$CAPABILITY_PROXY_DEFAULT_VERSION" \
    nohup bash "$script_path" -Dseatunnelx.java.proxy.port="$port" >>"$log_path" 2>&1 < /dev/null & echo $!
  )"
  echo "$pid" >"$pid_path"
  echo "$port" >"$port_path"

  local health_url="http://127.0.0.1:${port}/healthz"
  for _ in {1..30}; do
    if curl -fsS -o /dev/null "$health_url" 2>/dev/null; then
      echo "      本机 seatunnelx-java-proxy 已就绪: http://127.0.0.1:${port}"
      return 0
    fi
    sleep 1
  done

  echo "      本机 seatunnelx-java-proxy 启动后未通过健康检查，请检查 $log_path"
  return 1
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

port_listener_pids() {
  local port="$1"
  ss -ltnp 2>/dev/null | awk -v port=":$port" '
    $4 ~ port"$" {
      while (match($0, /pid=[0-9]+/)) {
        pid = substr($0, RSTART + 4, RLENGTH - 4)
        print pid
        $0 = substr($0, RSTART + RLENGTH)
      }
    }
  ' | sort -u
}

kill_port_listeners_if_exists() {
  local port="$1"
  local pids=""
  pids="$(port_listener_pids "$port" || true)"
  if [[ -z "$pids" ]]; then
    return 0
  fi

  echo "      检测到端口 $port 已被占用，清理旧进程: $(echo "$pids" | xargs)"
  while read -r pid; do
    [[ -z "$pid" ]] && continue
    kill "$pid" >/dev/null 2>&1 || true
  done <<< "$pids"
  sleep 1
  while read -r pid; do
    [[ -z "$pid" ]] && continue
    if kill -0 "$pid" >/dev/null 2>&1; then
      kill -9 "$pid" >/dev/null 2>&1 || true
    fi
  done <<< "$pids"
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
  local should_build="${1:-false}"
  local next_standalone_dir="$FRONTEND_DIR/.next/standalone"
  local next_standalone_entry=""
  local entry_relative_path=""
  local runtime_relative_dir=""

  if [[ ! -f "$FRONTEND_DIR/package.json" ]]; then
    echo "未找到 frontend/package.json，跳过前端"
    return 1
  fi

  cd "$FRONTEND_DIR"

  if [[ "$should_build" == "true" ]]; then
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
  kill_port_listeners_if_exists "$FRONTEND_PORT"
  HOSTNAME="0.0.0.0" PORT="$FRONTEND_PORT" NEXT_PUBLIC_BACKEND_BASE_URL="$NEXT_PUBLIC_BACKEND_BASE_URL" \
    pm2 start pnpm --name "$PM2_UI" --cwd "$FRONTEND_DIR" --update-env -- exec next dev --turbopack --hostname 0.0.0.0 --port "$FRONTEND_PORT"
  echo "      前端开发模式已启动 (http://127.0.0.1:$FRONTEND_PORT, command: pnpm run dev)."
  return 0
}

require_cmd pm2
if $RESTART_JAVA_PROXY; then
  require_cmd curl
fi
if $RUN_BACKEND && $DO_BUILD; then
  require_cmd go
fi
if $RUN_FRONTEND && ($DO_BUILD || $DO_RESTART); then
  require_cmd pnpm
fi

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

if ! $DO_BUILD && ! $DO_RESTART; then
  echo "无操作可执行：构建与重启均已禁用"
  exit 0
fi

if $FRONTEND_DEV && ! $RUN_FRONTEND; then
  echo "警告: --frontend-dev 在当前参数下不会生效（前端已被禁用）"
fi
if $FRONTEND_DEV && ! $DO_RESTART; then
  echo "警告: --frontend-dev 在 --build-only 下不会生效"
fi

total=0
if $DO_BUILD && $RUN_BACKEND; then total=$((total + 3)); fi
if $DO_BUILD && $RUN_FRONTEND && ! ($FRONTEND_DEV && $DO_RESTART); then total=$((total + 1)); fi
if $DO_RESTART && $RUN_BACKEND; then total=$((total + 1)); fi
if $DO_RESTART && $RUN_FRONTEND; then total=$((total + 1)); fi

if [[ "$total" -eq 0 ]]; then
  echo "无操作可执行：请检查参数组合"
  exit 0
fi

step=0
FRONTEND_PREPARED=false

if $DO_BUILD && $RUN_BACKEND; then
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

  step=$((step + 1)); echo "[$step/$total] 构建 seatunnelx-java-proxy 薄 jar ..."
  if command -v mvn >/dev/null 2>&1; then
    mvn -q -f tools/seatunnelx-java-proxy/pom.xml -DskipTests package
    proxy_jar="$(find tools/seatunnelx-java-proxy/target -maxdepth 1 -type f -name 'seatunnelx-java-proxy-*.jar' ! -name '*-bin.jar' | sort | head -n1)"
    if [[ -n "${proxy_jar:-}" && -f "${proxy_jar:-}" ]]; then
      mkdir -p lib
      cp -f "$proxy_jar" "lib/seatunnelx-java-proxy-${CAPABILITY_PROXY_DEFAULT_VERSION}.jar"
      echo "      已同步 seatunnelx-java-proxy jar 到 lib/seatunnelx-java-proxy-${CAPABILITY_PROXY_DEFAULT_VERSION}.jar."
      if [[ -d "$AGENT_PROXY_LIB_DIR" ]]; then
        cp -f "$proxy_jar" "$AGENT_PROXY_LIB_DIR/seatunnelx-java-proxy-${CAPABILITY_PROXY_DEFAULT_VERSION}.jar"
        echo "      已同步 seatunnelx-java-proxy jar 到 $AGENT_PROXY_LIB_DIR/seatunnelx-java-proxy-${CAPABILITY_PROXY_DEFAULT_VERSION}.jar."
      fi
      if [[ -d "$AGENT_HOME/scripts" && -f "$PROJECT_ROOT/scripts/seatunnelx-java-proxy.sh" ]]; then
        cp -f "$PROJECT_ROOT/scripts/seatunnelx-java-proxy.sh" "$AGENT_HOME/scripts/seatunnelx-java-proxy.sh"
        chmod +x "$AGENT_HOME/scripts/seatunnelx-java-proxy.sh"
        echo "      已同步 seatunnelx-java-proxy 启动脚本到 $AGENT_HOME/scripts/seatunnelx-java-proxy.sh."
      fi
    else
      echo "      未找到 seatunnelx-java-proxy 薄 jar，跳过同步."
    fi
  else
    echo "      未找到 mvn，跳过 seatunnelx-java-proxy 薄 jar 构建与同步."
  fi
fi

if $DO_BUILD && $RUN_FRONTEND && ! ($FRONTEND_DEV && $DO_RESTART); then
  step=$((step + 1)); echo "[$step/$total] 构建前端 standalone 产物 ..."
  if prepare_frontend_standalone true; then
    FRONTEND_PREPARED=true
    echo "      前端 standalone 构建完成."
  else
    echo "前端构建失败，已退出."
    exit 1
  fi
fi

if $DO_RESTART && $RUN_BACKEND; then
  step=$((step + 1)); echo "[$step/$total] 启动后端 (PM2: $PM2_API) ..."
  if [[ ! -f "$PROJECT_ROOT/seatunnelx" ]]; then
    echo "未找到 $PROJECT_ROOT/seatunnelx，请先执行一次包含后端构建的命令"
    exit 1
  fi
  pm2_delete_if_exists "$PM2_API"
  # 兜底：清理非 PM2 拉起的旧后端进程
  pkill -f "$PROJECT_ROOT/seatunnelx api" >/dev/null 2>&1 || true
  CONFIG_PATH="$CONFIG_PATH" pm2 start "$PROJECT_ROOT/seatunnelx" --name "$PM2_API" --cwd "$PROJECT_ROOT" --interpreter none -- api
  echo "      后端已启动 (API: http://127.0.0.1:8000)."
fi

if $DO_RESTART && $RUN_FRONTEND; then
  step=$((step + 1))
  if $FRONTEND_DEV; then
    echo "[$step/$total] 启动前端开发模式 (PM2: $PM2_UI) ..."
    if ! start_frontend_dev; then
      echo "前端开发模式启动失败，已退出."
      exit 1
    fi
  else
    echo "[$step/$total] 启动前端 standalone (PM2: $PM2_UI) ..."
    if ! $FRONTEND_PREPARED; then
      if ! prepare_frontend_standalone false; then
        echo "前端 standalone 准备失败，已退出."
        exit 1
      fi
    fi
    pm2_delete_if_exists "$PM2_UI"
    kill_port_listeners_if_exists "$FRONTEND_PORT"
    HOSTNAME="0.0.0.0" PORT="$FRONTEND_PORT" NEXT_PUBLIC_BACKEND_BASE_URL="$NEXT_PUBLIC_BACKEND_BASE_URL" \
      pm2 start "$FRONTEND_ENTRY" --name "$PM2_UI" --cwd "$FRONTEND_RUNTIME_DIR" --update-env
    echo "      前端已启动 (http://127.0.0.1:$FRONTEND_PORT)."
  fi
fi

if $DO_RESTART; then
  echo "[*] 保存 PM2 进程列表 (pm2 save) ..."
  pm2 save
  pm2 status
else
  echo "[*] 构建完成（未执行重启）."
fi

if $RESTART_JAVA_PROXY; then
  restart_local_java_proxy
fi

echo "完成."
