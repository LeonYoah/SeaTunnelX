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
STOP_OBSERVABILITY="${STOP_OBSERVABILITY:-true}"

stop_one() {
  local name="$1"
  local pidfile="$2"
  if [[ ! -f "$pidfile" ]]; then
    echo "$name not running (no pidfile)"
    return
  fi
  local pid
  pid="$(cat "$pidfile" 2>/dev/null || true)"
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    kill "$pid" || true
    sleep 1
    if kill -0 "$pid" 2>/dev/null; then
      kill -9 "$pid" || true
    fi
    echo "stopped $name (pid=$pid)"
  else
    echo "$name already stopped"
  fi
  rm -f "$pidfile"
}

stop_one "frontend" "$RUN_DIR/frontend.pid"
stop_one "backend" "$RUN_DIR/backend.pid"

if [[ "$STOP_OBSERVABILITY" == "true" || "$STOP_OBSERVABILITY" == "1" ]]; then
  if [[ -x "$BASE_DIR/deps/stop-observability.sh" ]]; then
    (cd "$BASE_DIR" && "$BASE_DIR/deps/stop-observability.sh") || true
  fi
fi
