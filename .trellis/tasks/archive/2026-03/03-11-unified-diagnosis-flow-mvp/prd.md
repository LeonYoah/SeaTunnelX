# 统一巡检与诊断流程 MVP

## Goal

将“巡检”和“诊断任务/诊断报告”在产品层统一为一个面向用户的“诊断”能力：用户先做轻量巡检，若发现异常，则在同一详情流中继续生成诊断包与诊断报告，而不是在多个页面与概念之间来回跳转。

## Requirements

### R1. 产品层统一为“诊断”流程

- 用户入口不再强调“巡检”和“诊断任务”是两个割裂功能；
- 巡检结果页应承担“诊断第 1 阶段”的角色；
- 发现异常后，应在当前页面直接给出后续动作，而不是仅给一个跳转链接。

### R2. 巡检完成后的异常后续动作清晰

- 若巡检无异常：明确告知“本次诊断未发现异常”；
- 若巡检有异常：在详情页显式展示：
  - 是否建议进一步采集现场
  - 可直接生成诊断包/诊断报告
  - 可只查看巡检结果暂不采集
- MVP 先支持“用户确认后生成诊断任务”，后续再扩展为自动策略。

### R3. 统一页面中的术语必须通俗易懂

- 用“诊断 / 继续排查 / 生成诊断报告 / 生成诊断包”等用户语言；
- 避免直接暴露“上下文 / 触发来源 / source ref”等系统术语；
- 页面应解释：
  - 巡检用于发现问题
  - 诊断包用于抓现场证据
  - 诊断报告用于汇总结论

### R4. MVP 优先复用现有后端能力

- 继续复用现有 inspection report、diagnostic task、diagnostic report/bundle 机制；
- 前端以“统一诊断动作入口”组织流程；
- 若需要默认从 inspection finding 创建任务，需显式选择或自动选择最严重 finding，并清楚提示用户。

### R5. 为后续自动诊断留扩展点

- 本阶段保留“巡检发现异常后询问用户是否继续”的交互；
- 为后续“自动生成诊断包/报告”预留状态位、文案和布局位置；
- 后续可扩展为诊断会话（diagnosis session）对象，但 MVP 不要求立刻新增持久化实体。

## Acceptance Criteria

- [ ] 巡检详情中出现统一的“继续诊断/生成报告”入口，而不是只有跳转到任务页
- [ ] 巡检无异常/有异常的后续动作表达清晰
- [ ] 页面用语对小白可理解，不再强调系统内部概念
- [ ] MVP 复用现有任务链路完成“从巡检结果继续生成诊断包/报告”
- [ ] 任务记录与页面文案为后续自动诊断预留扩展点

## Technical Notes

### 主要涉及文件

- `frontend/components/common/diagnostics/DiagnosticsInspectionCenter.tsx`
- `frontend/components/common/diagnostics/DiagnosticsTaskCenter.tsx`
- `frontend/components/common/diagnostics/DiagnosticsWorkspace.tsx`
- `frontend/lib/i18n/locales/zh.json`
- `frontend/lib/i18n/locales/en.json`
- 如需更顺滑的创建能力，可能涉及 `internal/apps/diagnostics/*`

### MVP 实施建议

1. 在巡检详情中新增“诊断下一步”区块；
2. 当存在 finding 时，展示：
   - 继续生成诊断报告
   - 查看巡检结果即可
   - 选择具体异常项继续排查
3. 先复用 inspection finding -> diagnostic task 的现有创建链路；
4. 后续阶段再把 inspection/task/report 统一为 diagnosis session。
