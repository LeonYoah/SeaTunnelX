# 优化诊断报告相关证据加载性能

## Goal

降低诊断报告 HTML 在打开“相关证据”时的卡顿，避免大体积原始日志内容拖慢整页交互与标签切换。

## What I already know

- 用户反馈：不点“相关证据”时页面流畅，点击后明显卡顿。
- 当前离线 HTML 报告会把 `ErrorContext.LogSamples[].Content` 直接内联到 `<pre>` 中。
- `collectDiagnosticWindowedLogSnippet(..., maxLines int)` 已接收 `maxLines`，但当前实现没有使用这个参数。
- DiagnosticsTaskCenter 的任务日志列表已有分页/过滤，但离线 HTML 报告没有按需加载或分页。

## Requirements (evolving)

- 相关证据页默认不要一次性渲染超大原始日志块。
- 保留用户查看原始日志的能力。
- 优化后不能破坏离线报告可分享能力。

## Acceptance Criteria (evolving)

- [ ] 打开“相关证据”时不再因大日志块导致明显卡顿。
- [ ] 原始日志仍可查看或下载。
- [ ] 后端生成报告时对日志样本大小有明确上限。

## Out of Scope (explicit)

- 不重做整套 diagnostics 报告结构。
- 不引入需要在线 API 才能查看离线报告的强依赖方案。

## Technical Notes

- 相关文件：`internal/apps/diagnostics/task_execute.go`
- 当前 evidence 页包含完整错误事件表、原始日志、配置预览等内容。
- 可能需要同时做“生成端截断”和“展示端按需展开”两层优化。
