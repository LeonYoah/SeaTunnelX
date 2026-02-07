# SeaTunnelX Agent 工作指南

本文档用于初始化本仓库的 AI/代码 Agent 协作规范，作为默认执行基线。

## 适用范围

- 默认作用域为整个仓库：`D:\ideaProject\SeaTunnelX`。
- `agent/` 目录是独立 Go 模块，负责运行时 Agent 能力。
- 除非 `.proto` 发生变更并需要重新生成，否则不要手改 protobuf 生成文件。

## 仓库结构速览

- `main.go`：后端启动入口。
- `frontend/`：Next.js 前端工程。
- `agent/`：SeaTunnelX Runtime Agent（独立 Go 模块）。
- `internal/`：后端核心业务逻辑。
- `scripts/`：`proto`、`tidy`、`swagger`、`license` 等脚本。
- `docs/`：项目文档。

## 常用命令

### 后端（根模块）

```bash
go mod tidy
go run main.go api
go test ./...
```

### 前端

```bash
cd frontend
pnpm install
pnpm dev
pnpm test
```

### Agent 模块

```bash
cd agent
go mod tidy
go run ./cmd
go test ./...
```

### Make 目标（根目录）

```bash
make tidy
make swagger
make proto
make check_license
make pre_commit
```

## 开发约定

- 变更保持最小化、聚焦当前任务。
- 延续现有架构和命名风格，避免无关重构。
- 行为变更必须同步补充或更新测试。
- 提交前至少运行与改动相关的测试与检查。
- 不提交本地二进制、临时文件和机器相关产物。

## Protobuf 变更说明

当修改 `.proto` 后，需要重新生成并确认以下文件更新：

- `internal/proto/agent/agent.pb.go`
- `internal/proto/agent/agent_grpc.pb.go`

并执行：

```bash
go test ./internal/proto/agent/...
```

## 提交前检查清单

- 代码可编译通过。
- 相关测试已通过。
- 如有行为变化，配置与文档已同步更新。
- 没有引入无关文件改动。
