#!/usr/bin/env bash
set -euo pipefail

#
# 根据 deps/*_config 模板生成 Prometheus / Alertmanager / Grafana 配置。
# 配置与数据写入带版本号的解压目录（prometheus-X.Y.Z、alertmanager-X.Y.Z、grafana-X.Y.Z）。
#

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
PROM_DIR="$(ls -d "$BASE_DIR"/prometheus-* 2>/dev/null | head -1)"
ALERT_DIR="$(ls -d "$BASE_DIR"/alertmanager-* 2>/dev/null | head -1)"
GRAFANA_DIR="$(ls -d "$BASE_DIR"/grafana-* 2>/dev/null | head -1)"
if [[ -z "$PROM_DIR" || -z "$ALERT_DIR" || -z "$GRAFANA_DIR" ]]; then
  echo "Observability components not installed. Run ./install-observability.sh first."
  exit 1
fi

# Prometheus URL 仅用于 Grafana datasource
PROMETHEUS_URL="${PROMETHEUS_URL:-http://127.0.0.1:9090}"

# Grafana 相关
GRAFANA_URL="${GRAFANA_URL:-http://127.0.0.1:3000}"
GRAFANA_URL="${GRAFANA_URL%/}"
GRAFANA_DOMAIN="${GRAFANA_DOMAIN:-}"
GRAFANA_PROXY_SUBPATH="${GRAFANA_PROXY_SUBPATH:-/api/v1/monitoring/proxy/grafana}"
GRAFANA_ROOT_URL="${GRAFANA_ROOT_URL:-${GRAFANA_PROXY_SUBPATH%/}/}"
GRAFANA_ADMIN_USER="${GRAFANA_ADMIN_USER:-admin}"
GRAFANA_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-admin}"

if [[ -z "$GRAFANA_DOMAIN" ]]; then
  GRAFANA_DOMAIN="${GRAFANA_URL#*://}"
  GRAFANA_DOMAIN="${GRAFANA_DOMAIN%%/*}"
  GRAFANA_DOMAIN="${GRAFANA_DOMAIN%%:*}"
  [[ -z "$GRAFANA_DOMAIN" ]] && GRAFANA_DOMAIN="127.0.0.1"
fi

# 仅创建解压目录中不存在的子目录
mkdir -p \
  "$PROM_DIR/rules" \
  "$PROM_DIR/data" \
  "$PROM_DIR/logs" \
  "$ALERT_DIR/data" \
  "$ALERT_DIR/logs" \
  "$GRAFANA_DIR/data" \
  "$GRAFANA_DIR/logs" \
  "$GRAFANA_DIR/plugins" \
  "$GRAFANA_DIR/conf" \
  "$GRAFANA_DIR/provisioning/datasources" \
  "$GRAFANA_DIR/provisioning/dashboards" \
  "$GRAFANA_DIR/provisioning/plugins" \
  "$GRAFANA_DIR/provisioning/alerting" \
  "$GRAFANA_DIR/dashboards"

# ---------- Alertmanager ----------
cp "$BASE_DIR/alertmanager_config/alertmanager.yml" \
  "$ALERT_DIR/alertmanager.yml"

# ---------- Prometheus ----------
cp "$BASE_DIR/prometheus_config/prometheus.yml" \
  "$PROM_DIR/prometheus.yml"

if compgen -G "$BASE_DIR/prometheus_config/rules/*.yml" > /dev/null; then
  cp "$BASE_DIR/prometheus_config/rules/"*.yml \
    "$PROM_DIR/rules/"
fi

# ---------- Grafana ----------
GRAFANA_DATA_PATH="$GRAFANA_DIR/data"
GRAFANA_LOGS_PATH="$GRAFANA_DIR/logs"
GRAFANA_PLUGINS_PATH="$GRAFANA_DIR/plugins"
GRAFANA_PROVISIONING_PATH="$GRAFANA_DIR/provisioning"

sed \
  -e "s#__GRAFANA_DATA__#$GRAFANA_DATA_PATH#g" \
  -e "s#__GRAFANA_LOGS__#$GRAFANA_LOGS_PATH#g" \
  -e "s#__GRAFANA_PLUGINS__#$GRAFANA_PLUGINS_PATH#g" \
  -e "s#__GRAFANA_PROVISIONING__#$GRAFANA_PROVISIONING_PATH#g" \
  -e "s#__GRAFANA_DOMAIN__#$GRAFANA_DOMAIN#g" \
  -e "s#__GRAFANA_ROOT_URL__#$GRAFANA_ROOT_URL#g" \
  -e "s#__GRAFANA_ADMIN_USER__#$GRAFANA_ADMIN_USER#g" \
  -e "s#__GRAFANA_ADMIN_PASSWORD__#$GRAFANA_ADMIN_PASSWORD#g" \
  "$BASE_DIR/grafana_config/grafana.ini.tpl" \
  > "$GRAFANA_DIR/conf/grafana.ini"

sed \
  -e "s#__PROMETHEUS_URL__#$PROMETHEUS_URL#g" \
  "$BASE_DIR/grafana_config/provisioning/datasources/prometheus.yml.tpl" \
  > "$GRAFANA_DIR/provisioning/datasources/prometheus.yml"

GRAFANA_DASHBOARDS_PATH="$GRAFANA_DIR/dashboards"
sed \
  -e "s#__GRAFANA_DASHBOARDS_PATH__#$GRAFANA_DASHBOARDS_PATH#g" \
  "$BASE_DIR/grafana_config/provisioning/dashboards/default.yml" \
  > "$GRAFANA_DIR/provisioning/dashboards/default.yml"

cp "$BASE_DIR/grafana_config/dashboards/"*.json \
  "$GRAFANA_DIR/dashboards/"

echo "Default observability config generated:"
echo "  - prometheus  : $PROMETHEUS_URL"
echo "  - grafana     : $GRAFANA_URL"
echo "  - Seatunnel targets: via HTTP SD (add http_sd_configs in Prometheus)"
echo "  - config dirs : $PROM_DIR, $ALERT_DIR, $GRAFANA_DIR"
