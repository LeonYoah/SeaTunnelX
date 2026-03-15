# 诊断包、报告与收集过程优化设计

> 设计思路（待评审后实施）

---

## 一、设计原则

- **言简意赅**：诊断包和 HTML 报告只保留核心现场证据，去掉冗余元数据
- **抓住重点**：报告以 Summary → Critical Findings → 关键证据三段式呈现，避免信息堆砌
- **收集即所需**：采集过程只产出用户分析问题所需的材料

---

## 二、诊断包内容精简

### 2.1 必须包含

| 内容 | 来源 | 说明 |
|------|------|------|
| 错误日志 | SeatunnelErrorEvent（时间范围内） | 按时间排序，与巡检窗口一致 |
| 告警信息 | AlertInstance（时间范围内） | 当前 firing 及近期 resolved 告警 |
| 指标数据 | Prometheus / 运行时采集 | CPU、内存、线程等的变化、异常、趋势 |

### 2.2 可选包含

| 内容 | 用户勾选 | 说明 |
|------|----------|------|
| 线程 Dump | 勾选 | 默认开启，便于分析阻塞、死锁 |
| JVM Dump | 勾选 | 默认关闭，体积大，内存分析时开启 |

### 2.3 移除/弱化

- **Manifest 元数据**：Version、CreatedBy、CreatedByName、完整 SourceRef 等 → 压缩或仅作内部用
- **执行过程元数据**：步骤 StartedAt/CompletedAt、节点详情等 → 不在包内冗余存多份，仅报告摘要展示
- **Config 快照**：保留，但这里特指 **SeaTunnel 集群运行配置文件**（如 `seatunnel.yaml`、`hazelcast.yaml`、`hazelcast-master.yaml`、`hazelcast-worker.yaml` 等）；后续聚焦「指定时间点及前后变更」，而不是控制面 clusterInfo 原样 dump

---

## 三、HTML 报告精简

### 3.1 目标结构（三段式）

```
┌─────────────────────────────────────────┐
│ 1. Summary 摘要                          │
│    - 时间范围、发现数、影响范围（一句话）   │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│ 2. Critical Findings 关键发现            │
│    - 按严重级别：严重 > 告警 > 信息       │
│    - 每条：标题 + 摘要 + 证据要点（非全文）│
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│ 3. 证据详情（按需展开）                   │
│    - 错误日志：时间线 + 关键行            │
│    - 告警：名称 + 状态 + 简述             │
│    - 指标：异常/趋势图或表（如有）        │
│    - 线程 Dump / JVM Dump：链接或摘要     │
└─────────────────────────────────────────┘
```

### 3.2 移除/弱化

- **影响范围大段统计**：Cluster Nodes、Error Occurrences、Critical Findings、Firing Alerts 等卡片 → 合并为 Summary 中一句
- **Recommendations 长篇**：优先动作 / Recommended Next Step → 仅在确有建议时简短列出
- **执行步骤/节点完整列表**：Steps、Nodes 详细表格 → 不在报告主体展示，仅「查看执行日志」弹窗内使用
- **中英双语重复**：如 "影响范围 / Impact Scope" → 以中文为主，英文可选或去除

---

## 四、收集过程优化

### 4.1 当前步骤 vs 目标

| 当前步骤 (DiagnosticStepCode) | 目标 |
|------------------------------|------|
| COLLECT_ERROR_CONTEXT | 保留，仅采集时间范围内的错误事件 |
| COLLECT_PROCESS_EVENTS | 保留，进程事件（重启、崩溃等） |
| COLLECT_ALERT_SNAPSHOT | 保留，告警快照 |
| COLLECT_CONFIG_SNAPSHOT | 弱化，仅保留与异常相关的配置（或移除） |
| COLLECT_LOG_SAMPLE | 保留，日志采样 |
| COLLECT_THREAD_DUMP | 可选，由用户勾选 |
| COLLECT_JVM_DUMP | 可选，由用户勾选 |
| ASSEMBLE_MANIFEST | 精简 Manifest 字段 |
| RENDER_HTML_SUMMARY | 按上述三段式模板重写 |
| COMPLETE | 保留 |

### 4.2 时间范围

- 默认与巡检 `lookback_minutes` 一致
- 支持用户再次选择（如历史报错与当前有关联时扩大窗口）
- 支持几小时几分钟
- 需后端 `CreateDiagnosticTaskRequest` 支持 `lookback_minutes` 覆盖

### 4.3 指标采集（基于现有 Observability）

- 数据源：**Prometheus + Alertmanager + Grafana**，通过 `observability` 配置与 Prometheus HTTP SD（`/monitoring/prometheus/discovery`）打通
- 指标范围：以 Prometheus 中现有 CPU、内存、FD、线程、失败任务等指标为主（见 metrics 模板与策略中心）
- Agent：进程事件（启动/停止/崩溃/重启等）由 Agent 上报并落库，用于补充「频繁重启、进程一直未启动」等本地事件
- 报告侧：只引用与当前诊断窗口相关的「异常片段」或「趋势摘要」，不在 HTML 中平铺原始 time-series

---

## 五、实施顺序建议

1. **HTML 报告模板**：先按三段式精简现有模板，去掉冗余区块
2. **诊断包 Manifest**：压缩元数据，保持向后兼容
3. **收集步骤**：CONFIG 弱化、THREAD_DUMP/JVM_DUMP 与用户勾选联动
4. **时间范围可调**：后端支持 lookback 覆盖，前端确认弹窗可编辑
5. **指标采集**：单独迭代，依赖指标数据源就绪

---

## 六、确认结论

- [x] 指标数据源：**已接入 Prometheus / Alertmanager / Grafana**，并通过 HTTP SD 和健康探测集成；Agent 通过进程监控模块上报进程事件
- [x] Config 快照：**需要**，且这里指 **SeaTunnel 集群运行配置文件快照**；后续以「指定时间点配置 + 诊断时间范围内的变更记录」形式保留
- [x] 报告语言：**保留中英双语**，但结构与信息密度按本设计精简

---

## 七、当前实现进度（2026-03-14）

### 已落地

- [x] 诊断包采集时间窗口开始按 `lookback_minutes` 收敛：
  - 错误事件：按诊断窗口过滤
  - 进程事件：优先走带过滤查询；兜底时做内存过滤
  - 告警快照：保留“窗口内 resolved + 当前仍在 firing”的相关告警，并保留显式来源告警
- [x] `COLLECT_CONFIG_SNAPSHOT` 已切换为采集 **SeaTunnel 实际运行配置文件**：
  - 通过 Agent `pull_config` 拉取 `seatunnel.yaml`
  - 按部署模式/节点角色拉取 `hazelcast.yaml` 或 `hazelcast-master.yaml` / `hazelcast-worker.yaml`
  - 补充 `hazelcast-client.yaml`
  - 补充 `log4j2.properties`、`log4j2_client.properties`、`plugin_config`、`jvm_*_options`
  - 补充 `config/`、`lib/`、`connectors/` 目录文件清单，便于确认运行配置、依赖与连接器安装情况（含旁路文件如 `.bak`）
  - 结合 `configs` 表按诊断时间窗口回溯配置变更记录
  - 产物以实际文件形式落盘，并生成 `config-snapshot.json` 摘要
- [x] 指标数据已真正进入诊断报告：
  - 后端直接查询 Prometheus `query_range`
  - 当前采集 CPU、JVM Heap、Old Gen、GC 时间占比、FD、死锁线程、作业线程池拒绝数、Hazelcast 分区安全
  - Prometheus 异常不会中断整份诊断包，只在报告中记录采集备注
- [x] Manifest 已做第一轮瘦身：
  - 移除 `created_by` / `created_by_name`
  - 移除 `started_at` / `completed_at`
  - 增加 `lookback_minutes` / `window_start` / `window_end`
- [x] HTML 报告已做第一轮结构收敛：
  - 导航调整为“摘要 / 关键发现 / 证据详情 / 诊断产物 / 附录”
  - 增加独立“关键发现”区块
  - 任务概览 / 执行过程降级为附录语义
  - 运行配置文件、目录清单、Prometheus 指标快照已进入证据详情
- [x] 已补充后端回归测试：
  - 诊断时间窗口解析
  - 告警窗口过滤
  - Manifest 精简字段验证

### 仍待继续

- [ ] Key Runtime Settings 继续按 SeaTunnel 版本补充更细的关键字段（当前已覆盖 metrics / hazelcast / JVM / log4j 的首批关键项）
- [ ] HTML 报告继续减少中英重复与长表格暴露（当前已完成首批折叠收敛，但仍可继续压缩信息密度）

### 已继续落地（2026-03-15）

- [x] Config 快照已从“纯文件快照”继续收敛为“关键配置摘要 + 原始文件清单”双层结构：
  - 自动提取 `seatunnel.yaml` / `hazelcast*.yaml` / `hazelcast-client.yaml` 中的关键运行参数
  - 自动提取 `jvm_*_options` 中的 Heap / Direct Memory / HeapDump / OOM 相关参数
  - 自动识别 `log4j2*.properties` 中的 root level、主日志文件、RoutingAppender 状态
  - `plugin_config` 生成轻量规则摘要
- [x] HTML 报告第二轮减噪已落地：
  - “关键配置摘要”前置，时间窗内配置变化改成时间线展示
  - 原始运行配置文件清单、目录清单、采集备注改为折叠展开
  - Prometheus 指标改成“优先关注指标 + 其余指标折叠”的展示方式

### 已识别限制（2026-03-14 补充）

- [x] 错误中心当前采集范围维持为 **Seatunnel engine / job 日志**（如 `seatunnel-engine-*.log`、`job-*.log`）
- [x] 用户手工执行 `seatunnel.sh` 时若错误仅出现在交互式终端 stdout/stderr，则**不属于当前错误中心采集范围**

---

## 八、自测记录（2026-03-15）

### 8.1 温和注入验证

- [x] **纯噪声包装块**（`Fatal Error` / `Please submit bug report` / `Reason:SeaTunnel job executed failed`）已验证：
  - 当 evidence 首行带完整 Seatunnel 日志头时，后端会先剥离日志头再做噪声归一化；
  - 噪声块**不会进入错误组/错误事件表**；
  - 同时修复了一个真实问题：**被忽略的噪声也必须推进 log cursor**，否则 Agent 会在每轮扫描中重复上报同一段噪声。
- [x] **真实包装异常归并**已验证：
  - 追加 `Failed to initialize connection. Error: DEADLINE_EXCEEDED ...` 样本后，
    新事件成功归并到现有 `DEADLINE_EXCEEDED` 错误组；
  - 未新增分组，现有组 `occurrence_count` 正常递增。

### 8.2 激进扰动验证

- [x] **真实 `restart_failed`** 已验证（北京时间 **2026-03-15 00:43:47**）：
  - 临时替换 `seatunnel-cluster.sh` 为失败脚本；
  - 杀掉真实 SeaTunnel Java 进程；
  - Agent 在 3 次连续失败检查后触发 crash，再经过 10 秒延迟执行自动拉起；
  - 控制面成功落到 `process_events.event_type = restart_failed`。
- [x] **真实 JVM GC 扰动** 已验证：
  - 使用 `jcmd <pid> GC.run` 持续触发 GC；
  - Prometheus 1 分钟窗口内：
    - CPU 约从 `0.024` 抬升到 `0.52`
    - GC 时间占比约从 `0` 抬升到 `37.29`
  - 说明诊断报告中的 CPU / GC 信号可在真实扰动下反映问题。
- [x] **真实 `restart_failed details` 端到端复核** 已完成（北京时间 **2026-03-15 01:17:37**）：
  - 再次真实打断 Seatunnel 进程并制造启动失败；
  - 控制面 `process_events` 已落到完整 details：
    - `crashed.details = {"consecutive_fails":"3","install_dir":"...","role":"master/worker"}`
    - `restart_failed.details = {"error":"process failed to start: start script failed: exit status 1","install_dir":"...","role":"master/worker"}`
  - 说明 Agent → gRPC → Monitor Service → DB 的 details 透传已打通。
- [x] **Heap / Old Gen 持续升高场景** 已验证（使用可回收 Java Agent 注入）：
  - 基线（北京时间 **2026-03-15 01:19** 左右）：
    - Heap ratio ≈ `0.137`
    - Old Gen ratio ≈ `0.065`
  - 注入 35 个 `10MB` 持有块后，Prometheus 观测：
    - `t=20s`：Heap ≈ `0.422`，Old Gen ≈ `0.407`
    - `t=45s`：Heap ≈ `0.430`，Old Gen ≈ `0.407`
    - `t=75s`：Heap ≈ `0.436`，Old Gen ≈ `0.407`
  - 自动释放并触发 GC 后：
    - `t=110s`：Heap ≈ `0.064`，Old Gen ≈ `0.057`
  - 说明诊断报告中的 Heap / Old Gen 信号可反映**持续占用**与**释放回落**。

### 8.3 本轮自测暴露并已修复的问题

- [x] **错误归一化未剥离日志头**：
  - 导致 `Fatal Error` 这类包装噪声在 evidence 首行为完整日志头时仍可能落库；
  - 已在 `normalize.go` 中补齐 Seatunnel 日志头剥离规则，并补单测。
- [x] **忽略噪声但未推进 cursor**：
  - 导致 Agent 会反复重报同一段噪声；
  - 已在 `IngestSeatunnelError` 中修复，并补服务层测试。
- [x] **Agent 上报进程事件时丢失 details**：
  - 真实 `restart_failed` 验证中发现 `process_events.details = null`；
  - 原因是 Agent 仅提取了 `install_dir/role`，但未把 `event.Details` 写入 protobuf `report.Details`；
  - 已修复 Agent 映射逻辑，并补单测。

### 8.4 仍建议后续补做

- [x] 继续验证 **restart_failed details** 的端到端回归（已完成）
- [x] 继续补做 **Heap/Old Gen 持续上升** 场景（已完成）
- [ ] 是否做真实 **OOM**：当前建议**暂不执行**
  - 原因 1：`restart_failed`、CPU / GC、Heap / Old Gen 这三类关键诊断信号已完成真实验证；
  - 原因 2：真实 OOM 会把 JVM 推到接近不可恢复边缘，额外收益主要是验证 `OutOfMemoryError` 错误采集与 OOM 后恢复链路；
  - 原因 3：若要做，建议拆成单独一次窗口执行，并提前准备：
    - 恢复脚本
    - JVM dump 磁盘容量检查
    - 失败后自动拉起与报告回收策略

---

## 九、本轮提交面与 PR 摘要建议（2026-03-15）

### 9.1 建议纳入“统一诊断链路自测修复”代码提交的文件

- `internal/apps/diagnostics/normalize.go`
- `internal/apps/diagnostics/normalize_test.go`
- `internal/apps/diagnostics/service.go`
- `internal/apps/diagnostics/service_test.go`
- `agent/cmd/main.go`
- `agent/cmd/main_test.go`
- `internal/apps/diagnostics/inspection_service_test.go`

> 说明：
>
> - 这批文件对应本轮真实自测暴露的 3 个修复点：
>   1. Seatunnel 包装噪声识别与日志头剥离
>   2. 忽略噪声时仍推进 log cursor
>   3. Agent 上报 `restart_failed` / `crashed` 时透传 details
> - 其中 `service.go`、`agent/cmd/main.go` 当前与前序诊断能力改动存在**同文件混改**，正式提交时建议：
>   - 使用交互式暂存拆 hunk；或
>   - 先整理成独立 commit，再与“报告/诊断包能力”分开提交

### 9.2 建议纳入“文档 / 自测沉淀”提交的文件

- `.trellis/tasks/03-11-diagnostics-redesign-unified-flow/design-bundle-report-collection.md`

### 9.3 当前不建议混入本轮自测修复提交的改动

- 前端巡检页参数/文案调整：
  - `frontend/components/common/diagnostics/DiagnosticsInspectionCenter.tsx`
  - `frontend/lib/services/diagnostics/types.ts`
  - `frontend/lib/i18n/locales/zh.json`
  - `frontend/lib/i18n/locales/en.json`
- 诊断报告 / 诊断包能力演进：
  - `internal/apps/diagnostics/task_execute.go`
  - `internal/apps/diagnostics/inspection_models.go`
  - `internal/apps/diagnostics/inspection_report_service.go`
  - `internal/apps/diagnostics/inspection_service.go`
  - `internal/apps/config/repository.go`
  - `internal/router/router.go`
- 监控侧其他修复：
  - `internal/apps/monitoring/service.go`
  - `internal/apps/monitoring/node_offline_evaluator_test.go`
- 工作区/临时文件：
  - `.trellis/.template-hashes.json`
  - `.trellis/tasks/00-bootstrap-guidelines/`
  - `.tmp/`

### 9.4 可直接写进 PR 描述的摘要

#### 背景

本轮围绕“统一诊断链路”做了一次真实集群自测，重点验证：

- Seatunnel 错误日志去噪与归并
- Agent 自动拉起失败事件的 details 透传
- 诊断报告中的 JVM / Prometheus 信号对真实扰动的可见性

#### 修复点

1. 修复 Seatunnel 包装噪声在带日志头时仍可能落库的问题
2. 修复忽略噪声后未推进 log cursor，导致重复上报的问题
3. 修复 Agent 上报进程事件时 `details` 丢失，导致 `restart_failed.details = null`

#### 自测结论

- 已验证 `DEADLINE_EXCEEDED` 包装异常可正确归并到现有错误组
- 已真实验证 `restart_failed` 与 `restart_failed.details`
- 已真实验证 CPU / GC / Heap / Old Gen 扰动能进入诊断信号
- 当前建议暂不执行真实 OOM，后续单独窗口评估

#### 风险与回滚

- 风险面主要集中在：
  - 错误归一化规则变化后，是否会误伤真实异常
  - Agent 进程事件 details 扩展后，是否影响既有 gRPC 上报兼容性
- 回滚策略：
  - 可直接回滚本轮 normalize / ingest / agent report 相关 commit
  - 不涉及数据库 schema 变更，无额外数据迁移回滚成本
