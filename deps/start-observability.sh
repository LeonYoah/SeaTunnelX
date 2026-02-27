#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
RUNTIME_DIR="$BASE_DIR/runtime"
AUTO_INIT="${OBSERVABILITY_AUTO_INIT:-true}"
PROFILE_ENV_FILE="${OBSERVABILITY_PROFILE_ENV_FILE:-$RUNTIME_DIR/observability.env}"

mkdir -p "$RUNTIME_DIR/alertmanager/logs" "$RUNTIME_DIR/prometheus/logs" "$RUNTIME_DIR/grafana/logs"

# Load persisted observability profile before generating configs (if any).
# 在生成配置前加载持久化 profile（如存在），用于重启后保持远程模式。
if [[ -f "$PROFILE_ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$PROFILE_ENV_FILE"
  set +a
  echo "loaded observability profile: $PROFILE_ENV_FILE"
fi

for bin in \
  "$BASE_DIR/alertmanager/alertmanager" \
  "$BASE_DIR/prometheus/prometheus" \
  "$BASE_DIR/grafana/bin/grafana"; do
  if [[ ! -x "$bin" ]]; then
    echo "required binary not found or not executable: $bin"
    exit 1
  fi
done

# Generate/update default configs before each start (can be disabled by OBSERVABILITY_AUTO_INIT=false)
if [[ "$AUTO_INIT" == "true" || "$AUTO_INIT" == "1" ]]; then
  "$BASE_DIR/init-observability-defaults.sh"
fi

# Safe stop old ones if running
for pidfile in \
  "$RUNTIME_DIR/alertmanager/alertmanager.pid" \
  "$RUNTIME_DIR/prometheus/prometheus.pid" \
  "$RUNTIME_DIR/grafana/grafana.pid"; do
  if [[ -f "$pidfile" ]]; then
    pid="$(cat "$pidfile" 2>/dev/null || true)"
    if [[ -n "${pid}" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" || true
      sleep 1
    fi
    rm -f "$pidfile"
  fi
done

setsid sh -c "exec $BASE_DIR/alertmanager/alertmanager --config.file=$RUNTIME_DIR/alertmanager/alertmanager.yml --storage.path=$RUNTIME_DIR/alertmanager/data --web.listen-address=:9093 >> $RUNTIME_DIR/alertmanager/logs/alertmanager.log 2>&1" < /dev/null &
echo $! > "$RUNTIME_DIR/alertmanager/alertmanager.pid"

setsid sh -c "exec $BASE_DIR/prometheus/prometheus --config.file=$RUNTIME_DIR/prometheus/prometheus.yml --storage.tsdb.path=$RUNTIME_DIR/prometheus/data --web.listen-address=:9090 --web.enable-lifecycle >> $RUNTIME_DIR/prometheus/logs/prometheus.log 2>&1" < /dev/null &
echo $! > "$RUNTIME_DIR/prometheus/prometheus.pid"

setsid sh -c "exec $BASE_DIR/grafana/bin/grafana server --homepath=$BASE_DIR/grafana --config=$RUNTIME_DIR/grafana/grafana.ini >> $RUNTIME_DIR/grafana/logs/grafana.log 2>&1" < /dev/null &
echo $! > "$RUNTIME_DIR/grafana/grafana.pid"

sleep 2

echo "Started services:"
for svc in alertmanager prometheus grafana; do
  pidfile="$RUNTIME_DIR/$svc/$svc.pid"
  if [[ "$svc" == "grafana" ]]; then
    pidfile="$RUNTIME_DIR/grafana/grafana.pid"
  fi
  pid="$(cat "$pidfile")"
  echo "  - $svc pid=$pid"
done

echo
echo "Endpoints:"
echo "  - Grafana     : http://127.0.0.1:3000 (admin/admin by default)"
echo "  - GrafanaProxy: /api/v1/monitoring/proxy/grafana/ (recommended for UI embed)"
echo "  - Prometheus  : http://127.0.0.1:9090"
echo "  - Alertmanager: http://127.0.0.1:9093"
