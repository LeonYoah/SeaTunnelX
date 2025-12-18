# SeaTunnel 一站式运维管理平台

🚀 Apache SeaTunnel 数据集成平台的运维管理工具

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.24-blue.svg)](https://golang.org/)
[![Next.js](https://img.shields.io/badge/Next.js-15-black.svg)](https://nextjs.org/)
[![React](https://img.shields.io/badge/React-19-blue.svg)](https://reactjs.org/)

## 📖 项目简介

SeaTunnel 一站式运维管理平台是为 Apache SeaTunnel 数据集成引擎打造的运维管理工具，提供任务管理、监控告警、资源调度等功能。

> 本项目基于 [linux-do/cdk](https://github.com/linux-do/cdk) 项目改造，原项目采用 MIT 协议开源。

### ✨ 主要特性

- 🔐 **多种认证方式** - 支持用户名密码登录和 OAuth（GitHub、Google）登录
- 🗄️ **多数据库支持** - 支持 SQLite（默认）、MySQL、PostgreSQL
- 🌍 **国际化支持** - 内置中英文切换
- ⚡ **轻量化部署** - Redis 可选，支持内存会话存储
- 🎨 **现代化界面** - 基于 Next.js 15 和 React 19 的响应式设计

## 🏗️ 架构概览

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Frontend      │    │    Backend      │    │   Database      │
│   (Next.js)     │◄──►│     (Go)        │◄──►│ (SQLite/MySQL)  │
│                 │    │                 │    │                 │
│ • React 19      │    │ • Gin Framework │    │ • SQLite 默认   │
│ • TypeScript    │    │ • GORM          │    │ • MySQL 可选    │
│ • Tailwind CSS  │    │ • OpenTelemetry │    │ • PostgreSQL    │
│ • Shadcn UI     │    │ • Swagger API   │    │ • Redis 可选    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 🛠️ 技术栈

### 后端
- **Go 1.24** - 主要开发语言
- **Gin** - Web 框架
- **GORM** - ORM 框架
- **SQLite/MySQL/PostgreSQL** - 数据库
- **Redis** - 缓存和会话存储（可选）

### 前端
- **Next.js 15** - React 框架
- **React 19** - UI 库
- **TypeScript** - 类型安全
- **Tailwind CSS 4** - 样式框架
- **Shadcn UI** - 组件库

## 📋 环境要求

- **Go** >= 1.24
- **Node.js** >= 18.0
- **pnpm** >= 8.0 (推荐)

## 🚀 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/LeonYoah/SeaTunnelX.git
cd SeaTunnelX
```

### 2. 配置环境

```bash
cp config.example.yaml config.yaml
```

默认配置使用 SQLite 数据库，无需额外配置即可启动。

### 3. 启动后端

```bash
go mod tidy
go run main.go api
```

### 4. 启动前端

```bash
cd frontend
pnpm install
pnpm dev
```

### 5. 访问应用

- **前端界面**: http://localhost:3000
- **默认账号**: admin / admin123

## ⚙️ 配置说明

### 主要配置项

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `auth.default_admin_username` | 默认管理员用户名 | `admin` |
| `auth.default_admin_password` | 默认管理员密码 | `admin123` |
| `database.type` | 数据库类型 | `sqlite` |
| `redis.enabled` | 是否启用 Redis | `false` |

### 🔐 OAuth 登录配置（可选）

平台支持 GitHub 和 Google OAuth 登录作为备选登录方式。

#### 获取 GitHub OAuth 凭证

1. 登录 GitHub，访问 [Developer Settings](https://github.com/settings/developers)
2. 点击 **"New OAuth App"**
3. 填写应用信息：
   - **Application name**: `SeaTunnel Platform`
   - **Homepage URL**: `http://localhost:3000`
   - **Authorization callback URL**: `http://localhost:3000/callback`
4. 创建后获取 **Client ID** 和 **Client Secret**

> 📖 详细教程：[GitHub OAuth2 配置指南](https://apifox.com/apiskills/how-to-use-github-oauth2/)

#### 获取 Google OAuth 凭证

1. 访问 [Google Cloud Console](https://console.cloud.google.com/)
2. APIs & Services → Credentials → Create Credentials → OAuth client ID
3. 添加 Authorized redirect URIs: `http://localhost:3000/callback`

> 📖 详细教程：[Google OAuth2 配置指南](https://apifox.com/apiskills/how-to-use-google-oauth2/)

#### 配置 OAuth 凭证

```yaml
oauth_providers:
  github:
    enabled: true
    client_id: "你的 GitHub Client ID"
    client_secret: "你的 GitHub Client Secret"
    redirect_uri: "http://localhost:3000/callback"
  google:
    enabled: true
    client_id: "你的 Google Client ID"
    client_secret: "你的 Google Client Secret"
    redirect_uri: "http://localhost:3000/callback"
```

## 🧪 测试

```bash
# 后端测试
go test ./...

# 前端测试
cd frontend && pnpm test
```

## 🔧 二次开发指南

### Protocol Buffers 代码生成

本项目使用 gRPC 进行 Agent 与 Control Plane 之间的通信。如果修改了 `.proto` 文件，需要重新生成 Go 代码。

#### 前置条件

1. **安装 Go protoc 插件**（Linux/macOS/Windows 通用）

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

#### Linux / macOS

1. **安装 protoc 编译器**

```bash
# macOS
brew install protobuf

# Ubuntu/Debian
sudo apt-get install protobuf-compiler

# CentOS/RHEL
sudo yum install protobuf-compiler
```

2. **生成代码**

```bash
# 使用脚本（推荐）
./scripts/proto.sh

# 或手动执行
protoc --proto_path=. \
    --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    internal/proto/agent/agent.proto
```

#### Windows (PowerShell)

1. **下载并安装 protoc 编译器**

```powershell
# 一键下载并配置 protoc（临时安装到 TEMP 目录）
$protocVersion = "28.3"
$protocZip = "protoc-$protocVersion-win64.zip"
$protocUrl = "https://github.com/protocolbuffers/protobuf/releases/download/v$protocVersion/$protocZip"
$protocDir = "$env:TEMP\protoc"

if (!(Test-Path $protocDir)) { 
    New-Item -ItemType Directory -Path $protocDir -Force 
}
Invoke-WebRequest -Uri $protocUrl -OutFile "$protocDir\$protocZip"
Expand-Archive -Path "$protocDir\$protocZip" -DestinationPath $protocDir -Force
$env:PATH = "$protocDir\bin;$env:PATH"

# 验证安装
protoc --version
```

2. **生成代码**

```powershell
# 设置环境变量（每次新开 PowerShell 需要执行）
$protocDir = "$env:TEMP\protoc"
$env:PATH = "$protocDir\bin;$env:USERPROFILE\go\bin;$env:PATH"

# 生成 protobuf 代码
protoc --proto_path=. `
    --go_out=. --go_opt=paths=source_relative `
    --go-grpc_out=. --go-grpc_opt=paths=source_relative `
    internal/proto/agent/agent.proto
```

> 💡 **提示**: Windows 用户也可以选择永久安装 protoc：
> - 从 [GitHub Releases](https://github.com/protocolbuffers/protobuf/releases) 下载对应版本
> - 解压到固定目录（如 `C:\protoc`）
> - 将 `C:\protoc\bin` 添加到系统 PATH 环境变量

#### 验证生成结果

生成成功后，以下文件会被更新：
- `internal/proto/agent/agent.pb.go` - Protobuf 消息定义
- `internal/proto/agent/agent_grpc.pb.go` - gRPC 服务定义

```bash
# 运行测试验证生成的代码
go test ./internal/proto/agent/...
```

### Agent 打包

Agent 是部署在目标主机上的守护进程，需要交叉编译为 Linux 二进制文件。

#### Linux / macOS

```bash
cd agent

# 打包 Linux amd64
GOOS=linux GOARCH=amd64 go build -o seatunnelx-agent ./cmd/main.go

# 打包 Linux arm64
GOOS=linux GOARCH=arm64 go build -o seatunnelx-agent-arm64 ./cmd/main.go
```

#### Windows (PowerShell)

```powershell
cd agent

# 打包 Linux amd64
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o seatunnelx-agent ./cmd/main.go

# 打包 Linux arm64
$env:GOOS="linux"; $env:GOARCH="arm64"; go build -o seatunnelx-agent-arm64 ./cmd/main.go

# 恢复环境变量（可选）
Remove-Item Env:GOOS; Remove-Item Env:GOARCH
```

#### 部署 Agent 二进制

打包完成后，将 `seatunnelx-agent` 复制到 `lib/agent/` 目录：

```bash
# Linux/macOS
cp agent/seatunnelx-agent lib/agent/seatunnelx-agent-linux-amd64
cp agent/seatunnelx-agent-arm64 lib/agent/seatunnelx-agent-linux-arm64

# Windows PowerShell
Copy-Item agent/seatunnelx-agent lib/agent/seatunnelx-agent-linux-amd64
Copy-Item agent/seatunnelx-agent-arm64 lib/agent/seatunnelx-agent-linux-arm64

# Windows PowerShell 一键操作

cd agent; $env:GOOS="linux"; $env:GOARCH="amd64"; go build -o seatunnelx-agent ./cmd/main.go; cd ..; Copy-Item agent/seatunnelx-agent lib/agent/seatunnelx-agent-linux-amd64 -Force

```

## 🚀 部署

### Docker 部署

```bash
docker build -t seatunnel-platform .
docker run -d -p 8000:8000 seatunnel-platform
```

## 📄 许可证

本项目基于 [Apache License 2.0](LICENSE) 开源。

## 🔗 相关链接

- [Apache SeaTunnel](https://seatunnel.apache.org/)
- [SeaTunnelX GitHub](https://github.com/LeonYoah/SeaTunnelX)
- [原项目 linux-do/cdk](https://github.com/linux-do/cdk)
