#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
PROM_DIR="$(ls -d "$BASE_DIR"/prometheus-* 2>/dev/null | head -1)"
ALERT_DIR="$(ls -d "$BASE_DIR"/alertmanager-* 2>/dev/null | head -1)"
GRAFANA_DIR="$(ls -d "$BASE_DIR"/grafana-* 2>/dev/null | head -1)"
if [[ -z "$PROM_DIR" || -z "$ALERT_DIR" || -z "$GRAFANA_DIR" ]]; then
  echo "Observability components not installed. Run ./install-observability.sh first."
  exit 1
fi

mkdir -p "$ALERT_DIR/logs" "$PROM_DIR/logs" "$GRAFANA_DIR/logs"

for bin in \
  "$ALERT_DIR/alertmanager" \
  "$PROM_DIR/prometheus" \
  "$GRAFANA_DIR/bin/grafana"; do
  if [[ ! -x "$bin" ]]; then
    echo "required binary not found or not executable: $bin"
    echo "run ./install-observability.sh first"
    exit 1
  fi
done

for cfg in \
  "$PROM_DIR/prometheus.yml" \
  "$ALERT_DIR/alertmanager.yml" \
  "$GRAFANA_DIR/conf/grafana.ini"; do
  if [[ ! -f "$cfg" ]]; then
    echo "config not found: $cfg"
    echo "run ./init-observability-defaults.sh first"
    exit 1
  fi
done

for pidfile in \
  "$ALERT_DIR/alertmanager.pid" \
  "$PROM_DIR/prometheus.pid" \
  "$GRAFANA_DIR/grafana.pid"; do
  if [[ -f "$pidfile" ]]; then
    pid="$(cat "$pidfile" 2>/dev/null || true)"
    if [[ -n "${pid}" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" || true
      sleep 1
    fi
    rm -f "$pidfile"
  fi
done

setsid sh -c "exec $ALERT_DIR/alertmanager --config.file=$ALERT_DIR/alertmanager.yml --storage.path=$ALERT_DIR/data --web.listen-address=:9093 >> $ALERT_DIR/logs/alertmanager.log 2>&1" < /dev/null &
echo $! > "$ALERT_DIR/alertmanager.pid"

setsid sh -c "exec $PROM_DIR/prometheus --config.file=$PROM_DIR/prometheus.yml --storage.tsdb.path=$PROM_DIR/data --web.listen-address=:9090 --web.enable-lifecycle >> $PROM_DIR/logs/prometheus.log 2>&1" < /dev/null &
echo $! > "$PROM_DIR/prometheus.pid"

setsid sh -c "exec $GRAFANA_DIR/bin/grafana server --homepath=$GRAFANA_DIR --config=$GRAFANA_DIR/conf/grafana.ini >> $GRAFANA_DIR/logs/grafana.log 2>&1" < /dev/null &
echo $! > "$GRAFANA_DIR/grafana.pid"

sleep 2

echo "Started services:"
for svc in alertmanager prometheus grafana; do
  case "$svc" in
    alertmanager) pidfile="$ALERT_DIR/alertmanager.pid" ;;
    prometheus)   pidfile="$PROM_DIR/prometheus.pid" ;;
    grafana)      pidfile="$GRAFANA_DIR/grafana.pid" ;;
  esac
  pid="$(cat "$pidfile")"
  echo "  - $svc pid=$pid"
done

echo
echo "Endpoints:"
echo "  - Grafana     : http://127.0.0.1:3000 (admin/admin by default)"
echo "  - GrafanaProxy: /api/v1/monitoring/proxy/grafana/ (recommended for UI embed)"
echo "  - Prometheus  : http://127.0.0.1:9090"
echo "  - Alertmanager: http://127.0.0.1:9093"
