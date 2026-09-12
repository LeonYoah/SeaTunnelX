# gRPC TLS 自动引导与 Agent CA 下发

> Control Plane **默认不开启** gRPC TLS。仅当 `grpc.tls_enabled=true` 时才准备证书；Agent 安装流在 CP 已开 TLS 时自动拉取 CA。

---

## Scenario: 安装流自动 TLS

### 1. Scope / Trigger

- Trigger: 新增/变更 gRPC TLS 启动引导、`GET /api/v1/agent/ca.crt`、Agent 安装脚本 TLS 字段、Agent `control_plane.tls` 校验
- 背景：Issue #23（明文 token）；本约定提供「显式开启 TLS 后自动生成证书 + 下发 CA」，不强制改 `RequireTransportSecurity`，**默认保持明文**

### 2. Signatures

- `tlsbootstrap.EnsureGRPCTLS(opts Options) (Result, error)` — 在 `initGRPCServer` 之前调用
- `tlsbootstrap.AgentCAPath() string` — 供安装 handler 注入 CA 路径
- `GET /api/v1/agent/ca.crt` — 下载 PEM CA（`DownloadCA`）
- Agent 配置：`control_plane.tls.enabled` / `ca_file`（必需）/ `cert_file`+`key_file`（可选 mTLS）

### 3. Contracts

**证书落盘（启用 TLS 且自动生成时）**

```
{storage.base_dir}/certs/
  ca.crt      # 可下发给 Agent；默认有效期约 99 年（36135 天）
  ca.key      # 仅 CP 本地，不下发
  server.crt  # 默认有效期约 99 年（36135 天）
  server.key
```

**服务端内存配置（单向 TLS）**

- 默认：`grpc.tls_enabled=false`（不生成、不强制开启）
- 显式开启后：`grpc.tls_enabled=true`，`cert_file`/`key_file` 指向 server 证书
- `grpc.ca_file=""`（留空，避免误开 mTLS ClientAuth）

**安装脚本环境**

- `GRPC_TLS_ENABLED=true|false`
- `AGENT_CA_FILE` 默认 `/etc/seatunnelx-agent/certs/ca.crt`
- TLS 开时：`download_ca` → 写 Agent 配置 `tls.enabled=true` + `ca_file`

**Docker**

- `Dockerfile.backend` / `Dockerfile.all-in-one` 预装 `openssl`

### 4. Validation & Error Matrix

| 条件 | 行为 |
|------|------|
| `grpc.tls_enabled=false`（默认） | **不生成、不开启**，即使磁盘已有证书 |
| `tls_enabled=true`，默认目录证书齐全 | 启用 TLS，**不覆盖**文件 |
| `tls_enabled=true`，目录有不完整残留 | **拒绝覆盖**，TLS 关闭，日志提示补齐或清空 |
| `tls_enabled=true`，有 openssl，目录空 | 生成 CA+server，启用 TLS |
| `tls_enabled=true`，无 openssl，且无完整证书 | TLS 强制关闭，打日志 |
| `GET .../ca.crt` 且 CP TLS 关 | 404 |
| Agent `tls.enabled` 且无 `ca_file` | `Validate` 失败 |
| Agent 仅配 `cert_file` 或仅配 `key_file` | `Validate` 失败（mTLS 必须成对） |

### 5. Good/Base/Bad Cases

- Good: `tls_enabled=true` + 本机有 openssl，首次启动生成证书；`install.sh` 自动带 CA 并开 Agent TLS
- Base: 默认 `tls_enabled=false`，明文 gRPC，不生成证书；E2E supervisor 遇 CA 404 保持明文 Agent
- Bad: 只有 `server.crt` 无 key；或 Agent 开 TLS 却不配 `ca_file`

### 6. Tests Required

- `internal/tlsbootstrap`：默认关闭不生成 / 无 openssl / 已有证书不覆盖 / 真实 openssl 生成
- `internal/apps/agent`：`DownloadCA` 404/200；安装脚本含 `GRPC_TLS_ENABLED` 与 `download_ca`
- `agent/internal/config`：单向 TLS 仅需 `ca_file`；cert/key 成对校验
- E2E real installer：`real-agent-supervisor.mjs` 在 backend healthy 后拉取 `/api/v1/agent/ca.crt`；TLS 关则 404 保持明文 Agent 配置

### 7. Wrong vs Correct

#### Wrong

```go
// 服务端把 CA 配进 grpc.ca_file，导致误开 mTLS
config.Config.GRPC.CAFile = caCertPath
```

```yaml
# Agent 开 TLS 却要求客户端证书（单向 TLS 场景）
tls:
  enabled: true
  cert_file: /path/client.crt
  key_file: /path/client.key
  # 缺少 ca_file
```

```js
// E2E 在 CP 已自动开 TLS 后，仍用 plaintext Agent 配置启动
spawn(goBin, ['run', './cmd', '--config', agentConfigPath])
```

#### Correct

```go
// 单向 TLS：服务端 ca_file 留空；CA 仅通过 AgentCAPath / DownloadCA 下发
applyGRPCConfig(true, serverCert, serverKey, "")
```

```yaml
# Agent 单向 TLS
tls:
  enabled: true
  ca_file: /etc/seatunnelx-agent/certs/ca.crt
```

```js
// E2E：先拉 CA 再启动 Agent
const effective = await resolveAgentConfigWithTLS(agentConfigPath)
spawn(goBin, ['run', './cmd', '--config', effective])
```
---

## Design Decision: 用 openssl CLI 而非纯 Go 生成

**Context**：需要在安装/启动时自签 CA + server，且 Docker/本机常见已有 openssl。

**Decision**：`exec` 调用 openssl；无 openssl 则不强行开 TLS。证书生成与运行时 TLS（Go `crypto/tls`）解耦——运行不依赖 openssl，仅引导生成依赖。

**Out of scope**：mTLS 强制、证书自动轮换、HTTP API 全面 HTTPS。
