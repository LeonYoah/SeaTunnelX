#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
PROM_DIR="$(ls -d "$BASE_DIR"/prometheus-* 2>/dev/null | head -1)"
ALERT_DIR="$(ls -d "$BASE_DIR"/alertmanager-* 2>/dev/null | head -1)"
GRAFANA_DIR="$(ls -d "$BASE_DIR"/grafana-* 2>/dev/null | head -1)"
if [[ -z "$PROM_DIR" || -z "$ALERT_DIR" || -z "$GRAFANA_DIR" ]]; then
  echo "Observability components not installed."
  exit 1
fi

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
    echo "stopped $name (pid $pid)"
  else
    echo "$name already stopped"
  fi
  rm -f "$pidfile"
}

stop_one "grafana" "$GRAFANA_DIR/grafana.pid"
stop_one "prometheus" "$PROM_DIR/prometheus.pid"
stop_one "alertmanager" "$ALERT_DIR/alertmanager.pid"
