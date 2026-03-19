# 前端开发规范

> 本项目的前端开发最佳实践。

---

## 概述

本目录包含前端开发相关规范。控制台为 **Next.js** 应用（App Router），使用 **React**、**TypeScript**、**Tailwind CSS** 及 **shadcn/ui** 风格组件。API 访问通过 `lib/services/` 下的 **services** 统一完成，使用共享 **axios** 客户端与 **BaseService**；后端响应格式为 `{ error_msg, data }`。

---

## 规范索引

| 文档 | 说明 | 状态 |
|------|------|------|
| [目录结构](./directory-structure.md) | App Router、组件、lib、hooks | 已填写 |
| [API 与 Services](./api-and-services.md) | API 客户端、服务层、错误处理 | 已填写 |
| [UI 约定](./ui-conventions.md) | 布局、筛选栏、页面标题、i18n | 已填写 |

---

## 使用方式

- 遵循代码库中的**实际约定**（参见 `frontend/` 与 `agent.md`）。
- 新页面放在 `app/(main)/...` 下；新领域 UI 放在 `components/common/<domain>/`。
- 仅通过 `lib/services` 调用后端；使用统一的响应类型与错误处理。
- 需要生成**可部署前端产物**时，统一使用 `cd frontend && pnpm run pack:standalone`。
  - 原因：项目的 Docker / CI / PM2 发布链路依赖 `dist-standalone/` 产物，而不只是 `.next` 构建结果。
  - `pnpm build` 仅用于 Next.js 原始构建排查；**不要**把它当作本项目默认的前端交付构建命令。
- 需要**本地重启 / 发布前后端服务**时，优先使用仓库根目录 `./scripts/restart.sh`。
  - 原因：该脚本已包含后端构建、前端 `next build`、standalone 组装、PM2 重启与保存。
  - **不要**在执行 `./scripts/restart.sh` 之前再额外跑一次 `pnpm run pack:standalone`，除非你是在单独排查前端 standalone 构建问题。
- TypeScript 类型检查默认可复用增量缓存：`frontend/tsconfig.json` 已启用 `"incremental": true`。
  - 常规检查可直接运行：`cd frontend && pnpm exec tsc --noEmit`
  - **不要**在日常开发时主动删除 `frontend/tsconfig.tsbuildinfo`，否则会退回冷启动全量检查。
  - 2026-03-20 实测：冷启动约 **49.8s**，紧接着的增量复跑约 **9~12s**。
  - 仅在怀疑类型缓存脏了、结果异常时，才手动删除 `tsconfig.tsbuildinfo` 后重跑一次全量检查。

## 注释约定

- **自己新增或修改的前端代码注释，要求中英双语。** 适用于组件说明、复杂交互说明、状态管理说明、边界条件注释等。
- **默认顺序为“中文在前，英文在后”。** 两种语言应表达同一语义，不要写成两套不一致的说明。
- **第三方 / 生成 / 历史镜像代码不强制回填双语。** 但只要本次新增了注释，就应按双语写法补齐。
- **仍然遵循“少而准”的原则。** 不要为了满足双语要求去逐行翻译显而易见的代码。

## 中英文切换约定

- **面向用户的前端界面仍要求支持中英文切换。** 例如 diagnostics、巡检中心、诊断报告、任务中心等模块都应能随当前语言切换展示。
- **这里的“双语支持”指“可切换的单语言展示”**，不是同屏同时展示 `中文 / English`，也不是把两种语言拼在一条文案里。

---

**语言**：本目录下所有文档均使用**中文**。
