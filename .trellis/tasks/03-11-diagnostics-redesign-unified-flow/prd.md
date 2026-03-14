# 诊断中心重设计：统一巡检流程 + 条件模板自动触发

## 背景

当前实现偏离了原始设计意图：巡检（Inspection）和诊断包任务（DiagnosticTask）被设计成两个并列的顶层概念（两个独立 Tab），导致用户心智割裂。原始意图是：**巡检是唯一入口，诊断包是巡检完成后的后续动作**。

## Goal

重新对齐诊断中心的架构，使其回归原始意图：
1. 巡检是唯一的主流程入口（手动 + 自动）
2. 诊断包生成是巡检完成后的按钮动作，不是独立 Tab
3. 错误中心保留在诊断中心（Tab 不变）
4. 自动巡检基于条件模板配置，支持 Java 严重错误、Prometheus 指标异常、本地事件（频繁重启、进程未启动等）
5. 巡检与告警关联：发现异常时提醒用户执行巡检，巡检负责汇总错误信息供用户分析

## 架构变更

### Workspace Tab 变更

**Before（3个 Tab）：**
- errors / inspections / tasks

**After（2个 Tab）：**
- errors（错误中心，不变）
- inspections（巡检中心，新增详情页）

### 新增路由

- `/diagnostics/inspections/:id` — 巡检详情页（Findings + 生成诊断包入口）
- `/diagnostics/inspections/:id/bundle` 或同页展开 — 诊断包执行详情

### 自动巡检配置入口

- 诊断中心右上角「自动巡检设置」按钮，或 `/settings/diagnostics`

## Requirements

### R1：Workspace Tab 精简

- 移除 `tasks` Tab（诊断任务不再作为顶层导航）
- 保留 `errors` 和 `inspections` 两个 Tab
- `WorkspaceBoundary` 概念删除（无用户价值）

### R2：巡检详情页

- 路由：`/diagnostics/inspections/:id`
- 展示巡检执行状态（pending → running → completed / failed）
- 展示 Findings 列表，按 critical → warning → info 排序
- 当状态为 `completed` 且有 Findings 时，显示「一键生成诊断包和诊断报告」按钮（首次）/「重新生成」按钮（已生成后）
- 诊断包执行完成后，展示：下载报告、下载诊断包、重新生成 按钮
- **移除**：独立的「查看诊断详情」页面；诊断包执行信息改为「查看执行日志」弹窗展示（步骤、失败原因等）
- **移除**：「先只查看结果」按钮及对应流程

#### R2.1：巡检中心布局去重

- **移除**：右侧「检查结果」中第一个卡片（与左侧检查列表重复的摘要块）
- 右侧直接展示 Findings 列表，无需重复的报告摘要卡片

### R3：诊断包与报告精简

**诊断包内容**：只保留核心现场证据，去掉冗余元数据

| 必须包含 | 可选包含 |
|----------|----------|
| 时间范围内的错误日志 | 线程 Dump (thread_dump) |
| 告警信息 | JVM Dump (jvm_dump) |
| CPU / 内存 / 线程等指标的变化、异常、趋势 | - |

**HTML 报告**：言简意赅，抓重点，避免信息堆砌

**生成确认弹窗**：点击「一键生成诊断包」或「重新生成」时弹出，包含：
- 确认提示
- 时间范围（默认与巡检一致，可调整；历史报错可能与当前有关联）
- 是否包含线程 Dump（勾选）
- 是否包含 JVM Dump（勾选）

**其他**：
- API 响应补充 `cluster_name`
- Findings 默认按严重级别排序
- `related_error_group_id` 不为零时，可点击跳转错误中心

### R3.1：【发现的问题】页面精简

- **只展示**：发现的报错、异常指标、异常事件
- **移除**：「当前正在围绕这项排查」「查看关联错误」「查看更多诊断选项」等操作入口

### R4：自动巡检触发条件模板

#### 后端：新增 InspectionAutoPolicy 模型

```go
type InspectionAutoPolicy struct {
    ID               uint
    ClusterID        uint       // 0 = 全局策略
    Enabled          bool
    Conditions       []InspectionConditionItem  // JSON 存储
    CooldownMinutes  int        // 冷却时间，防重复触发
    CreatedAt        time.Time
    UpdatedAt        time.Time
}

type InspectionConditionItem struct {
    TemplateCode string                 // 对应 BuiltinConditionTemplate.Code
    Enabled      bool
    Overrides    map[string]interface{} // 用户调整的阈值，覆盖模板默认值
}
```

#### 巡检数据来源扩展（R1.5 新增）

巡检不仅收集错误日志，还应纳入：

| 来源 | 说明 | 与告警的关系 |
|------|------|--------------|
| 错误日志 | 现有 SeatunnelErrorEvent | 错误激增可触发自动巡检 |
| Prometheus 指标异常 | GC 频繁、堆内存上涨、CPU 高负载等 | 指标异常可触发自动巡检 |
| 本地事件 (ProcessEvent) | 频繁重启、进程一直未启动、崩溃、重启失败 | 与告警联动，异常时提醒用户巡检 |
| 告警触发 | 指定告警规则 firing | 告警即触发巡检 |

**设计原则**：巡检负责搜罗各类异常信号（错误、指标、事件），汇总后供用户分析；告警发现问题后引导用户去巡检。

#### 内置条件模板目录

| Code | Category | Name | 触发逻辑 |
|------|----------|------|----------|
| `JAVA_OOM` | java_error | Java 内存溢出 (OOM) | exception_class 含 OutOfMemoryError → 立即触发 |
| `JAVA_STACKOVERFLOW` | java_error | Java 栈溢出 | exception_class = StackOverflowError → 立即触发 |
| `JAVA_METASPACE` | java_error | Metaspace 耗尽 | message 含 "Metaspace" → 立即触发 |
| `PROM_GC_FREQUENT` | prometheus | GC 频繁 | jvm_gc_pause_seconds_count rate 5m > N 次/分钟，持续 M 分钟 |
| `PROM_HEAP_RISING` | prometheus | 堆内存持续上涨 | jvm_memory_used_bytes{area="heap"} 连续 N 分钟单调递增 |
| `PROM_HEAP_HIGH` | prometheus | 堆内存使用率高 | used/max > N% 持续 M 分钟 |
| `PROM_CPU_HIGH` | prometheus | CPU 持续高负载 | process_cpu_usage > N% 持续 M 分钟 |
| `ERROR_SPIKE` | error_rate | 错误频率激增 | M 分钟内错误数 > N 条 |
| `NODE_UNHEALTHY` | node_unhealthy | 节点持续异常 | N 个节点异常持续 M 分钟 |
| `PROCESS_FREQUENT_RESTART` | local_event | 进程频繁重启 | 时间窗口内重启次数 > N |
| `PROCESS_NOT_STARTED` | local_event | 进程一直未启动 | 进程预期运行但持续未启动 |
| `ALERT_FIRING` | alert_firing | 告警规则触发 | 指定告警规则 firing |
| `SCHEDULED` | schedule | 定时巡检 | Cron 表达式 |

#### java_error 触发路径

```
agent 上报错误事件（exception_class + message）
    ↓
错误事件入库 → 策略检查器扫描 InspectionAutoPolicy
    ↓
命中 java_error 条件 → 检查冷却时间
    ↓
冷却未命中 → 创建 InspectionReport（trigger_source = "auto"）+ 记录触发原因
```

#### InspectionTriggerSource 新增 auto

```go
InspectionTriggerSourceAuto = "auto"
```

InspectionReport 新增字段：
```go
AutoTriggerReason string  // e.g. "JAVA_OOM: java.lang.OutOfMemoryError"
```

### R5：自动巡检配置 UI

- 策略列表页（按集群分组，含全局策略）
- 策略编辑弹窗：
  - 集群选择（全局 or 指定集群）
  - 条件模板勾选列表（分 Category 展示）
  - 勾选后可展开调整阈值（仅对 prometheus / error_rate / schedule 类型）
  - java_error 类型不需要阈值调整，仅开关
  - 冷却时间设置

## Acceptance Criteria

- [ ] Workspace 只有 2 个 Tab（errors / inspections），Task Tab 已移除
- [ ] 点击巡检记录进入详情页，展示 Findings（按严重级别排序）
- [ ] 详情页在 completed + 有 Findings 时出现「一键生成诊断包和诊断报告」按钮；已生成后显示「重新生成」
- [ ] 生成/重新生成时弹出确认弹窗（时间范围、线程 Dump、JVM Dump 勾选）
- [ ] 诊断包完成后展示「下载报告」「下载诊断包」「重新生成」按钮
- [ ] 「查看执行日志」弹窗展示步骤及失败原因（替代原诊断详情页内联展示）
- [ ] 巡检中心右侧移除与列表重复的第一个卡片
- [ ] 【发现的问题】仅展示发现项，移除「当前正在围绕这项排查」「查看关联错误」「查看更多诊断选项」
- [ ] API 响应包含 cluster_name
- [ ] Findings 中的 error_group 链接可跳转
- [ ] InspectionAutoPolicy CRUD API 实现
- [ ] 内置条件模板包含 local_event（PROCESS_FREQUENT_RESTART、PROCESS_NOT_STARTED）
- [ ] InspectionTriggerSource 支持 `auto`，新增 AutoTriggerReason 字段
- [ ] java_error 条件触发路径打通（agent 错误事件 → 策略检查 → 巡检创建）
- [ ] Prometheus 指标条件检查器框架搭建（至少 GC 和堆内存两个模板实现）
- [ ] 自动巡检配置 UI 可用（策略列表 + 编辑弹窗）
- [ ] 文案符合专业产品用语规范

## 文案规范（R5.1）

- 使用**专业、正式**的产品用语，避免口语化表达
- 示例：不以「你可以直接继续抓现场证据」等口语呈现，改为「支持一键生成诊断包以采集现场证据」

## 表与模型冗余分析（待确认）

当前涉及监控/巡检的表：

| 表/模型 | 用途 | 状态建议 |
|---------|------|----------|
| `ClusterInspectionReport` | 巡检报告 | 保留 |
| `ClusterInspectionFinding` | 巡检发现项 | 保留 |
| `DiagnosticTask` / `DiagnosticTaskStep` / `DiagnosticNodeExecution` | 诊断包执行 | 保留（仅从前端 Tab 隐藏） |
| `InspectionAutoPolicy` | 自动巡检策略 | 保留 |
| `MonitorConfig` | 进程监控配置 | 保留 |
| `ProcessEvent` | 进程事件（重启、崩溃等） | 保留，巡检/告警均依赖 |
| `AlertRule` / `AlertPolicy` / `AlertState` 等 | 告警体系 | 保留 |
| `WorkspaceBoundary` | 诊断工作台边界 | PRD 已要求删除 |
| 其他 `monitoring.*` 表 | 通知、投递等 | 需结合实际调用确认 |

**建议**：实施前做一次依赖扫描，标记未被引用的表或字段，便于后续清理。

---

## Technical Notes

### 补充需求（2026-03-13）

- 优先排查并修复多节点场景下的诊断中心相关问题。
- 排查范围同时覆盖：
  - 错误中心的错误日志采集归属展示（需要让用户知道错误来自哪个节点）
  - 多节点下巡检 / Findings / 诊断包执行链路是否正常
- 本轮工作以“先验证、发现问题后直接修复”为目标，而不是仅输出问题清单。

### 保留现有代码

- `ClusterInspectionReport` / `ClusterInspectionFinding` 数据模型保留
- `DiagnosticTask` / `DiagnosticTaskStep` / `DiagnosticNodeExecution` 后端模型保留（只从前端 Tab 隐藏）
- 错误中心（errors Tab）不动

### 实施顺序建议

1. 后端：InspectionAutoPolicy 模型 + CRUD API
2. 后端：内置模板注册 + java_error 触发检查器
3. 后端：Prometheus 指标检查器框架（接入已有 Prometheus client）
4. 后端：local_event 检查器（PROCESS_FREQUENT_RESTART、PROCESS_NOT_STARTED）
5. 后端：InspectionReport 新增 auto source + AutoTriggerReason
6. 前端：移除 Task Tab，调整 Workspace 为 2 Tab
7. 前端：巡检中心布局去重（移除右侧重复卡片）
8. 前端：巡检详情页（Findings + 一键生成诊断包 + 确认弹窗 + 查看执行日志弹窗）
9. 前端：【发现的问题】精简（移除多余操作入口）
10. 前端：诊断包内容与 HTML 报告精简
11. 前端：自动巡检配置 UI
12. 文案：全文梳理为专业产品用语

### Dev Type

fullstack
