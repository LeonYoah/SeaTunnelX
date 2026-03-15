# 巡检中心中英文自由切换改造

## Goal

将 diagnostics 全域语言策略从“后端双语并排输出 + 浏览器端局部切换”收敛为“个人中心维护用户语言偏好，前端按当前 locale 显式透传 lang，后端按单语言输出”，从而让巡检中心、诊断任务、错误中心与离线诊断报告都支持统一的中英文自由切换，并同步更新项目规范，明确后端不再默认输出双语拼接内容。

## What I already know

- 前端已有 `frontend/lib/i18n/provider.tsx`，当前 locale 只依赖 `localStorage` + 浏览器语言。
- 个人中心入口在 `frontend/components/common/layout/ManagementBar.tsx` 的 `ProfileButton`，当前仅支持邮箱修改。
- 后端 `auth/profile` 目前只保存邮箱，`internal/apps/auth/handler.go` 与前端 `services.auth.updateProfile` 均未包含语言字段。
- diagnostics 后端当前大量使用 `internal/apps/diagnostics/display_text.go` 中的 `bilingualText(...)`，以及 `task_execute.go` 中 HTML 语言切换按钮与浏览器语言检测逻辑。
- diagnostics 前端当前混合使用 i18n key 与双语/硬编码展示，且下载/预览 HTML URL 尚未显式透传语言参数。
- 后端规范 `.trellis/spec/backend/quality-guidelines.md` 仍写着“注释须中英双语”，与本次语言策略调整不一致。

## Assumptions (temporary)

- 用户语言偏好字段放在 auth user profile 内，允许值为 `zh` / `en`。
- 前端 diagnostics 所有依赖后端生成文本的接口与 HTML 下载/预览链接都可以安全增加 `lang` 参数。
- diagnostics 领域本次优先切换为“单语言输出”，非 diagnostics 其他模块暂不做同等级切换。

## Open Questions

- 无。范围与持久化方式已确认：diagnostics 全域，语言偏好落后端 profile。

## Requirements (evolving)

- 在个人中心中提供语言切换入口，并将用户语言偏好保存到后端 profile。
- 前端 i18n 初始化优先读取用户 profile.language，回退到本地缓存与浏览器语言。
- diagnostics 全域前端请求与 HTML 预览/下载链接显式传递 `lang`。
- 后端 diagnostics 按 `lang` 生成单语言文本，不再默认拼接“中文 / English”。
- 巡检中心、巡检详情、诊断任务、错误中心、离线 HTML 报告的语言体验保持一致。
- 更新前后端规范，明确项目目标是“中英文自由切换”，而非“默认双语并排设计”。

## Acceptance Criteria (evolving)

- [ ] 用户可在个人中心查看并修改语言偏好（中文 / English）。
- [ ] `GET /api/v1/auth/user-info` 与 `PUT /api/v1/auth/profile` 返回/保存 language 字段。
- [ ] 刷新页面或跨设备登录后，前端会基于用户 profile 恢复语言。
- [ ] diagnostics 相关请求与 HTML 预览/下载链接显式携带 `lang`，不再依赖浏览器猜测。
- [ ] diagnostics 后端输出改为单语言，移除默认双语并排内容与 HTML 内置语言切换器。
- [ ] 巡检中心、错误中心、诊断任务中心与离线报告在 zh/en 下都能正常显示。
- [ ] `.trellis/spec/backend/` 与 `.trellis/spec/frontend/` 中相关语言策略已更新。

## Definition of Done (team quality bar)

- 前后端相关测试补齐或更新。
- 相关 lint / typecheck / Go test 通过。
- 代码与规范同步更新，无残留默认双语并排设计。

## Out of Scope (explicit)

- 非 diagnostics 领域的所有后端业务文本全量切换。
- 新增第三种语言或完整国际化后台管理系统。
- 修改现有 next-intl 基础设施到服务端路由级 locale 分段。

## Technical Notes

- 预计涉及文件：
  - `frontend/components/common/layout/ManagementBar.tsx`
  - `frontend/lib/i18n/provider.tsx`
  - `frontend/lib/i18n/config.ts`
  - `frontend/hooks/use-auth.ts`
  - `frontend/lib/services/auth/*`
  - `frontend/lib/services/diagnostics/*`
  - `frontend/components/common/diagnostics/*`
  - `internal/apps/auth/*`
  - `internal/apps/diagnostics/display_text.go`
  - `internal/apps/diagnostics/task_execute.go`
  - `internal/apps/diagnostics/task_service.go`
  - `internal/apps/diagnostics/inspection_*`
  - `.trellis/spec/backend/index.md`
  - `.trellis/spec/backend/quality-guidelines.md`
  - `.trellis/spec/frontend/ui-conventions.md`
- 关键跨层数据流：
  - Profile.language（DB）→ auth API → useAuth cache → I18nProvider locale → diagnostics service `lang` → diagnostics handler/service → HTML/report/render text。
