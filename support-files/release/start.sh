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

set -euo pipefail

BASE_DIR="$(cd "$(dirname "$0")/.." && pwd)"
RUN_DIR="$BASE_DIR/run"
LOG_DIR="$BASE_DIR/logs"

BACKEND_BIN="$BASE_DIR/seatunnelx"
FRONTEND_NODE_BIN="${FRONTEND_NODE_BIN:-$BASE_DIR/runtime/node/bin/node}"
FRONTEND_SERVER="$BASE_DIR/frontend/server.js"
CONFIG_PATH="${CONFIG_PATH:-$BASE_DIR/config.yaml}"

BACKEND_HOST_WAS_SET="${BACKEND_HOST+x}"
BACKEND_PORT_WAS_SET="${BACKEND_PORT+x}"
BACKEND_ADDR_WAS_SET="${BACKEND_ADDR+x}"
GRPC_PORT_WAS_SET="${GRPC_PORT+x}"
PROMETHEUS_PORT_WAS_SET="${PROMETHEUS_PORT+x}"
PROMETHEUS_URL_WAS_SET="${PROMETHEUS_URL+x}"
ALERTMANAGER_PORT_WAS_SET="${ALERTMANAGER_PORT+x}"
ALERTMANAGER_URL_WAS_SET="${ALERTMANAGER_URL+x}"
GRAFANA_PORT_WAS_SET="${GRAFANA_PORT+x}"
GRAFANA_URL_WAS_SET="${GRAFANA_URL+x}"
CONTROL_PLANE_BASE_URL_WAS_SET="${CONTROL_PLANE_BASE_URL+x}"
APP_EXTERNAL_URL_WAS_SET="${APP_EXTERNAL_URL+x}"

FRONTEND_ENABLE="${FRONTEND_ENABLE:-true}"
FRONTEND_PORT="${FRONTEND_PORT:-80}"
FRONTEND_HOST="${FRONTEND_HOST:-0.0.0.0}"
BACKEND_HOST="${BACKEND_HOST:-}"
BACKEND_PORT="${BACKEND_PORT:-8000}"
BACKEND_ADDR="${BACKEND_ADDR:-${BACKEND_HOST}:${BACKEND_PORT}}"
GRPC_PORT="${GRPC_PORT:-9000}"
PROMETHEUS_PORT="${PROMETHEUS_PORT:-9090}"
ALERTMANAGER_PORT="${ALERTMANAGER_PORT:-9093}"
GRAFANA_PORT="${GRAFANA_PORT:-3000}"
JAVA_PROXY_PORT="${JAVA_PROXY_PORT:-${SEATUNNELX_JAVA_PROXY_PORT:-18080}}"
CONTROL_PLANE_BASE_URL="${CONTROL_PLANE_BASE_URL:-http://127.0.0.1:${BACKEND_PORT}}"
APP_EXTERNAL_URL="${APP_EXTERNAL_URL:-$CONTROL_PLANE_BASE_URL}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://127.0.0.1:${PROMETHEUS_PORT}}"
ALERTMANAGER_URL="${ALERTMANAGER_URL:-http://127.0.0.1:${ALERTMANAGER_PORT}}"
GRAFANA_URL="${GRAFANA_URL:-http://127.0.0.1:${GRAFANA_PORT}}"
NEXT_PUBLIC_BACKEND_BASE_URL="${NEXT_PUBLIC_BACKEND_BASE_URL:-$CONTROL_PLANE_BASE_URL}"
START_OBSERVABILITY="${START_OBSERVABILITY:-auto}"

export BACKEND_PORT GRPC_PORT PROMETHEUS_PORT ALERTMANAGER_PORT GRAFANA_PORT
export CONTROL_PLANE_BASE_URL APP_EXTERNAL_URL PROMETHEUS_URL ALERTMANAGER_URL GRAFANA_URL
export SEATUNNELX_JAVA_PROXY_PORT="$JAVA_PROXY_PORT"

mkdir -p "$RUN_DIR" "$LOG_DIR"

validate_port() {
  local name="$1"
  local value="$2"
  if [[ ! "$value" =~ ^[0-9]+$ ]] || (( value < 1 || value > 65535 )); then
    echo "invalid $name: $value"
    exit 1
  fi
}

validate_port "FRONTEND_PORT" "$FRONTEND_PORT"
validate_port "BACKEND_PORT" "$BACKEND_PORT"
validate_port "GRPC_PORT" "$GRPC_PORT"
validate_port "PROMETHEUS_PORT" "$PROMETHEUS_PORT"
validate_port "ALERTMANAGER_PORT" "$ALERTMANAGER_PORT"
validate_port "GRAFANA_PORT" "$GRAFANA_PORT"
validate_port "JAVA_PROXY_PORT" "$JAVA_PROXY_PORT"

yaml_quote() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  printf '"%s"' "$value"
}

set_top_yaml_scalar() {
  local file="$1"
  local section="$2"
  local key="$3"
  local value="$4"
  local tmp
  tmp="$(mktemp)"
  awk -v section="$section" -v key="$key" -v value="$value" '
    /^[^[:space:]#][^:]*:/ {
      top=$0
      sub(":.*", "", top)
      in_section=(top == section)
    }
    in_section && $0 ~ "^[[:space:]]+" key ":[[:space:]]*" {
      indent=$0
      sub("[^ ].*", "", indent)
      print indent key ": " value
      changed=1
      next
    }
    { print }
    END { if (!changed) exit 2 }
  ' "$file" >"$tmp" && mv "$tmp" "$file" || {
    local code=$?
    rm -f "$tmp"
    if [[ "$code" -ne 2 ]]; then
      echo "failed to update $section.$key in $file"
      exit 1
    fi
  }
}

set_nested_yaml_scalar() {
  local file="$1"
  local section="$2"
  local subsection="$3"
  local key="$4"
  local value="$5"
  local tmp
  tmp="$(mktemp)"
  awk -v section="$section" -v subsection="$subsection" -v key="$key" -v value="$value" '
    /^[^[:space:]#][^:]*:/ {
      top=$0
      sub(":.*", "", top)
      in_section=(top == section)
      in_subsection=0
    }
    in_section && $0 ~ "^[[:space:]][[:space:]]" subsection ":[[:space:]]*$" {
      in_subsection=1
    }
    in_section && in_subsection && $0 ~ "^[[:space:]][[:space:]][[:space:]][[:space:]]" key ":[[:space:]]*" {
      indent=$0
      sub("[^ ].*", "", indent)
      print indent key ": " value
      changed=1
      next
    }
    { print }
    END { if (!changed) exit 2 }
  ' "$file" >"$tmp" && mv "$tmp" "$file" || {
    local code=$?
    rm -f "$tmp"
    if [[ "$code" -ne 2 ]]; then
      echo "failed to update $section.$subsection.$key in $file"
      exit 1
    fi
  }
}

apply_config_overrides() {
  if [[ ! -f "$CONFIG_PATH" ]]; then
    return
  fi

  if [[ -n "$BACKEND_PORT_WAS_SET" || -n "$BACKEND_HOST_WAS_SET" || -n "$BACKEND_ADDR_WAS_SET" ]]; then
    set_top_yaml_scalar "$CONFIG_PATH" "app" "addr" "$(yaml_quote "$BACKEND_ADDR")"
  fi
  if [[ -n "$BACKEND_PORT_WAS_SET" || -n "$CONTROL_PLANE_BASE_URL_WAS_SET" || -n "$APP_EXTERNAL_URL_WAS_SET" ]]; then
    set_top_yaml_scalar "$CONFIG_PATH" "app" "external_url" "$(yaml_quote "$APP_EXTERNAL_URL")"
  fi
  if [[ -n "$GRPC_PORT_WAS_SET" ]]; then
    set_top_yaml_scalar "$CONFIG_PATH" "grpc" "port" "$GRPC_PORT"
  fi
  if [[ -n "$PROMETHEUS_PORT_WAS_SET" || -n "$PROMETHEUS_URL_WAS_SET" ]]; then
    set_nested_yaml_scalar "$CONFIG_PATH" "observability" "prometheus" "url" "$(yaml_quote "$PROMETHEUS_URL")"
  fi
  if [[ -n "$ALERTMANAGER_PORT_WAS_SET" || -n "$ALERTMANAGER_URL_WAS_SET" ]]; then
    set_nested_yaml_scalar "$CONFIG_PATH" "observability" "alertmanager" "url" "$(yaml_quote "$ALERTMANAGER_URL")"
  fi
  if [[ -n "$GRAFANA_PORT_WAS_SET" || -n "$GRAFANA_URL_WAS_SET" ]]; then
    set_nested_yaml_scalar "$CONFIG_PATH" "observability" "grafana" "url" "$(yaml_quote "$GRAFANA_URL")"
  fi
}

start_backend() {
  local pidfile="$RUN_DIR/backend.pid"
  if [[ -f "$pidfile" ]]; then
    local pid
    pid="$(cat "$pidfile" 2>/dev/null || true)"
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      echo "backend already running (pid=$pid)"
      return
    fi
    rm -f "$pidfile"
  fi

  if [[ ! -x "$BACKEND_BIN" ]]; then
    echo "backend binary not found: $BACKEND_BIN"
    exit 1
  fi

  CONFIG_PATH="$CONFIG_PATH" nohup "$BACKEND_BIN" api >>"$LOG_DIR/backend.log" 2>&1 &
  echo $! >"$pidfile"
  sleep 1
  if kill -0 "$(cat "$pidfile")" 2>/dev/null; then
    echo "backend started (pid=$(cat "$pidfile"))"
  else
    rm -f "$pidfile"
    echo "backend failed to start, check log: $LOG_DIR/backend.log"
    exit 1
  fi
}

start_frontend() {
  if [[ "$FRONTEND_ENABLE" != "true" && "$FRONTEND_ENABLE" != "1" ]]; then
    echo "frontend disabled by FRONTEND_ENABLE=$FRONTEND_ENABLE"
    return
  fi

  local pidfile="$RUN_DIR/frontend.pid"
  if [[ -f "$pidfile" ]]; then
    local pid
    pid="$(cat "$pidfile" 2>/dev/null || true)"
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      echo "frontend already running (pid=$pid)"
      return
    fi
    rm -f "$pidfile"
  fi

  if [[ ! -x "$FRONTEND_NODE_BIN" ]]; then
    echo "node runtime not found: $FRONTEND_NODE_BIN"
    exit 1
  fi
  if [[ ! -f "$FRONTEND_SERVER" ]]; then
    echo "frontend standalone server not found: $FRONTEND_SERVER"
    exit 1
  fi

  HOSTNAME="$FRONTEND_HOST" \
  PORT="$FRONTEND_PORT" \
  NEXT_PUBLIC_BACKEND_BASE_URL="$NEXT_PUBLIC_BACKEND_BASE_URL" \
  nohup "$FRONTEND_NODE_BIN" "$FRONTEND_SERVER" >>"$LOG_DIR/frontend.log" 2>&1 &
  echo $! >"$pidfile"
  sleep 1
  if kill -0 "$(cat "$pidfile")" 2>/dev/null; then
    echo "frontend started (pid=$(cat "$pidfile"), host=$FRONTEND_HOST, port=$FRONTEND_PORT)"
  else
    rm -f "$pidfile"
    echo "frontend failed to start, check log: $LOG_DIR/frontend.log"
    exit 1
  fi
}

start_observability() {
  if [[ "$START_OBSERVABILITY" == "false" || "$START_OBSERVABILITY" == "0" ]]; then
    echo "observability start skipped by START_OBSERVABILITY=$START_OBSERVABILITY"
    return
  fi

  if [[ -x "$BASE_DIR/deps/start-observability.sh" ]]; then
    echo "starting bundled observability stack..."
    if [[ -x "$BASE_DIR/deps/init-observability-defaults.sh" ]]; then
      "$BASE_DIR/deps/init-observability-defaults.sh"
    fi
    (cd "$BASE_DIR" && "$BASE_DIR/deps/start-observability.sh")
  else
    echo "bundled observability stack not found, skipping"
  fi
}

apply_config_overrides
start_backend
start_frontend
start_observability

echo
echo "done."
echo "  config  : $CONFIG_PATH"
echo "  backend : follow app.addr in config.yaml (env BACKEND_PORT=$BACKEND_PORT)"
echo "  grpc    : follow grpc.port in config.yaml (env GRPC_PORT=$GRPC_PORT)"
if [[ "$FRONTEND_ENABLE" == "true" || "$FRONTEND_ENABLE" == "1" ]]; then
  echo "  frontend: http://$FRONTEND_HOST:$FRONTEND_PORT"
fi
echo "  observability: prometheus=$PROMETHEUS_PORT alertmanager=$ALERTMANAGER_PORT grafana=$GRAFANA_PORT"
echo "  java-proxy   : $JAVA_PROXY_PORT"
echo "  tips    : set FRONTEND_PORT / BACKEND_PORT / GRPC_PORT / GRAFANA_PORT / PROMETHEUS_PORT / ALERTMANAGER_PORT / JAVA_PROXY_PORT to override ports"
