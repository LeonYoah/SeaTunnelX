#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
PROM_DIR="$BASE_DIR/prometheus"
ALERT_DIR="$BASE_DIR/alertmanager"
GRAFANA_DIR="$BASE_DIR/grafana"

for svc in alertmanager prometheus grafana; do
  case "$svc" in
    alertmanager) pidfile="$ALERT_DIR/alertmanager.pid" ;;
    prometheus)   pidfile="$PROM_DIR/prometheus.pid" ;;
    grafana)      pidfile="$GRAFANA_DIR/grafana.pid" ;;
    *)            continue ;;
  esac
  if [[ -f "$pidfile" ]]; then
    pid="$(cat "$pidfile" 2>/dev/null || true)"
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      echo "$svc: running (pid=$pid)"
    else
      echo "$svc: pidfile exists but process not running"
    fi
  else
    echo "$svc: stopped"
  fi
done

echo
echo "Ports:"
ss -lntp 2>/dev/null | grep -E ':9090|:9093|:3000' || true
