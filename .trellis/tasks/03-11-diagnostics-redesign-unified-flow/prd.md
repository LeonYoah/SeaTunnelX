# 诊断中心重设计：统一巡检流程 + 条件模板自动触发

## 背景

当前实现偏离了原始设计意图：巡检（Inspection）和诊断包任务（DiagnosticTask）被设计成两个并列的顶层概念（两个独立 Tab），导致用户心智割裂。原始意图是：**巡检是唯一入口，诊断包是巡检完成后的后续动作**。

## Goal

重新对齐诊断中心的架构，使其回归原始意图：
1. 巡检是唯一的主流程入口（手动 + 自动）
2. 诊断包生成是巡检完成后的按钮动作，不是独立 Tab
3. 错误中心保留在诊断中心（Tab 不变）
4. 自动巡检基于条件模板配置，支持 Java 严重错误关键字 + Prometheus 指标

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
- 当状态为 `completed` 且有 Findings 时，显示「生成诊断包」按钮
- 诊断包详情可在同页展开（Accordion）或子路由展示
  - 步骤进度（10步精简展示，异步轮询）
  - 节点执行状态（按节点展开）
  - 完成后：下载报告 / 预览 HTML 链接

### R3：诊断报告优化

- API 响应中补充 `cluster_name`（join 查询，不仅返回 cluster_id）
- Findings 列表默认按严重级别排序（critical first）
- `related_error_group_id` 不为零时，展示可点击链接跳转错误中心对应分组
- 报告 HTML 结构分三段：Summary → Critical Findings → 证据详情

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

- [ ] Workspace 只有 2个 Tab（errors / inspections），Task Tab 已移除
- [ ] 点击巡检记录进入详情页，展示 Findings（按严重级别排序）
- [ ] 详情页在 completed + 有 Findings 时出现「生成诊断包」按钮
- [ ] 诊断包执行步骤和节点状态在详情页可查看（异步刷新）
- [ ] API 响应包含 cluster_name
- [ ] Findings 中的 error_group 链接可跳转
- [ ] InspectionAutoPolicy CRUD API 实现
- [ ] 内置 11 个条件模板注册
- [ ] InspectionTriggerSource 支持 `auto`，新增 AutoTriggerReason 字段
- [ ] java_error 条件触发路径打通（agent 错误事件 → 策略检查 → 巡检创建）
- [ ] Prometheus 指标条件检查器框架搭建（至少 GC 和堆内存两个模板实现）
- [ ] 自动巡检配置 UI 可用（策略列表 + 编辑弹窗）

## Technical Notes

### 保留现有代码

- `ClusterInspectionReport` / `ClusterInspectionFinding` 数据模型保留
- `DiagnosticTask` / `DiagnosticTaskStep` / `DiagnosticNodeExecution` 后端模型保留（只从前端 Tab 隐藏）
- 错误中心（errors Tab）不动

### 实施顺序建议

1. 后端：InspectionAutoPolicy 模型 + CRUD API
2. 后端：内置模板注册 + java_error 触发检查器
3. 后端：Prometheus 指标检查器框架（接入已有 Prometheus client）
4. 后端：InspectionReport 新增 auto source + AutoTriggerReason
5. 前端：移除 Task Tab，调整 Workspace 为 2 Tab
6. 前端：巡检详情页（Findings + 生成诊断包入口）
7. 前端：自动巡检配置 UI

### Dev Type

fullstack
