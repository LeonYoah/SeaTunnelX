# 目录结构

> 本项目中后端代码的组织方式。

---

## 概述

后端为 Go 项目，代码位于 `internal/` 下。采用**按应用分模块**的布局：每个功能（主机、集群、审计、监控等）对应 `internal/apps/<name>/` 下的一个「应用」目录，handler、service、repository、model、错误定义分离清晰。路由与依赖组装集中在 `internal/router/router.go`。

---

## 目录布局

```
internal/
├── apps/                    # 功能模块（按领域分目录）
│   ├── admin/               # 管理员与用户管理
│   ├── agent/               # Agent 安装脚本、handler、管理器
│   ├── audit/               # 命令日志、审计日志
│   ├── auth/                # 登录、用户信息、会话
│   ├── cluster/             # 集群与节点 CRUD、操作
│   ├── config/              # 配置文件管理（模板/节点）
│   ├── dashboard/           # 仪表盘概览与统计
│   ├── deepwiki/            # 文档服务
│   ├── discovery/           # 从主机发现集群
│   ├── health/              # 健康检查
│   ├── host/                # 主机 CRUD、安装命令
│   ├── installer/           # 包管理、安装向导
│   ├── monitor/             # 监控配置、进程事件
│   ├── monitoring/          # 监控中心（告警、Grafana 代理等）
│   ├── oauth/               # OAuth 提供商（GitHub、Google）
│   ├── plugin/              # 插件市场、安装到集群
│   └── task/                # 任务管理
├── cmd/                     # 入口（root、api、worker、scheduler）
├── config/                  # 全局配置加载与校验
├── db/                      # 数据库初始化、迁移（GORM）
├── grpc/                    # gRPC 服务端与 handler（与 Agent 通信）
├── logger/                  # 基于 Zap 的日志（otelzap、trace ID）
├── otel_trace/              # OpenTelemetry 追踪
├── proto/                   # Protobuf 定义与生成代码（Agent）
├── router/                  # Gin 路由、中间件（日志、会话）
├── session/                 # 会话存储（默认内存实现）
├── task/                    # legacy scheduler/worker 壳入口
├── tlsbootstrap/            # gRPC TLS 自动引导（openssl 检测与证书生成）
└── utils/                   # 共享工具（http_client、自定义类型等）
```

---

## 模块组织

- **按应用划分**：`internal/apps/<name>/` 下通常包含：
  - `handler.go` — HTTP handler（Gin）、请求/响应类型、`getStatusCodeForError`
  - `service.go` — 业务逻辑，调用 repository 及其他 service
  - `repository.go` — 数据访问（GORM），所有查询使用 `WithContext(ctx)`
  - `model.go` — 领域结构体、常量、GORM model 标签
  - `err.go` — 哨兵错误（`errors.New`）及可选错误码
  - 可选：`routers.go`、`middlewares.go`、`types.go`、`*_test.go`
- **路由**：所有 API 路由在 `internal/router/router.go` 中注册；handler 与 service 在该处构造（如 `hostRepo := host.NewRepository(db.DB(context.Background()))`）；跨应用依赖通过 `router.go` 中定义的适配器注入（如 `hostStatusUpdaterAdapter`、`agentCommandSenderAdapter`）。
- **无全局应用状态**：repository 与 service 通过构造函数接收 `*gorm.DB` 或接口；数据库在 router 中通过 `db.GetDB(ctx)` 或 `db.DB(ctx)` 获取。

---

## 命名约定

- **包名**：一个目录对应一个包名，包名与目录一致（如 `cluster`、`host`、`audit`）。
- **文件**：多词文件用 snake_case（`repository.go`、`error_handling.go`）。实体放在 `model.go`，错误放在 `err.go`。
- **Handler**：`NewHandler(service, ...)`，方法如 `CreateCluster`、`ListClusters`；请求/响应类型同文件（如 `CreateClusterResponse`）。
- **Repository**：`NewRepository(db *gorm.DB)`，方法如 `Create`、`GetByID`、`List`、`Update`、`Delete`；首参均为 `ctx context.Context`。
- **错误**：包级哨兵错误在 `err.go` 中定义（如 `ErrClusterNotFound`）；需要 HTTP 映射时可定义错误码常量（如 `ErrCodeClusterNotFound`）。

---

## 示例

- **结构清晰的应用**：`internal/apps/cluster/` — 清晰的 handler → service → repository，`err.go` 中哨兵错误，`model.go` 中状态/角色常量与 GORM model。
- **路由与依赖组装**：`internal/router/router.go` — 如何创建 repo/service，以及适配器如何将 agent manager、host service 等注入其他应用。
- **DB 访问**：`internal/db/database.go` — `GetDB(ctx)`、`GetGlobalDB()`；迁移在 `internal/db/migrator/migrator.go`，通过 `AutoMigrate` 注册所有应用 model。
