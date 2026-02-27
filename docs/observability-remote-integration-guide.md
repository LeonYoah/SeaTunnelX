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

### 6.3 开关回归（enabled true/false）

仓库内置脚本：`scripts/observability-enabled-switch-regression.sh`

```bash
./scripts/observability-enabled-switch-regression.sh
```

默认行为：

- 使用 `go run . api` 启动临时实例（保证测试当前源码而非历史二进制）；
- 自动验证两组用例：
  - `observability.enabled=true`：`/prometheus/discovery`、`/alertmanager/webhook` 期望 `200`
  - `observability.enabled=false`：上述接口期望 `404`

---

## 8. 常见问题（MVP）

### Q1：`/api/v1/monitoring/prometheus/discovery` 返回 404

优先检查：

1. `observability.enabled` 是否为 `true`；
2. 是否修改了 `observability.prometheus.http_sd_path`；
3. 反向代理是否把 `/api/v1/...` 路径改写掉。

### Q2：`/api/v1/monitoring/alertmanager/webhook` 返回 404

优先检查：

1. `observability.enabled` 是否为 `true`；
2. 是否修改了 `observability.alertmanager.webhook_path`；
3. Alertmanager 回调 URL 是否与 SeaTunnelX 暴露路径一致。

---

## 9. deps 三件套联调参考（2026-02-27 实测）

如果三件套已部署在 `deps` 目录（默认端口 `9090/9093/3000`），推荐直接使用内置脚本切到远程模式：

```bash
./deps/set-observability-remote-mode.sh http://127.0.0.1:8000
```

脚本会完成：

- 写入持久化 profile：`deps/runtime/observability.env`
- 重新生成 `prometheus.yml`（切换为 `http_sd_configs`）
- 重新生成 `alertmanager.yml`（追加 webhook receiver）
- best-effort 热重载 Prometheus / Alertmanager

> 从现在开始，执行 `deps/start-observability.sh` 时会自动加载该 profile，避免重启后被默认静态配置覆盖。

如需手工检查，关键点如下：

1. **Prometheus**
   - `seatunnel_engine_http` job 使用 `http_sd_configs`
   - URL 指向：
     `http://127.0.0.1:8000/api/v1/monitoring/prometheus/discovery`
   - 执行 reload：`POST http://127.0.0.1:9090/-/reload`

2. **Alertmanager**
   - 在 receiver 增加 webhook：
     `http://127.0.0.1:8000/api/v1/monitoring/alertmanager/webhook`
   - 热重载（SIGHUP）或重启 Alertmanager

3. **验证**
   - 执行：
     ```bash
     SEATUNNELX_USERNAME=admin \
     SEATUNNELX_PASSWORD=<password> \
     ./scripts/observability-remote-smoke.sh https://cpa.120501.xyz
     ```
   - 结果应覆盖并通过：
     - `prometheus/discovery`
     - `alertmanager/webhook`
     - `remote-alerts`
     - `clusters/health`
     - `platform-health`

---

## 7. 监控中心 UI（MVP）能力说明

当前监控中心（以 Grafana 为主）对应 MVP 已支持：

1. **平台健康摘要**
   - 数据来源：`GET /api/v1/monitoring/platform-health`
   - 展示总集群、健康/降级/异常集群、活动/严重告警

2. **集群健康表**
   - 数据来源：`GET /api/v1/clusters/health`
   - 支持一键跳转到集群详情和告警页（带 `cluster_id` 过滤）

3. **远程告警列表**
   - 数据来源：`GET /api/v1/monitoring/remote-alerts`
   - 支持筛选：`cluster_id`、`status`、`start_time`、`end_time`
   - 支持分页：`page`、`page_size`
