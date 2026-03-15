# 诊断任务来源上下文重构

## Goal

重构诊断任务中心的“来源上下文”交互，使 URL 上下文只承担预填作用，用户可以在界面中明确查看、切换、清除任务来源，避免长期被旧的 `错误组 #3` / `finding_id` / `alert_id` 上下文粘住。

## Requirements

### R1. URL 上下文只做预填，不做永久绑定

- 支持从错误中心、巡检中心、告警中心通过 URL 参数进入诊断任务页；
- 首次进入时可根据 `group_id` / `finding_id` / `alert_id` / `source` 预填来源；
- 用户进入任务页后，应允许主动切换为手动创建或其他来源，不应始终被旧 query 绑定。

### R2. 明确展示当前来源上下文

- 任务创建区需要显式展示当前“触发来源”；
- 需要展示来源摘要与 source ref（错误组、巡检报告/发现、告警等）；
- 提供“清除上下文”入口，清除后退回 `manual`。

### R3. 提供来源切换能力

- 增加来源模式切换：
  - 手动创建
  - 错误组触发
  - 巡检发现触发
  - 告警触发
- 当某类来源缺失必要上下文时，应阻止直接创建并给予明确提示；
- `cluster` 预过滤与 `trigger source` 选择应解耦。

### R4. 保持与现有后端模型兼容

- 继续复用后端既有 `manual / error_group / inspection_finding / alert` 四种来源；
- 不改变既有任务创建接口字段语义；
- 若后续扩展自动诊断来源，应为 UI 留出可扩展结构。

### R5. 文案与页面体验产品化

- 前端所有新增文案提供中英文 i18n；
- 明确区分“当前筛选上下文”与“任务触发来源”；
- 避免用户误以为系统只能围绕某个固定错误组创建任务。

## Acceptance Criteria

- [ ] 进入任务页时，URL 上下文仅用于预填，而不会长期锁死创建来源
- [ ] 页面可清楚显示当前触发来源与 source ref
- [ ] 用户可一键清除上下文并切换为手动创建
- [ ] 用户可在受支持来源之间切换，且校验逻辑明确
- [ ] 中英文文案补齐，页面不再长期出现误导性的“错误组 #3”粘连状态

## Technical Notes

### 主要涉及文件

- `frontend/components/common/diagnostics/DiagnosticsWorkspace.tsx`
- `frontend/components/common/diagnostics/DiagnosticsTaskCenter.tsx`
- `frontend/lib/i18n/locales/zh.json`
- `frontend/lib/i18n/locales/en.json`

### 已知现状

- 当前 `DiagnosticsWorkspace` 直接从 URL 读取 `group_id / report_id / finding_id / alert_id / source`；
- `DiagnosticsTaskCenter` 中 `buildCreateContext(...)` 会按 `finding -> group -> alert -> manual` 固定推导；
- tab 切换与页面停留过程中没有显式“来源切换器”，导致历史 query 在用户视角里像是“永久上下文”。

### 设计原则

- 保留 deep link 能力；
- 但 deep link 只负责“预定位”，真正的任务创建上下文以页面内状态为准；
- 后续自动诊断来源可沿用同一来源选择/展示框架扩展。
