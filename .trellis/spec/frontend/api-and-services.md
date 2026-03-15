# API 与 Services

> 前端如何调用后端以及如何统一处理错误。

---

## 概述

所有后端请求通过 `lib/services/core/api-client.ts` 的 **axios** 客户端发出。客户端使用 `baseURL: '/api/v1'`、`withCredentials: true` 和 60 秒超时。响应格式为 **ApiResponse&lt;T&gt;**，包含 `error_msg` 与 `data`。`lib/services/core/base.service.ts` 中的 **BaseService** 封装 get/post/put/delete，成功时仅返回 `data`；错误由响应拦截器处理，以 `Error` 形式 reject，消息为后端返回或通用提示。

---

## API 响应结构

- **成功**：后端返回 `{ "error_msg": "", "data": <载荷> }`。Services 返回类型为 `T` 的 `data`。
- **错误**：后端返回 HTTP 4xx/5xx，通常带 `{ "error_msg": "消息" }`。拦截器用 `Error` reject，若存在 `error_msg` 则 `message` 为该值。

来自 `lib/services/core/types.ts`：

```ts
export interface ApiResponse<T = unknown> {
  error_msg: string;
  data: T;
}

export interface ApiError {
  error_msg: string;
}
```

---

## 服务层

- **BaseService**：抽象类，提供静态方法 `get<T>`、`post<T>`、`put<T>`、`delete<T>`，以及按需透传查询参数的 `postWithParams<T>`、`putWithParams<T>`。各方法调用 `apiClient.*`，期望 `ApiResponse<T>`，并返回 `response.data.data`。子类设置 `protected static basePath`，通过 `getFullPath(path)` 拼 URL。
- **约定**：
  - 每个领域一个 service 类（如 `ClusterService`、`HostService`）。
  - 方法对应后端接口（如 `list(params)`、`getById(id)`、`create(body)`）。
  - 返回类型为 `data` 的类型（如 `ClusterInfo`、`PaginatedResponse<ClusterInfo>`）；不向调用方暴露原始 `ApiResponse`。
  - 若后端来自 Go / GORM 等实现，**不要假定数组字段一定是 `[]`**；对于会被 UI 直接 `.map()` / `.filter()` 的字段，优先在 service 或 session 恢复边界统一做 normalize，把 `null` 兜底成 `[]`。
  - 若接口返回**后端本地化文案**（如 diagnostics 摘要、诊断报告、SSE 事件、模板说明），service 应显式附带当前语言参数 `lang`，避免后端回落到默认语言或双语拼接结果。
- **使用**：组件或 hooks 从 `lib/services/<domain>` 导入 service，调用静态方法；错误用 try/catch 捕获，消息在 UI 中展示（toast 或行内错误状态）。

---

## 错误处理

- **401**：拦截器重定向到 `/login`（或 `/login?redirect=...`），`/auth/login` 自身除外，以便登录失败能展示后端错误。
- **403**：以「权限不足」消息 reject。
- **5xx**：以「服务器内部错误，请稍后重试」reject。
- **超时 (ECONNABORTED)**：以「请求超时，请检查网络连接」reject。
- **后端 error_msg**：若存在 `error.response?.data?.error_msg`，用该消息 reject；否则使用通用提示。
- **在组件中**：对 service 调用包一层 try/catch；用 toast（如 sonner）或行内错误状态展示 `error.message`。不要向用户暴露堆栈或打日志。

---

## 新增一个 Service

1. 在 `lib/services/<domain>/` 下创建 `<domain>.service.ts`，可选 `types.ts` 和 `index.ts`。
2. 继承 `BaseService`，设置 `basePath` 为后端前缀（如 `'/clusters'`）。
3. 添加方法，调用 `this.get/post/put/delete` 并指定路径与返回类型 `T`（即 `data` 类型）。
4. 从 `lib/services/<domain>/index.ts` 导出，如需再从 `lib/services/index.ts` 导出。
5. 与后端契约一致：列表接口常返回 `{ data: { total, clusters } }` 等；类型 `T` 与之对应。

---

## 常见错误

- 在共享 `apiClient` 之外使用 `fetch` 或裸 axios（会绕过 401 重定向与错误统一处理）。
- 假定成功时一定存在 `data`；拦截器只透传响应，BaseService 假定为 `ApiResponse<T>` 并返回 `data` — 需保证后端 2xx 时始终返回 `data`。
- 基于会话的认证忘记 `withCredentials: true`；客户端与请求拦截器中已配置。
- 在调用处定义包含 `error_msg` 的响应类型；应把成功视为 `T`，错误视为抛出的 `Error`。
- 直接信任后端数组字段永远不是 `null`；尤其是 Go 的 nil slice 可能在 JSON 中变成 `null`，会让 UI 在 `.map()` / `.filter()` / `.length` 处崩溃。对这类字段要在边界统一 normalize。
