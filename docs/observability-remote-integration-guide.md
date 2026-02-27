# SeaTunnelX 可观测性远程接入指南（MVP）

## 1. 目标

本指南用于把已有的 Prometheus / Alertmanager / Grafana 与 SeaTunnelX 远程对接，形成最小闭环：

- Prometheus 从 SeaTunnelX 拉取 HTTP SD 目标；
- Alertmanager 将告警推送回 SeaTunnelX；
- SeaTunnelX 提供告警查询和健康聚合 API；
- Grafana 导入 SeaTunnelX 提供的默认 Dashboard JSON。

---

## 2. SeaTunnelX 配置

在 `config.yaml` 中启用：

```yaml
app:
  external_url: "https://your-seatunnelx.example.com"

observability:
  enabled: true
  prometheus:
    url: "http://prometheus.example.com:9090"
    http_sd_path: "/api/v1/monitoring/prometheus/discovery"
  alertmanager:
    url: "http://alertmanager.example.com:9093"
    webhook_path: "/api/v1/monitoring/alertmanager/webhook"
  grafana:
    url: "http://grafana.example.com:3000"
```

说明：

- `observability.enabled=true` 时，SeaTunnelX 启用远程集成 API；
- `http_sd_path` / `webhook_path` 可按需修改，但建议保持默认；
- `app.external_url` 必须是可外部访问的 HTTP(S) 地址。

---

## 3. Prometheus 配置（一次性）

在 `prometheus.yml` 增加：

```yaml
scrape_configs:
  - job_name: 'seatunnel_engine_http'
    metrics_path: /metrics
    scheme: http
    http_sd_configs:
      - url: https://your-seatunnelx.example.com/api/v1/monitoring/prometheus/discovery
        refresh_interval: 30s
```

---

## 4. Alertmanager 配置（一次性）

在 `alertmanager.yml` 增加 receiver：

```yaml
route:
  receiver: 'seatunnelx'

receivers:
  - name: 'seatunnelx'
    webhook_configs:
      - url: 'https://your-seatunnelx.example.com/api/v1/monitoring/alertmanager/webhook'
        send_resolved: true
```

---

## 5. Grafana 默认面板

默认 Dashboard JSON 已固定放在：

- `deps/grafana_config/dashboards/seatunnel-overview-en.json`
- `deps/grafana_config/dashboards/seatunnel-overview-zh.json`

建议：

1. Grafana 中先创建 Prometheus 数据源，UID 使用 `prometheus`；
2. 导入上述 JSON；
3. 通过语言切换使用中英文面板 UID：
   - `seatunnel-overview-en`
   - `seatunnel-overview-zh`

---

## 6. 联调与 Smoke 测试

仓库内置脚本：`scripts/observability-remote-smoke.sh`

### 6.1 仅验证公开接口

```bash
./scripts/observability-remote-smoke.sh https://your-seatunnelx.example.com
```

会检查：

- Prometheus HTTP SD 接口；
- Alertmanager Webhook 入库接口。

### 6.2 额外验证登录后接口

```bash
SEATUNNELX_USERNAME=admin \
SEATUNNELX_PASSWORD=admin \
./scripts/observability-remote-smoke.sh https://your-seatunnelx.example.com
```

会额外检查：

- `GET /api/v1/monitoring/remote-alerts`
- `GET /api/v1/clusters/health`
- `GET /api/v1/monitoring/platform-health`

