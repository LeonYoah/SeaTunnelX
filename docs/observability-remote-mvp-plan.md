# SeaTunnelX 可观测性（远程集成）MVP 规划

基于 `docs/observability-remote-design.md`。

## 1. MVP 目标

在不托管三件套进程（Prometheus/Alertmanager/Grafana）的前提下，完成最小可用远程集成闭环：

1. Prometheus 通过 SeaTunnelX HTTP SD 动态发现所有集群 metrics 目标；
2. Alertmanager 通过固定 webhook 推送告警到 SeaTunnelX；
3. SeaTunnelX 完成告警入库并可查询；
4. SeaTunnelX 提供集群级与平台级健康摘要 API；
5. Grafana 看板 JSON 以固定目录交付，支持导入使用。

---

## 2. 范围

## In Scope（MVP）

### M1：接入基础能力（本轮优先）

- 配置模型调整（远程模式）：
  - 保留 `observability.enabled` 作为总开关；
  - 新增：
    - `observability.prometheus.http_sd_path`
    - `observability.alertmanager.webhook_path`
- 启动校验（fail-fast）：
  - 当 `observability.enabled=true` 时，校验 `app.external_url` 为合法 HTTP(S) URL；
  - 校验三件套 URL 格式合法；
- 新增公开接口：
  - `GET /api/v1/monitoring/prometheus/discovery`
  - `POST /api/v1/monitoring/alertmanager/webhook`
- Alertmanager webhook 告警落库（最小字段集 + 去重 upsert）。

### M2：可视化闭环

- 新增告警查询 API（按集群、状态、时间过滤）；
- 新增集群健康与平台健康聚合 API：
  - `GET /api/v1/clusters/health`
  - `GET /api/v1/monitoring/platform-health`
- Grafana Dashboard JSON 固定目录交付与下载说明。

### M3：联调与发布

- Prometheus/Alertmanager/Grafana 对外接入手册；
- 联调脚本与 smoke 测试；
- 兼容性与回归验证。

## Out of Scope（MVP 外）

- SeaTunnelX 拉起与管理三件套进程；
- Alertmanager/Silence 高级策略编排 UI；
- 告警通知渠道高级路由器；
- 多租户隔离与复杂 RBAC。

---

## 3. M1 详细拆解

### 3.1 配置模型

- [x] `internal/config/model.go` 增加：
  - `ObservabilityPrometheusConfig.HTTPSDPath`
  - `ObservabilityAlertmanagerConfig.WebhookPath`
- [x] `internal/config/config.go` 增加默认值：
  - `http_sd_path: /api/v1/monitoring/prometheus/discovery`
  - `webhook_path: /api/v1/monitoring/alertmanager/webhook`
- [x] `config.example.yaml` 同步更新。

### 3.2 启动校验

- [x] `observability.enabled=true` 时：
  - [x] `app.external_url` 非空且为 `http/https`；
  - [x] `observability.prometheus.url` / `alertmanager.url` / `grafana.url` 为合法 URL。
- [x] 校验失败 fail-fast（启动失败并打印明确错误）。

### 3.3 HTTP SD 接口

- [x] 新增 handler：`GET /api/v1/monitoring/prometheus/discovery`
- [x] 返回 Prometheus HTTP SD 标准数组：
  - `targets`: `host:port`
  - `labels`: `cluster_id` / `cluster_name` / `env`
- [x] 仅返回可探测通过的 metrics 目标（MVP 可按现有探测逻辑）。

### 3.4 Alertmanager Webhook

- [x] 新增 handler：`POST /api/v1/monitoring/alertmanager/webhook`
- [x] 接收 Alertmanager webhook 标准 payload；
- [x] 落库字段（MVP）：
  - `fingerprint`, `status`, `alertname`, `severity`, `cluster_id`, `cluster_name`, `env`, `starts_at`, `ends_at`, `summary`, `description`, `labels_json`, `annotations_json`, `last_received_at`
- [x] 去重策略：`fingerprint + starts_at` 唯一 upsert。

---

## 4. 验收标准（M1）

- [x] `observability.enabled=true` 且 `app.external_url` 非法时，SeaTunnelX 启动失败并提示具体错误；
- [x] Prometheus 请求 HTTP SD 接口可拿到标准 TargetGroup JSON；
- [x] Alertmanager webhook 推送后，告警记录可在数据库中查询到，重复告警可正确 upsert；
- [x] `observability.enabled=false` 时，不注册以上公开接口。

---

## 5. M2/M3 进度追踪（持续更新）

### M2：可视化闭环

- [x] 告警查询 API：`GET /api/v1/monitoring/remote-alerts`
- [x] 集群健康聚合 API：`GET /api/v1/clusters/health`
- [x] 平台健康聚合 API：`GET /api/v1/monitoring/platform-health`
- [x] 告警状态/时间/分页过滤能力（MVP）

### M3：联调与发布

- [x] 对外接入手册（`docs/observability-remote-integration-guide.md`）
- [x] 联调脚本与 smoke 测试（`scripts/observability-remote-smoke.sh`）
- [x] Grafana Dashboard 固定目录交付（`deps/grafana_config/dashboards`）

---

## 6. 提交建议

1. `docs(observability): add remote integration mvp plan`
2. `feat(config): add remote observability paths and startup validation`
3. `feat(monitoring): add prometheus http-sd endpoint`
4. `feat(monitoring): add alertmanager webhook ingest and persistence`
