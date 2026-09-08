# 安装流自动生成 TLS 证书并默认开启

## Goal

Control Plane 安装/启动时检测本机 `openssl`：有则自动生成 CA/服务端证书并默认开启 gRPC TLS；无则保持 TLS 关闭。Agent 一键安装自动拉取 CA。Docker 镜像预装 `openssl`。用户可用自有证书替换后重启。

## Requirements

- 启动/安装引导时检测 `openssl` 命令是否可用
- 有 openssl 且尚无证书时：自动生成 CA + server 证书，写入约定目录，并将 `grpc.tls_enabled` 设为 true，填好 `cert_file`/`key_file`（单向 TLS，服务端 `ca_file` 留空以免误开 mTLS）
- 证书已存在：不覆盖，沿用现有文件；日志提示可用自有证书替换后重启
- 无 openssl：不开启 TLS，明确日志说明原因
- Docker（backend / all-in-one）安装 `openssl`
- Agent 安装脚本：若 Control Plane 已开 TLS，下载 CA 到本机并写入 `control_plane.tls.enabled=true` + `ca_file`
- Agent 配置校验：单向 TLS 仅要求 `ca_file`（`cert_file`/`key_file` 可选，留给 mTLS）
- 提供 CA 下载接口（如 `GET /api/v1/agent/ca.crt`）供安装流使用

## Acceptance Criteria

- [x] 本机有 openssl、无现成证书时：启动后 gRPC TLS 开启，证书落盘
- [x] 本机无 openssl：TLS 保持关闭，有清晰日志
- [x] 已有证书文件：不覆盖，可替换后重启生效
- [x] Docker backend / all-in-one 镜像含 openssl，容器内可自动生成
- [x] `curl .../install.sh | bash` 在 CP 已开 TLS 时自动带上 CA 并启用 Agent TLS
- [x] Agent 仅配 `ca_file` 即可通过 Validate 并成功拨号（单向 TLS）
- [x] 相关单测通过

## Technical Notes

- 证书默认目录建议：`{data_dir 或工作目录}/certs/`（`ca.crt` / `server.crt` / `server.key`）
- SAN：至少包含 `localhost`、`127.0.0.1`，以及 `app.external_url` 解析出的 host
- 分支：`feat/install-auto-tls-openssl`
- Issue 背景：#23 明文 token；本任务先解决「可默认开 TLS + 安装下发 CA」
