#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
RUNTIME_DIR="$BASE_DIR/runtime"

SEATUNNEL_METRICS_TARGETS="${SEATUNNEL_METRICS_TARGETS:-127.0.0.1:8081}"
SEATUNNEL_CLUSTER_LABEL="${SEATUNNEL_CLUSTER_LABEL:-seatunnel-5801}"
SEATUNNEL_SERVICE_LABEL="${SEATUNNEL_SERVICE_LABEL:-seatunnel-engine}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://127.0.0.1:9090}"
PROMETHEUS_USE_HTTP_SD="${PROMETHEUS_USE_HTTP_SD:-false}"
PROMETHEUS_HTTP_SD_URL="${PROMETHEUS_HTTP_SD_URL:-http://127.0.0.1:8000/api/v1/monitoring/prometheus/discovery}"
PROMETHEUS_HTTP_SD_REFRESH_INTERVAL="${PROMETHEUS_HTTP_SD_REFRESH_INTERVAL:-30s}"
GRAFANA_URL="${GRAFANA_URL:-http://127.0.0.1:3000}"
GRAFANA_URL="${GRAFANA_URL%/}"
GRAFANA_DOMAIN="${GRAFANA_DOMAIN:-}"
GRAFANA_PROXY_SUBPATH="${GRAFANA_PROXY_SUBPATH:-/api/v1/monitoring/proxy/grafana}"
GRAFANA_ROOT_URL="${GRAFANA_ROOT_URL:-${GRAFANA_PROXY_SUBPATH%/}/}"
GRAFANA_ADMIN_USER="${GRAFANA_ADMIN_USER:-admin}"
GRAFANA_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-admin}"
ALERTMANAGER_WEBHOOK_URL="${ALERTMANAGER_WEBHOOK_URL:-}"

if [[ -z "$GRAFANA_DOMAIN" ]]; then
  GRAFANA_DOMAIN="${GRAFANA_URL#*://}"
  GRAFANA_DOMAIN="${GRAFANA_DOMAIN%%/*}"
  GRAFANA_DOMAIN="${GRAFANA_DOMAIN%%:*}"
  [[ -z "$GRAFANA_DOMAIN" ]] && GRAFANA_DOMAIN="127.0.0.1"
fi

mkdir -p \
  "$RUNTIME_DIR/prometheus/rules" \
  "$RUNTIME_DIR/prometheus/data" \
  "$RUNTIME_DIR/prometheus/logs" \
  "$RUNTIME_DIR/alertmanager/data" \
  "$RUNTIME_DIR/alertmanager/logs" \
  "$RUNTIME_DIR/grafana/data" \
  "$RUNTIME_DIR/grafana/logs" \
  "$RUNTIME_DIR/grafana/plugins" \
  "$RUNTIME_DIR/grafana/provisioning/datasources" \
  "$RUNTIME_DIR/grafana/provisioning/dashboards" \
  "$RUNTIME_DIR/grafana/provisioning/plugins" \
  "$RUNTIME_DIR/grafana/provisioning/alerting" \
  "$RUNTIME_DIR/grafana/dashboards"

# ---------- Alertmanager ----------
{
  echo "global:"
  echo "  resolve_timeout: 5m"
  echo
  echo "route:"
  echo "  receiver: default"
  echo "  group_by: [alertname, cluster, instance]"
  echo "  group_wait: 30s"
  echo "  group_interval: 5m"
  echo "  repeat_interval: 2h"
  echo
  echo "receivers:"
  echo "  - name: default"
  if [[ -n "$ALERTMANAGER_WEBHOOK_URL" ]]; then
    echo "    webhook_configs:"
    echo "      - url: '$ALERTMANAGER_WEBHOOK_URL'"
    echo "        send_resolved: true"
  fi
} > "$RUNTIME_DIR/alertmanager/alertmanager.yml"

# ---------- Prometheus ----------
{
  echo "global:"
  echo "  scrape_interval: 15s"
  echo "  evaluation_interval: 15s"
  echo
  echo "rule_files:"
  echo "  - $RUNTIME_DIR/prometheus/rules/*.yml"
  echo
  echo "alerting:"
  echo "  alertmanagers:"
  echo "    - static_configs:"
  echo "        - targets: ['127.0.0.1:9093']"
  echo
  echo "scrape_configs:"
  echo "  - job_name: 'prometheus'"
  echo "    static_configs:"
  echo "      - targets: ['127.0.0.1:9090']"
  echo
  echo "  - job_name: 'alertmanager'"
  echo "    static_configs:"
  echo "      - targets: ['127.0.0.1:9093']"
  echo
  echo "  - job_name: 'seatunnel_engine_http'"
  echo "    metrics_path: /metrics"
  if [[ "$PROMETHEUS_USE_HTTP_SD" == "true" || "$PROMETHEUS_USE_HTTP_SD" == "1" ]]; then
    echo "    scheme: http"
    echo "    http_sd_configs:"
    echo "      - url: '$PROMETHEUS_HTTP_SD_URL'"
    echo "        refresh_interval: $PROMETHEUS_HTTP_SD_REFRESH_INTERVAL"
    # Backward compatibility for existing rules/panels still using label 'cluster'.
    # 向后兼容：为仍使用 cluster 标签的规则/面板补齐映射。
    echo "    relabel_configs:"
    echo "      - source_labels: [cluster_name]"
    echo "        target_label: cluster"
  else
    echo "    static_configs:"
    echo "      - targets:"
    IFS=',' read -ra TARGETS <<< "$SEATUNNEL_METRICS_TARGETS"
    for target in "${TARGETS[@]}"; do
      target_trimmed="$(echo "$target" | xargs)"
      [[ -z "$target_trimmed" ]] && continue
      echo "          - '$target_trimmed'"
    done
    echo "        labels:"
    echo "          cluster: '$SEATUNNEL_CLUSTER_LABEL'"
    echo "          service: '$SEATUNNEL_SERVICE_LABEL'"
  fi
} > "$RUNTIME_DIR/prometheus/prometheus.yml"

# cleanup legacy generated file to avoid duplicated alert names
rm -f "$RUNTIME_DIR/prometheus/rules/seatunnel-alerts.yml"

cat > "$RUNTIME_DIR/prometheus/rules/seatunnel-default-rules.yml" <<'RULES'
groups:
  - name: seatunnel-default-recording
    interval: 30s
    rules:
      - record: seatunnel:job_thread_pool_queue_depth:max
        expr: max by (cluster, instance) (job_thread_pool_queueTaskCount{job="seatunnel_engine_http"})

      - record: seatunnel:job_thread_pool_submit_rate5m
        expr: sum by (cluster, instance) (rate(job_thread_pool_task_total{job="seatunnel_engine_http"}[5m]))

      - record: seatunnel:job_thread_pool_complete_rate5m
        expr: sum by (cluster, instance) (rate(job_thread_pool_completedTask_total{job="seatunnel_engine_http"}[5m]))

      - record: seatunnel:job_thread_pool_reject_rate5m
        expr: sum by (cluster, instance) (rate(job_thread_pool_rejection_total{job="seatunnel_engine_http"}[5m]))

      - record: seatunnel:job_thread_pool_backlog_gap_rate5m
        expr: seatunnel:job_thread_pool_submit_rate5m - seatunnel:job_thread_pool_complete_rate5m

      - record: seatunnel:jvm_heap_usage_percent
        expr: |
          100 * jvm_memory_bytes_used{job="seatunnel_engine_http",area="heap"}
          /
          clamp_min(jvm_memory_bytes_max{job="seatunnel_engine_http",area="heap"}, 1)

      - record: seatunnel:jvm_nonheap_usage_percent
        expr: |
          100 * jvm_memory_bytes_used{job="seatunnel_engine_http",area="nonheap"}
          /
          clamp_min(jvm_memory_bytes_max{job="seatunnel_engine_http",area="nonheap"}, 1)

      - record: seatunnel:gc_time_ratio_percent5m
        expr: 100 * rate(jvm_gc_collection_seconds_sum{job="seatunnel_engine_http"}[5m])

      - record: seatunnel:fd_usage_percent
        expr: 100 * process_open_fds{job="seatunnel_engine_http"} / clamp_min(process_max_fds{job="seatunnel_engine_http"}, 1)

      - record: seatunnel:hazelcast_executor_queue_util_percent
        expr: |
          100 * hazelcast_executor_queueSize{job="seatunnel_engine_http"}
          /
          clamp_min(
            hazelcast_executor_queueSize{job="seatunnel_engine_http"}
            + hazelcast_executor_queueRemainingCapacity{job="seatunnel_engine_http"},
            1
          )

  - name: seatunnel-default-alerts
    interval: 15s
    rules:
      - alert: SeaTunnelMetricsEndpointDown
        expr: up{job="seatunnel_engine_http"} == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "SeaTunnel metrics endpoint is down"
          description: "Metrics endpoint {{ $labels.instance }} (cluster {{ $labels.cluster }}) has been unreachable for more than 2 minutes."

      - alert: SeaTunnelNodeStateDown
        expr: node_state{job="seatunnel_engine_http"} == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "SeaTunnel node is down"
          description: "SeaTunnel node {{ $labels.address }} in cluster {{ $labels.cluster }} reports node_state=0."

      - alert: SeaTunnelPartitionUnsafe
        expr: |
          hazelcast_partition_isClusterSafe{job="seatunnel_engine_http"} == 0
          or
          hazelcast_partition_isLocalMemberSafe{job="seatunnel_engine_http"} == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "SeaTunnel partition safety lost"
          description: "Cluster {{ $labels.cluster }} has unsafe partition state on {{ $labels.instance }}."

      - alert: SeaTunnelJobThreadPoolQueueHigh
        expr: job_thread_pool_queueTaskCount{job="seatunnel_engine_http"} > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "SeaTunnel job thread pool queue is high"
          description: "Queue tasks on {{ $labels.instance }} exceeded 100 for 5 minutes."

      - alert: SeaTunnelCoordinatorBacklogGrowing
        expr: |
          seatunnel:job_thread_pool_queue_depth:max > 50
          and
          seatunnel:job_thread_pool_backlog_gap_rate5m > 0.5
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "SeaTunnel coordinator backlog keeps growing"
          description: "Queue depth remains high and submit rate is higher than complete rate on {{ $labels.instance }}."

      - alert: SeaTunnelCoordinatorRejectingTasks
        expr: seatunnel:job_thread_pool_reject_rate5m > 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "SeaTunnel coordinator is rejecting tasks"
          description: "Task rejections detected on {{ $labels.instance }} in cluster {{ $labels.cluster }}."

      - alert: SeaTunnelJVMHeapUsageHigh
        expr: seatunnel:jvm_heap_usage_percent > 85
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "SeaTunnel JVM heap usage high"
          description: "Heap usage on {{ $labels.instance }} is above 85% for over 10 minutes."

      - alert: SeaTunnelProcessCpuHigh
        expr: rate(process_cpu_seconds_total{job="seatunnel_engine_http"}[5m]) > 0.8
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "SeaTunnel process CPU usage high"
          description: "CPU usage on {{ $labels.instance }} is above 80% for over 10 minutes."

      - alert: SeaTunnelGCTimeHigh
        expr: seatunnel:gc_time_ratio_percent5m > 20
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "SeaTunnel JVM GC time high"
          description: "GC time ratio on {{ $labels.instance }} is high (>20%) for over 10 minutes."

      - alert: SeaTunnelFDUsageHigh
        expr: seatunnel:fd_usage_percent > 80
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "SeaTunnel file descriptor usage high"
          description: "FD usage on {{ $labels.instance }} exceeds 80% for over 10 minutes."

      - alert: SeaTunnelThreadDeadlock
        expr: jvm_threads_deadlocked{job="seatunnel_engine_http"} > 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "SeaTunnel JVM thread deadlock detected"
          description: "Deadlocked JVM threads detected on {{ $labels.instance }}."
RULES

# ---------- Grafana ----------
cat > "$RUNTIME_DIR/grafana/grafana.ini" <<EOF2
[paths]
data = $RUNTIME_DIR/grafana/data
logs = $RUNTIME_DIR/grafana/logs
plugins = $RUNTIME_DIR/grafana/plugins
provisioning = $RUNTIME_DIR/grafana/provisioning

[server]
http_addr = 0.0.0.0
http_port = 3000
domain = $GRAFANA_DOMAIN
root_url = $GRAFANA_ROOT_URL
serve_from_sub_path = true
enforce_domain = false

[security]
admin_user = $GRAFANA_ADMIN_USER
admin_password = $GRAFANA_ADMIN_PASSWORD
allow_embedding = true

[users]
allow_sign_up = false

[auth.anonymous]
enabled = true
org_role = Viewer

[live]
# Disable Grafana Live websocket channel by default to reduce iframe embed noise/retry overhead.
# 默认关闭 Grafana Live WebSocket，减少嵌入场景下的重试噪音与开销。
max_connections = 0

[plugins]
preinstall =
EOF2

cat > "$RUNTIME_DIR/grafana/provisioning/datasources/prometheus.yml" <<EOF2
apiVersion: 1

datasources:
  - name: Prometheus
    uid: prometheus
    type: prometheus
    access: proxy
    url: $PROMETHEUS_URL
    isDefault: true
    editable: true
EOF2

cat > "$RUNTIME_DIR/grafana/provisioning/dashboards/default.yml" <<EOF2
apiVersion: 1

providers:
  - name: 'SeatunnelX Monitoring'
    orgId: 1
    folder: 'SeatunnelX'
    type: file
    disableDeletion: false
    editable: true
    options:
      path: $RUNTIME_DIR/grafana/dashboards
EOF2

# cleanup legacy dashboard file
rm -f "$RUNTIME_DIR/grafana/dashboards/seatunnel-overview.json"

cat > "$RUNTIME_DIR/grafana/dashboards/seatunnel-overview-en.json" <<'DASH'
{
  "id": null,
  "uid": "seatunnel-overview-en",
  "title": "SeaTunnelX Deep Monitoring",
  "timezone": "browser",
  "schemaVersion": 39,
  "version": 3,
  "refresh": "15s",
  "editable": true,
  "graphTooltip": 0,
  "time": {
    "from": "now-6h",
    "to": "now"
  },
  "templating": {
    "list": [
      {
        "name": "cluster",
        "type": "query",
        "datasource": {
          "type": "prometheus",
          "uid": "prometheus"
        },
        "query": "label_values(node_state{job=\"seatunnel_engine_http\"},cluster)",
        "definition": "label_values(node_state{job=\"seatunnel_engine_http\"},cluster)",
        "refresh": 1,
        "includeAll": true,
        "multi": true,
        "allValue": ".*",
        "current": {
          "text": "All",
          "value": [
            "$__all"
          ]
        }
      },
      {
        "name": "instance",
        "type": "query",
        "datasource": {
          "type": "prometheus",
          "uid": "prometheus"
        },
        "query": "label_values(node_state{job=\"seatunnel_engine_http\",cluster=~\"$cluster\"},instance)",
        "definition": "label_values(node_state{job=\"seatunnel_engine_http\",cluster=~\"$cluster\"},instance)",
        "refresh": 1,
        "includeAll": true,
        "multi": true,
        "allValue": ".*",
        "current": {
          "text": "All",
          "value": [
            "$__all"
          ]
        }
      }
    ]
  },
  "annotations": {
    "list": []
  },
  "panels": [
    {
      "id": 1,
      "type": "stat",
      "title": "Scrape Availability (Up)",
      "gridPos": {
        "h": 4,
        "w": 4,
        "x": 0,
        "y": 0
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "min(up{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"})",
          "refId": "A"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "none",
          "mappings": [
            {
              "type": "value",
              "options": {
                "0": {
                  "text": "DOWN"
                },
                "1": {
                  "text": "UP"
                }
              }
            }
          ],
          "thresholds": {
            "mode": "absolute",
            "steps": [
              {
                "color": "red",
                "value": null
              },
              {
                "color": "green",
                "value": 1
              }
            ]
          }
        },
        "overrides": []
      },
      "options": {
        "reduceOptions": {
          "calcs": [
            "lastNotNull"
          ],
          "fields": "",
          "values": false
        },
        "orientation": "auto",
        "textMode": "auto",
        "colorMode": "value",
        "graphMode": "none",
        "justifyMode": "auto"
      }
    },
    {
      "id": 2,
      "type": "stat",
      "title": "Seatunnel Node State",
      "gridPos": {
        "h": 4,
        "w": 4,
        "x": 4,
        "y": 0
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "min(node_state{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"})",
          "refId": "A"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "none",
          "mappings": [
            {
              "type": "value",
              "options": {
                "0": {
                  "text": "DOWN"
                },
                "1": {
                  "text": "UP"
                }
              }
            }
          ],
          "thresholds": {
            "mode": "absolute",
            "steps": [
              {
                "color": "red",
                "value": null
              },
              {
                "color": "green",
                "value": 1
              }
            ]
          }
        },
        "overrides": []
      },
      "options": {
        "reduceOptions": {
          "calcs": [
            "lastNotNull"
          ],
          "fields": "",
          "values": false
        },
        "orientation": "auto",
        "textMode": "auto",
        "colorMode": "value",
        "graphMode": "none",
        "justifyMode": "auto"
      }
    },
    {
      "id": 3,
      "type": "stat",
      "title": "Cluster Safe",
      "gridPos": {
        "h": 4,
        "w": 4,
        "x": 8,
        "y": 0
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "min(hazelcast_partition_isClusterSafe{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"})",
          "refId": "A"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "none",
          "mappings": [
            {
              "type": "value",
              "options": {
                "0": {
                  "text": "UNSAFE"
                },
                "1": {
                  "text": "SAFE"
                }
              }
            }
          ],
          "thresholds": {
            "mode": "absolute",
            "steps": [
              {
                "color": "red",
                "value": null
              },
              {
                "color": "green",
                "value": 1
              }
            ]
          }
        },
        "overrides": []
      },
      "options": {
        "reduceOptions": {
          "calcs": [
            "lastNotNull"
          ],
          "fields": "",
          "values": false
        },
        "orientation": "auto",
        "textMode": "auto",
        "colorMode": "value",
        "graphMode": "none",
        "justifyMode": "auto"
      }
    },
    {
      "id": 4,
      "type": "stat",
      "title": "Running Jobs",
      "gridPos": {
        "h": 4,
        "w": 4,
        "x": 12,
        "y": 0
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "sum(job_count{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\",type=\"running\"})",
          "refId": "A"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "none"
        },
        "overrides": []
      },
      "options": {
        "reduceOptions": {
          "calcs": [
            "lastNotNull"
          ],
          "fields": "",
          "values": false
        },
        "orientation": "auto",
        "textMode": "auto",
        "colorMode": "value",
        "graphMode": "none",
        "justifyMode": "auto"
      }
    },
    {
      "id": 5,
      "type": "stat",
      "title": "Failing+Failed Jobs",
      "gridPos": {
        "h": 4,
        "w": 4,
        "x": 16,
        "y": 0
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "sum(job_count{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\",type=~\"failing|failed\"})",
          "refId": "A"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "none",
          "thresholds": {
            "mode": "absolute",
            "steps": [
              {
                "color": "green",
                "value": null
              },
              {
                "color": "orange",
                "value": 1
              },
              {
                "color": "red",
                "value": 3
              }
            ]
          }
        },
        "overrides": []
      },
      "options": {
        "reduceOptions": {
          "calcs": [
            "lastNotNull"
          ],
          "fields": "",
          "values": false
        },
        "orientation": "auto",
        "textMode": "auto",
        "colorMode": "value",
        "graphMode": "none",
        "justifyMode": "auto"
      }
    },
    {
      "id": 6,
      "type": "stat",
      "title": "Coordinator Queue Depth",
      "gridPos": {
        "h": 4,
        "w": 4,
        "x": 20,
        "y": 0
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "max(job_thread_pool_queueTaskCount{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"})",
          "refId": "A"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "none",
          "thresholds": {
            "mode": "absolute",
            "steps": [
              {
                "color": "green",
                "value": null
              },
              {
                "color": "orange",
                "value": 20
              },
              {
                "color": "red",
                "value": 100
              }
            ]
          }
        },
        "overrides": []
      },
      "options": {
        "reduceOptions": {
          "calcs": [
            "lastNotNull"
          ],
          "fields": "",
          "values": false
        },
        "orientation": "auto",
        "textMode": "auto",
        "colorMode": "value",
        "graphMode": "none",
        "justifyMode": "auto"
      }
    },
    {
      "id": 7,
      "type": "timeseries",
      "title": "Job Lifecycle State Distribution",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 0,
        "y": 4
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "sum by(type) (job_count{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"})",
          "legendFormat": "{{type}}",
          "refId": "A"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "short",
          "custom": {
            "drawStyle": "line",
            "lineInterpolation": "smooth",
            "lineWidth": 2,
            "fillOpacity": 10,
            "showPoints": "never",
            "spanNulls": true,
            "axisPlacement": "auto",
            "stacking": {
              "mode": "normal",
              "group": "A"
            }
          }
        },
        "overrides": []
      },
      "options": {
        "legend": {
          "displayMode": "table",
          "placement": "bottom",
          "calcs": [
            "lastNotNull",
            "max"
          ]
        },
        "tooltip": {
          "mode": "multi",
          "sort": "desc"
        }
      }
    },
    {
      "id": 8,
      "type": "timeseries",
      "title": "Coordinator Throughput (5m)",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 4
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "sum(rate(job_thread_pool_task_total{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"}[5m]))",
          "legendFormat": "submitted/s",
          "refId": "A"
        },
        {
          "expr": "sum(rate(job_thread_pool_completedTask_total{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"}[5m]))",
          "legendFormat": "completed/s",
          "refId": "B"
        },
        {
          "expr": "sum(rate(job_thread_pool_rejection_total{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"}[5m]))",
          "legendFormat": "rejected/s",
          "refId": "C"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "ops",
          "custom": {
            "drawStyle": "line",
            "lineInterpolation": "smooth",
            "lineWidth": 2,
            "fillOpacity": 10,
            "showPoints": "never",
            "spanNulls": true,
            "axisPlacement": "auto"
          }
        },
        "overrides": []
      },
      "options": {
        "legend": {
          "displayMode": "table",
          "placement": "bottom",
          "calcs": [
            "lastNotNull",
            "max"
          ]
        },
        "tooltip": {
          "mode": "multi",
          "sort": "desc"
        }
      }
    },
    {
      "id": 9,
      "type": "timeseries",
      "title": "Coordinator Saturation",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 0,
        "y": 12
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "max(job_thread_pool_activeCount{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"})",
          "legendFormat": "active threads",
          "refId": "A"
        },
        {
          "expr": "max(job_thread_pool_poolSize{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"})",
          "legendFormat": "pool size",
          "refId": "B"
        },
        {
          "expr": "max(job_thread_pool_queueTaskCount{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"})",
          "legendFormat": "queue depth",
          "refId": "C"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "short",
          "custom": {
            "drawStyle": "line",
            "lineInterpolation": "smooth",
            "lineWidth": 2,
            "fillOpacity": 10,
            "showPoints": "never",
            "spanNulls": true,
            "axisPlacement": "auto"
          }
        },
        "overrides": []
      },
      "options": {
        "legend": {
          "displayMode": "table",
          "placement": "bottom",
          "calcs": [
            "lastNotNull",
            "max"
          ]
        },
        "tooltip": {
          "mode": "multi",
          "sort": "desc"
        }
      }
    },
    {
      "id": 10,
      "type": "timeseries",
      "title": "Hazelcast Executor Queue by Type",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 12
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "sum by(type) (hazelcast_executor_queueSize{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"})",
          "legendFormat": "{{type}}",
          "refId": "A"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "short",
          "custom": {
            "drawStyle": "line",
            "lineInterpolation": "smooth",
            "lineWidth": 2,
            "fillOpacity": 10,
            "showPoints": "never",
            "spanNulls": true,
            "axisPlacement": "auto",
            "stacking": {
              "mode": "normal",
              "group": "A"
            }
          }
        },
        "overrides": []
      },
      "options": {
        "legend": {
          "displayMode": "table",
          "placement": "bottom",
          "calcs": [
            "lastNotNull",
            "max"
          ]
        },
        "tooltip": {
          "mode": "multi",
          "sort": "desc"
        }
      }
    },
    {
      "id": 11,
      "type": "timeseries",
      "title": "Hazelcast Queue Utilization %",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 0,
        "y": 20
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "100 * hazelcast_executor_queueSize{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"} / clamp_min(hazelcast_executor_queueSize{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"} + hazelcast_executor_queueRemainingCapacity{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"}, 1)",
          "legendFormat": "{{type}}",
          "refId": "A"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "percent",
          "custom": {
            "drawStyle": "line",
            "lineInterpolation": "smooth",
            "lineWidth": 2,
            "fillOpacity": 10,
            "showPoints": "never",
            "spanNulls": true,
            "axisPlacement": "auto"
          }
        },
        "overrides": []
      },
      "options": {
        "legend": {
          "displayMode": "table",
          "placement": "bottom",
          "calcs": [
            "lastNotNull",
            "max"
          ]
        },
        "tooltip": {
          "mode": "multi",
          "sort": "desc"
        }
      }
    },
    {
      "id": 12,
      "type": "timeseries",
      "title": "JVM Memory Pressure %",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 20
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "100 * jvm_memory_bytes_used{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\",area=\"heap\"} / clamp_min(jvm_memory_bytes_max{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\",area=\"heap\"}, 1)",
          "legendFormat": "heap {{instance}}",
          "refId": "A"
        },
        {
          "expr": "100 * jvm_memory_bytes_used{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\",area=\"nonheap\"} / clamp_min(jvm_memory_bytes_max{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\",area=\"nonheap\"}, 1)",
          "legendFormat": "nonheap {{instance}}",
          "refId": "B"
        },
        {
          "expr": "100 * jvm_memory_pool_bytes_used{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\",pool=\"G1 Old Gen\"} / clamp_min(jvm_memory_pool_bytes_max{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\",pool=\"G1 Old Gen\"}, 1)",
          "legendFormat": "old-gen {{instance}}",
          "refId": "C"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "percent",
          "custom": {
            "drawStyle": "line",
            "lineInterpolation": "smooth",
            "lineWidth": 2,
            "fillOpacity": 10,
            "showPoints": "never",
            "spanNulls": true,
            "axisPlacement": "auto"
          }
        },
        "overrides": []
      },
      "options": {
        "legend": {
          "displayMode": "table",
          "placement": "bottom",
          "calcs": [
            "lastNotNull",
            "max"
          ]
        },
        "tooltip": {
          "mode": "multi",
          "sort": "desc"
        }
      }
    },
    {
      "id": 13,
      "type": "timeseries",
      "title": "GC Time Ratio % (5m)",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 0,
        "y": 28
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "100 * rate(jvm_gc_collection_seconds_sum{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"}[5m])",
          "legendFormat": "{{instance}} {{gc}}",
          "refId": "A"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "percent",
          "custom": {
            "drawStyle": "line",
            "lineInterpolation": "smooth",
            "lineWidth": 2,
            "fillOpacity": 10,
            "showPoints": "never",
            "spanNulls": true,
            "axisPlacement": "auto"
          }
        },
        "overrides": []
      },
      "options": {
        "legend": {
          "displayMode": "table",
          "placement": "bottom",
          "calcs": [
            "lastNotNull",
            "max"
          ]
        },
        "tooltip": {
          "mode": "multi",
          "sort": "desc"
        }
      }
    },
    {
      "id": 14,
      "type": "timeseries",
      "title": "Thread State Breakdown",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 28
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "sum by(state) (jvm_threads_state{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"})",
          "legendFormat": "{{state}}",
          "refId": "A"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "short",
          "custom": {
            "drawStyle": "line",
            "lineInterpolation": "smooth",
            "lineWidth": 2,
            "fillOpacity": 10,
            "showPoints": "never",
            "spanNulls": true,
            "axisPlacement": "auto",
            "stacking": {
              "mode": "normal",
              "group": "A"
            }
          }
        },
        "overrides": []
      },
      "options": {
        "legend": {
          "displayMode": "table",
          "placement": "bottom",
          "calcs": [
            "lastNotNull",
            "max"
          ]
        },
        "tooltip": {
          "mode": "multi",
          "sort": "desc"
        }
      }
    },
    {
      "id": 15,
      "type": "timeseries",
      "title": "Process CPU & FD Pressure",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 0,
        "y": 36
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "100 * rate(process_cpu_seconds_total{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"}[5m])",
          "legendFormat": "cpu %",
          "refId": "A"
        },
        {
          "expr": "100 * process_open_fds{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"} / clamp_min(process_max_fds{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"}, 1)",
          "legendFormat": "fd usage %",
          "refId": "B"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "percent",
          "custom": {
            "drawStyle": "line",
            "lineInterpolation": "smooth",
            "lineWidth": 2,
            "fillOpacity": 10,
            "showPoints": "never",
            "spanNulls": true,
            "axisPlacement": "auto"
          }
        },
        "overrides": []
      },
      "options": {
        "legend": {
          "displayMode": "table",
          "placement": "bottom",
          "calcs": [
            "lastNotNull",
            "max"
          ]
        },
        "tooltip": {
          "mode": "multi",
          "sort": "desc"
        }
      }
    },
    {
      "id": 16,
      "type": "timeseries",
      "title": "Process Memory Footprint",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 36
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "process_resident_memory_bytes{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"}",
          "legendFormat": "RSS {{instance}}",
          "refId": "A"
        },
        {
          "expr": "process_virtual_memory_bytes{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"}",
          "legendFormat": "Virtual {{instance}}",
          "refId": "B"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "bytes",
          "custom": {
            "drawStyle": "line",
            "lineInterpolation": "smooth",
            "lineWidth": 2,
            "fillOpacity": 10,
            "showPoints": "never",
            "spanNulls": true,
            "axisPlacement": "auto"
          }
        },
        "overrides": []
      },
      "options": {
        "legend": {
          "displayMode": "table",
          "placement": "bottom",
          "calcs": [
            "lastNotNull",
            "max"
          ]
        },
        "tooltip": {
          "mode": "multi",
          "sort": "desc"
        }
      }
    },
    {
      "id": 17,
      "type": "timeseries",
      "title": "Critical Anomaly Signals",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 0,
        "y": 44
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus"
      },
      "targets": [
        {
          "expr": "sum(rate(job_thread_pool_rejection_total{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"}[5m]))",
          "legendFormat": "rejection/s",
          "refId": "A"
        },
        {
          "expr": "max(jvm_threads_deadlocked{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"})",
          "legendFormat": "deadlocked threads",
          "refId": "B"
        },
        {
          "expr": "1 - min(hazelcast_partition_isLocalMemberSafe{job=\"seatunnel_engine_http\",cluster=~\"$cluster\",instance=~\"$instance\"})",
          "legendFormat": "partition unsafe flag (1=unsafe)",
          "refId": "C"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "short",
          "custom": {
            "drawStyle": "line",
            "lineInterpolation": "smooth",
            "lineWidth": 2,
            "fillOpacity": 10,
            "showPoints": "never",
            "spanNulls": true,
            "axisPlacement": "auto"
          }
        },
        "overrides": []
      },
      "options": {
        "legend": {
          "displayMode": "table",
          "placement": "bottom",
          "calcs": [
            "lastNotNull",
            "max"
          ]
        },
        "tooltip": {
          "mode": "multi",
          "sort": "desc"
        }
      }
    },
    {
      "id": 18,
      "type": "text",
      "title": "Deep Troubleshooting Playbook",
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 44
      },
      "options": {
        "mode": "markdown",
        "content": "### Deep Troubleshooting Path\n1. **Jobs stuck / latency rising**: check `Coordinator Queue Depth` and `Coordinator Throughput` first. If queue keeps rising while completed/s stays low, focus on scheduler bottlenecks.\n2. **Cluster up but jobs not progressing**: inspect `Cluster Safe` and `Hazelcast Executor Queue by Type`; high queue with unsafe partition usually means coordination-layer blockage.\n3. **Frequent failures/flapping**: correlate `Failing+Failed Jobs` with rejection/s in `Critical Anomaly Signals`.\n4. **Slow memory degradation**: track old-gen in `JVM Memory Pressure %`; if old-gen remains high and GC ratio rises, investigate state accumulation / job leakage.\n5. **High CPU but no throughput gain**: compare `Process CPU & FD Pressure` vs `Coordinator Throughput`; high CPU + low completed/s often indicates busy-waiting or downstream blocking.\n6. **Resource exhaustion risk**: monitor fd usage % and RSS growth trend for proactive scaling / connection lifecycle optimization."
      }
    }
  ]
}
DASH

RUNTIME_DIR="$RUNTIME_DIR" python3 - <<'PY'
import json
import os
from pathlib import Path

runtime_dir = Path(os.environ["RUNTIME_DIR"]) / "grafana" / "dashboards"
en_file = runtime_dir / "seatunnel-overview-en.json"
zh_file = runtime_dir / "seatunnel-overview-zh.json"

dashboard_en = json.loads(en_file.read_text())

playbook_en = (
    "### Deep Troubleshooting Path\n"
    "1. **Split-brain / partition risk first**: watch `Cluster Safe` and `Local Member Safe`; any persistent `0` is HA consistency risk.\n"
    "2. **CPU vs FD correlation**: high CPU + high FD% often indicates downstream/socket pressure; high FD alone often suggests connection leak risk.\n"
    "3. **Memory & GC pressure**: if old-gen stays high while `GC Time Ratio` climbs, investigate state accumulation / leakage.\n"
    "4. **Thread deadlock**: `Deadlocked Threads > 0` requires immediate thread-dump and lock analysis.\n"
    "5. **Jobs stuck / backlog rising**: if completed/s stays low while failure/rejection rises, focus on scheduler or downstream bottlenecks.\n"
    "6. **Recovery priority**: restore partition safety first, then reduce rejection and queue backlog."
)


def make_row(panel_id, title, y):
    return {
        "id": panel_id,
        "type": "row",
        "title": title,
        "gridPos": {"h": 1, "w": 24, "x": 0, "y": y},
        "collapsed": False,
        "panels": [],
    }


def make_stat_panel(
    panel_id,
    title,
    expr,
    grid_pos,
    unit="none",
    mappings=None,
    thresholds=None,
):
    defaults = {"unit": unit}
    if mappings:
        defaults["mappings"] = mappings
    if thresholds:
        defaults["thresholds"] = thresholds
    return {
        "id": panel_id,
        "type": "stat",
        "title": title,
        "gridPos": grid_pos,
        "datasource": {"type": "prometheus", "uid": "prometheus"},
        "targets": [{"expr": expr, "refId": "A"}],
        "fieldConfig": {"defaults": defaults, "overrides": []},
        "options": {
            "reduceOptions": {
                "calcs": ["lastNotNull"],
                "fields": "",
                "values": False,
            },
            "orientation": "auto",
            "textMode": "auto",
            "colorMode": "value",
            "graphMode": "none",
            "justifyMode": "auto",
        },
    }


panel_blacklist_ids = {19, 20, 21, 1001, 1002, 1003, 1004, 1005}
base_panels = [
    p
    for p in dashboard_en.get("panels", [])
    if p.get("id") not in panel_blacklist_ids and p.get("type") != "row"
]
panel_by_title = {p.get("title"): p for p in base_panels if p.get("title")}


def require_panel(title):
    panel = panel_by_title.get(title)
    if panel is None:
        raise SystemExit(f"panel not found: {title}")
    return panel


def set_grid(panel, x, y, w, h):
    panel["gridPos"] = {"x": x, "y": y, "w": w, "h": h}


panel_up = require_panel("Scrape Availability (Up)")
panel_node = require_panel("Seatunnel Node State")
panel_cluster_safe = require_panel("Cluster Safe")
panel_running = require_panel("Running Jobs")
panel_failing = require_panel("Failing+Failed Jobs")
panel_lifecycle = require_panel("Job Lifecycle State Distribution")
panel_throughput = require_panel("Coordinator Throughput (5m)")
panel_saturation = require_panel("Coordinator Saturation")
panel_hz_queue_type = require_panel("Hazelcast Executor Queue by Type")
panel_hz_util = require_panel("Hazelcast Queue Utilization %")
panel_jvm = require_panel("JVM Memory Pressure %")
panel_gc = require_panel("GC Time Ratio % (5m)")
panel_threads = require_panel("Thread State Breakdown")
panel_cpu_fd = require_panel("Process CPU & FD Pressure")
panel_process_mem = require_panel("Process Memory Footprint")
panel_anomaly = require_panel("Critical Anomaly Signals")
panel_playbook = require_panel("Deep Troubleshooting Playbook")

panel_playbook.setdefault("options", {})["content"] = playbook_en

panel_local_member_safe = make_stat_panel(
    panel_id=19,
    title="Local Member Safe",
    expr='min(hazelcast_partition_isLocalMemberSafe{job="seatunnel_engine_http",cluster=~"$cluster",instance=~"$instance"})',
    grid_pos={"x": 9, "y": 1, "w": 3, "h": 4},
    mappings=[
        {
            "type": "value",
            "options": {"0": {"text": "UNSAFE"}, "1": {"text": "SAFE"}},
        }
    ],
    thresholds={
        "mode": "absolute",
        "steps": [
            {"color": "red", "value": None},
            {"color": "green", "value": 1},
        ],
    },
)

panel_deadlocked = make_stat_panel(
    panel_id=20,
    title="Deadlocked Threads",
    expr='max(jvm_threads_deadlocked{job="seatunnel_engine_http",cluster=~"$cluster",instance=~"$instance"})',
    grid_pos={"x": 12, "y": 1, "w": 3, "h": 4},
    thresholds={
        "mode": "absolute",
        "steps": [
            {"color": "green", "value": None},
            {"color": "red", "value": 1},
        ],
    },
)

panel_fd_usage = make_stat_panel(
    panel_id=21,
    title="FD Usage %",
    expr='max(seatunnel:fd_usage_percent{cluster=~"$cluster",instance=~"$instance"})',
    grid_pos={"x": 15, "y": 1, "w": 3, "h": 4},
    unit="percent",
    thresholds={
        "mode": "absolute",
        "steps": [
            {"color": "green", "value": None},
            {"color": "orange", "value": 70},
            {"color": "red", "value": 85},
        ],
    },
)

# Re-layout for grouped triage workflow
set_grid(panel_up, 0, 1, 3, 4)
set_grid(panel_node, 3, 1, 3, 4)
set_grid(panel_cluster_safe, 6, 1, 3, 4)
set_grid(panel_running, 18, 1, 3, 4)
set_grid(panel_failing, 21, 1, 3, 4)

set_grid(panel_cpu_fd, 0, 6, 12, 8)
set_grid(panel_jvm, 12, 6, 12, 8)
set_grid(panel_gc, 0, 14, 12, 8)
set_grid(panel_threads, 12, 14, 12, 8)
set_grid(panel_process_mem, 0, 22, 24, 8)

set_grid(panel_throughput, 0, 31, 12, 8)
set_grid(panel_saturation, 12, 31, 12, 8)
set_grid(panel_lifecycle, 0, 39, 24, 8)

set_grid(panel_hz_queue_type, 0, 48, 12, 8)
set_grid(panel_hz_util, 12, 48, 12, 8)

set_grid(panel_anomaly, 0, 57, 12, 10)
set_grid(panel_playbook, 12, 57, 12, 10)

row_ha = make_row(1001, "P0 Availability & HA", 0)
row_resource = make_row(1002, "P1 Resource Pressure (CPU/Memory/GC/Threads/FD)", 5)
row_schedule = make_row(1003, "P2 Scheduling & Throughput", 30)
row_hazelcast = make_row(1004, "P3 Hazelcast Consistency", 47)
row_anomaly = make_row(1005, "P4 Anomaly & Playbook", 56)

dashboard_en["panels"] = [
    row_ha,
    panel_up,
    panel_node,
    panel_cluster_safe,
    panel_local_member_safe,
    panel_deadlocked,
    panel_fd_usage,
    panel_running,
    panel_failing,
    row_resource,
    panel_cpu_fd,
    panel_jvm,
    panel_gc,
    panel_threads,
    panel_process_mem,
    row_schedule,
    panel_throughput,
    panel_saturation,
    panel_lifecycle,
    row_hazelcast,
    panel_hz_queue_type,
    panel_hz_util,
    row_anomaly,
    panel_anomaly,
    panel_playbook,
]

en_file.write_text(json.dumps(dashboard_en, ensure_ascii=False, indent=2) + "\n")

# Build Chinese dashboard from English template and translate titles/playbook.
dashboard_zh = json.loads(json.dumps(dashboard_en))
dashboard_zh["uid"] = "seatunnel-overview-zh"
dashboard_zh["title"] = "SeaTunnelX 深度监控"

title_map = {
    "P0 Availability & HA": "P0 可用性与高可用",
    "P1 Resource Pressure (CPU/Memory/GC/Threads/FD)": "P1 资源压力（CPU/内存/GC/线程/FD）",
    "P2 Scheduling & Throughput": "P2 调度与吞吐",
    "P3 Hazelcast Consistency": "P3 Hazelcast 一致性",
    "P4 Anomaly & Playbook": "P4 异常与排障手册",
    "Scrape Availability (Up)": "抓取可用性 (Up)",
    "Seatunnel Node State": "SeaTunnel 节点状态",
    "Cluster Safe": "集群分区安全",
    "Local Member Safe": "本地成员分区安全",
    "Deadlocked Threads": "死锁线程数",
    "FD Usage %": "FD 使用率 %",
    "Running Jobs": "运行中作业",
    "Failing+Failed Jobs": "失败/失败中作业",
    "Job Lifecycle State Distribution": "作业生命周期状态分布",
    "Coordinator Throughput (5m)": "协调器吞吐 (5分钟)",
    "Coordinator Saturation": "协调器饱和度",
    "Hazelcast Executor Queue by Type": "Hazelcast 执行器队列（按类型）",
    "Hazelcast Queue Utilization %": "Hazelcast 队列利用率 %",
    "JVM Memory Pressure %": "JVM 内存压力 %",
    "GC Time Ratio % (5m)": "GC 时间占比 % (5分钟)",
    "Thread State Breakdown": "线程状态分布",
    "Process CPU & FD Pressure": "进程 CPU 与 FD 压力",
    "Process Memory Footprint": "进程内存占用",
    "Critical Anomaly Signals": "关键异常信号",
    "Deep Troubleshooting Playbook": "深度排障手册",
}

playbook_zh = (
    "### 深度排障路径\n"
    "1. **先看脑裂/分区风险**：`Cluster Safe` 与 `Local Member Safe` 只要持续为 `0`，就是高可用一致性风险。\n"
    "2. **CPU 与 FD 联动看**：CPU 高 + FD% 高常见于下游/网络 socket 压力；FD 高但 CPU 一般更像连接泄漏风险。\n"
    "3. **内存与 GC 压力**：old-gen 长期高位且 `GC Time Ratio` 上升，优先排查状态累积/泄漏。\n"
    "4. **线程死锁**：`Deadlocked Threads > 0` 需要立即做 thread dump 与锁链路分析。\n"
    "5. **任务卡住/吞吐不升**：若 completed/s 长期偏低且 rejection 上升，优先定位调度器或下游瓶颈。\n"
    "6. **恢复顺序**：先恢复分区安全，再降低 rejection 和队列积压。"
)

for panel in dashboard_zh.get("panels", []):
    title = panel.get("title")
    if title in title_map:
        panel["title"] = title_map[title]
    if panel.get("type") == "text" and panel.get("id") == 18:
        panel.setdefault("options", {})["content"] = playbook_zh

zh_file.write_text(json.dumps(dashboard_zh, ensure_ascii=False, indent=2) + "\n")
PY

echo "Default observability config generated:"
if [[ "$PROMETHEUS_USE_HTTP_SD" == "true" || "$PROMETHEUS_USE_HTTP_SD" == "1" ]]; then
  echo "  - prometheus mode : http_sd"
  echo "  - http_sd url     : $PROMETHEUS_HTTP_SD_URL"
else
  echo "  - prometheus mode : static_targets"
  echo "  - target metrics  : $SEATUNNEL_METRICS_TARGETS"
  echo "  - cluster label   : $SEATUNNEL_CLUSTER_LABEL"
fi
echo "  - prometheus   : $PROMETHEUS_URL"
echo "  - grafana      : $GRAFANA_URL"
