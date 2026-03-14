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
- **Config 快照**：保留，但聚焦「指定时间点及前后变更」——记录关键配置键在诊断时间范围内的变更轨迹，而不是完整原样 dump

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
- [x] Config 快照：**需要**，以「指定时间点配置 + 诊断时间范围内的变更记录」形式保留
- [x] 报告语言：**保留中英双语**，但结构与信息密度按本设计精简
