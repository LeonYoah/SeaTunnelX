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
FRONTEND_PORT="${FRONTEND_PORT:-80}"
BACKEND_PORT="${BACKEND_PORT:-8000}"
GRPC_PORT="${GRPC_PORT:-9000}"
PROMETHEUS_PORT="${PROMETHEUS_PORT:-9090}"
ALERTMANAGER_PORT="${ALERTMANAGER_PORT:-9093}"
GRAFANA_PORT="${GRAFANA_PORT:-3000}"
JAVA_PROXY_PORT="${JAVA_PROXY_PORT:-${SEATUNNELX_JAVA_PROXY_PORT:-18080}}"

status_one() {
  local name="$1"
  local pidfile="$2"
  if [[ ! -f "$pidfile" ]]; then
    echo "$name: stopped"
    return
  fi
  local pid
  pid="$(cat "$pidfile" 2>/dev/null || true)"
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    echo "$name: running (pid=$pid)"
  else
    echo "$name: pidfile exists but process not running"
  fi
}

status_one "backend" "$RUN_DIR/backend.pid"
status_one "frontend" "$RUN_DIR/frontend.pid"

echo
echo "ports:"
ss -lntp | grep -E ":${BACKEND_PORT}\\b|:${GRPC_PORT}\\b|:${FRONTEND_PORT}\\b|:${PROMETHEUS_PORT}\\b|:${ALERTMANAGER_PORT}\\b|:${GRAFANA_PORT}\\b|:${JAVA_PROXY_PORT}\\b" || true

if [[ -x "$BASE_DIR/deps/status-observability.sh" ]]; then
  echo
  echo "observability:"
  (cd "$BASE_DIR" && "$BASE_DIR/deps/status-observability.sh") || true
fi
