# 质量规范

> 后端开发的代码质量标准。

---

## 概述

项目使用 **Go**，采用 handler、service、repository 分层。代码按功能组织在 `internal/apps/<name>/` 下，命名与错误处理统一。测试使用 Go 自带 `testing` 包，部分模块在测试中使用真实 DB（如 SQLite）。代码风格与格式化遵循常见 Go 实践，与 `golangci-lint` 等工具一致。

---

## 注释、文件头与语言输出

### 注释语言

- **注释不再强制双语。** 新增注释按文件现有风格保持一致即可，重点是清晰、简洁、可维护。若同一文件已稳定采用中英双语注释，可继续保持；不要为了“形式上的双语”重复描述显而易见的代码。

### 注释密度

- **注释不宜过密。** 仅在有必要处添加注释：非常规逻辑、业务规则、复杂算法或对外 API 约定。避免逐行注释或重复代码已表达的含义。

### 新建文件须带 License 头

- **每个新建的后端源文件必须在文件顶部添加 Apache 2.0 License 头。** 紧接在 `package` 声明之前，`*/` 与 `package` 之间不要多空行。完整内容如下：

```
/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
```

### 用户可见语言输出

- **后端不应默认返回“中文 / English”拼接文案。** 面向 UI、HTML 报告、SSE 事件、任务日志等用户可见内容，应根据前端显式传入的 `lang`（如 `zh` / `en`）输出单语言结果。
- **语言切换由前端驱动。** 语言偏好来源于个人中心 / 用户资料，前端在请求 diagnostics 等需要本地化输出的接口时负责透传 `lang`，后端负责按该语言返回结果。
- **内部标识与用户文案分离。** 状态码、step code、template code 等内部字段保持稳定常量；展示名称、说明、推荐动作等用户文案再做本地化转换。

### 部署会话 Cookie 约定

- **私有化部署默认应将 `app.session_domain` 留空。** 这样浏览器会按当前访问 host 绑定 `seatunnel_session_id`，最适合 IP、内网域名和自定义域名混用场景。
- **不要在示例配置里把 `app.session_domain` 固定写成 `localhost`。** 若实际通过 `10.x.x.x`、`192.168.x.x` 或企业域名访问，但 Cookie domain 仍是 `localhost`，常见现象是：
  - `POST /api/v1/auth/login` 返回 200
  - 浏览器未正确回传会话 Cookie
  - 后续 `GET /api/v1/auth/user-info` 及其他受保护接口全部返回 `401`
- **只有在访问入口固定为某个域名时才显式设置 `app.session_domain`**，例如 `stx.company.com`。
- **`app.session_secure` 仅在浏览器始终通过 HTTPS 访问时设为 `true`。** 若仍通过 HTTP 调试或内网明文访问，设为 `true` 会导致浏览器不发送 Cookie。

#### Good / Base / Bad

- **Good**
  - `app.session_domain: ""`
  - 部署后通过 `http://10.0.0.5:8000` 或 `https://stx.company.com` 访问
  - 登录成功后 `/api/v1/auth/user-info` 返回 200
- **Base**
  - `app.session_domain: "stx.company.com"`
  - 确认所有用户都只通过该域名访问
- **Bad**
  - `app.session_domain: "localhost"`
  - 实际通过 IP / 企业域名访问
  - 现象：登录成功但后续接口持续 `401`

---

## 禁止模式

- **全局可变状态**：不得用全局变量持有 service 或 DB，应通过构造函数注入（如 `NewHandler(service, auditRepo)`）。仅允许的全局实例为 `internal/db` 的 DB 与 `internal/logger` 的 logger。
- **省略 context**：不得新增省略首参 `context.Context` 的 repository 或 service 方法，否则会破坏链路与取消传播。
- **在 API 中直接暴露 gorm.ErrRecordNotFound**：不得向 handler 返回 `gorm.ErrRecordNotFound`，应在 repository 中映射为领域错误（如 `ErrClusterNotFound`），再在 handler 中映射为 HTTP 状态码。
- **记录敏感信息**：不得在日志中输出密码、令牌、API 密钥或任何用于认证的配置，也不得记录可能包含凭据的请求/响应体。
- **忽略错误**：不得对影响正确性的错误使用 `_ = ...`，要么处理错误，要么在注释中说明为何可以忽略（如尽力而为的清理）。

---

## 必须遵循的模式

- **Context 传递**：从 HTTP handler 经 service 到 repository 一路传递 `ctx`；每次 DB 调用使用 `r.db.WithContext(ctx)`（或事务）。
- **领域失败用哨兵错误**：在 `err.go` 中定义包级哨兵错误；在 handler 中用 `getStatusCodeForError(err)`（或等价函数）返回一致的 HTTP 状态码。
- **响应结构**：统一使用 `ErrorMsg` 表示错误、`Data` 表示成功载荷，并显式设置 HTTP 状态码（400、404、409、500）。
- **新功能代码**：新功能放在 `internal/apps/<name>/` 下，按需包含 handler、service、repository、model、err；在 `internal/router/router.go` 中注册路由与依赖。
- **迁移**：新增持久化实体时，将对应 model 加入 `internal/db/migrator/migrator.go` 的 `AutoMigrate` 列表。

---

## 测试要求

- **单元测试**：使用同包下的 `*_test.go`。多场景优先用表驱动测试。在可行时 mock 外部依赖（如 DB）；部分测试使用真实 SQLite（如 `cluster/repository_test.go`、`audit/repository_test.go`）。
- **覆盖重点**：以 repository 和 service 逻辑为主；handler 常通过间接方式覆盖。关键路径（如重名、未找到）应有测试。
- **命名**：测试函数名为 `TestXxx`；子测试用 `t.Run("描述", ...)`。示例：`TestRepository_Create_clusterNameDuplicate_returnsErrClusterNameDuplicate`。
- **DB 测试**：使用真实 DB 时，避免依赖共享全局状态；尽量每个测试使用独立 DB 或事务（参见现有 `repository_test.go` 写法）。

---

## 代码评审检查项

- 新接口使用统一的请求/响应与错误模式（`ErrorMsg`、`Data`、`getStatusCodeForError`）。
- 所有新 DB 访问均使用 `WithContext(ctx)`。
- 新领域错误已加入 `err.go` 并在 handler 的状态码辅助函数中完成映射。
- 日志中无敏感信息与 PII。
- 新表/新 model 已加入 migrator 的 `AutoMigrate` 列表。
- 在可行处为新 repository 与 service 行为补充测试。
- 不为 service 或 repository 使用全局变量；依赖在 `router.go` 或 `cmd` 中组装。
