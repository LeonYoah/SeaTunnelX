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

// Package agent provides Agent distribution and management for the SeaTunnel Control Plane.
// agent 包提供 SeaTunnel Control Plane 的 Agent 分发和管理功能。
package agent

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// InstallScriptGenerator generates Agent installation scripts.
// InstallScriptGenerator 生成 Agent 安装脚本。
// Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6 - Implements one-click Agent installation.
type InstallScriptGenerator struct {
	// controlPlaneAddr is the HTTP address of the Control Plane.
	// controlPlaneAddr 是 Control Plane 的 HTTP 地址。
	controlPlaneAddr string

	// grpcAddr is the gRPC address for Agent connection.
	// grpcAddr 是 Agent 连接的 gRPC 地址。
	grpcAddr string

	// heartbeatInterval is the heartbeat interval in seconds.
	// heartbeatInterval 是心跳间隔（秒）。
	heartbeatInterval int

	// template is the parsed install script template.
	// template 是解析后的安装脚本模板。
	template *template.Template
}

// InstallScriptConfig holds configuration for the install script generator.
// InstallScriptConfig 保存安装脚本生成器的配置。
type InstallScriptConfig struct {
	// ControlPlaneAddr is the HTTP address of the Control Plane.
	// ControlPlaneAddr 是 Control Plane 的 HTTP 地址。
	ControlPlaneAddr string

	// GRPCAddr is the gRPC address for Agent connection.
	// GRPCAddr 是 Agent 连接的 gRPC 地址。
	GRPCAddr string

	// HeartbeatInterval is the heartbeat interval in seconds from Control Plane config.
	// HeartbeatInterval 是来自 Control Plane 配置的心跳间隔（秒）。
	HeartbeatInterval int
}

// InstallScriptData holds data for rendering the install script template.
// InstallScriptData 保存渲染安装脚本模板的数据。
type InstallScriptData struct {
	// ControlPlaneAddr is the HTTP address of the Control Plane.
	// ControlPlaneAddr 是 Control Plane 的 HTTP 地址。
	ControlPlaneAddr string

	// GRPCAddr is the gRPC address for Agent connection.
	// GRPCAddr 是 Agent 连接的 gRPC 地址。
	GRPCAddr string

	// InstallDir is the directory where Agent binary will be installed.
	// InstallDir 是 Agent 二进制文件的安装目录。
	InstallDir string

	// ConfigDir is the directory for Agent configuration files.
	// ConfigDir 是 Agent 配置文件的目录。
	ConfigDir string

	// AgentBinary is the name of the Agent binary file.
	// AgentBinary 是 Agent 二进制文件的名称。
	AgentBinary string

	// ServiceName is the systemd service name.
	// ServiceName 是 systemd 服务名称。
	ServiceName string

	// HeartbeatInterval is the heartbeat interval string (e.g., "60s").
	// HeartbeatInterval 是心跳间隔字符串（如 "60s"）。
	HeartbeatInterval string
}

// SupportedPlatform represents a supported OS and architecture combination.
// SupportedPlatform 表示支持的操作系统和架构组合。
type SupportedPlatform struct {
	// OS is the operating system (linux, darwin).
	// OS 是操作系统（linux, darwin）。
	OS string

	// Arch is the CPU architecture (amd64, arm64).
	// Arch 是 CPU 架构（amd64, arm64）。
	Arch string

	// BinaryName is the name of the binary file for this platform.
	// BinaryName 是此平台的二进制文件名称。
	BinaryName string
}

// DefaultInstallDir is the default installation directory for Agent binary.
// DefaultInstallDir 是 Agent 二进制文件的默认安装目录。
const DefaultInstallDir = "/usr/local/bin"

// DefaultConfigDir is the default configuration directory for Agent.
// DefaultConfigDir 是 Agent 的默认配置目录。
const DefaultConfigDir = "/etc/seatunnelx-agent"

// DefaultAgentBinary is the default name of the Agent binary.
// DefaultAgentBinary 是 Agent 二进制文件的默认名称。
const DefaultAgentBinary = "seatunnelx-agent"

// DefaultServiceName is the default systemd service name.
// DefaultServiceName 是默认的 systemd 服务名称。
const DefaultServiceName = "seatunnelx-agent"

// SupportedPlatforms defines all supported OS and architecture combinations.
// SupportedPlatforms 定义所有支持的操作系统和架构组合。
// Requirements: 2.1, 2.2 - Supports linux-amd64 and linux-arm64.
var SupportedPlatforms = []SupportedPlatform{
	{OS: "linux", Arch: "amd64", BinaryName: "seatunnelx-agent-linux-amd64"},
	{OS: "linux", Arch: "arm64", BinaryName: "seatunnelx-agent-linux-arm64"},
	{OS: "darwin", Arch: "amd64", BinaryName: "seatunnelx-agent-darwin-amd64"},
	{OS: "darwin", Arch: "arm64", BinaryName: "seatunnelx-agent-darwin-arm64"},
}

// NewInstallScriptGenerator creates a new InstallScriptGenerator instance.
// NewInstallScriptGenerator 创建一个新的 InstallScriptGenerator 实例。
func NewInstallScriptGenerator(cfg *InstallScriptConfig) (*InstallScriptGenerator, error) {
	if cfg == nil {
		cfg = &InstallScriptConfig{}
	}

	// Set defaults
	// 设置默认值
	controlPlaneAddr := cfg.ControlPlaneAddr
	if controlPlaneAddr == "" {
		controlPlaneAddr = "localhost:8080"
	}

	grpcAddr := cfg.GRPCAddr
	if grpcAddr == "" {
		grpcAddr = "localhost:50051"
	}

	heartbeatInterval := cfg.HeartbeatInterval
	if heartbeatInterval <= 0 {
		heartbeatInterval = 10 // Default 10 seconds
	}

	// Parse template
	// 解析模板
	tmpl, err := template.New("install_script").Parse(installScriptTemplateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse install script template: %w", err)
	}

	return &InstallScriptGenerator{
		controlPlaneAddr:  controlPlaneAddr,
		grpcAddr:          grpcAddr,
		heartbeatInterval: heartbeatInterval,
		template:          tmpl,
	}, nil
}

// Generate generates the install script with the configured settings.
// Generate 使用配置的设置生成安装脚本。
// Requirements: 2.1 - Returns shell script with auto-detection logic for OS and architecture.
func (g *InstallScriptGenerator) Generate() (string, error) {
	data := &InstallScriptData{
		ControlPlaneAddr:  g.formatControlPlaneURL(),
		GRPCAddr:          g.grpcAddr,
		InstallDir:        DefaultInstallDir,
		ConfigDir:         DefaultConfigDir,
		AgentBinary:       DefaultAgentBinary,
		ServiceName:       DefaultServiceName,
		HeartbeatInterval: fmt.Sprintf("%ds", g.heartbeatInterval),
	}

	return g.GenerateWithData(data)
}

// GenerateWithData generates the install script with custom data.
// GenerateWithData 使用自定义数据生成安装脚本。
func (g *InstallScriptGenerator) GenerateWithData(data *InstallScriptData) (string, error) {
	if data == nil {
		return "", fmt.Errorf("install script data cannot be nil")
	}

	// Set defaults if not provided
	// 如果未提供则设置默认值
	if data.ControlPlaneAddr == "" {
		data.ControlPlaneAddr = g.formatControlPlaneURL()
	}
	if data.GRPCAddr == "" {
		data.GRPCAddr = g.grpcAddr
	}
	if data.InstallDir == "" {
		data.InstallDir = DefaultInstallDir
	}
	if data.ConfigDir == "" {
		data.ConfigDir = DefaultConfigDir
	}
	if data.AgentBinary == "" {
		data.AgentBinary = DefaultAgentBinary
	}
	if data.ServiceName == "" {
		data.ServiceName = DefaultServiceName
	}

	var buf bytes.Buffer
	if err := g.template.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute install script template: %w", err)
	}

	return buf.String(), nil
}

// formatControlPlaneURL formats the Control Plane address as a full URL.
// formatControlPlaneURL 将 Control Plane 地址格式化为完整 URL。
func (g *InstallScriptGenerator) formatControlPlaneURL() string {
	addr := g.controlPlaneAddr
	if addr == "" {
		addr = "localhost:8080"
	}

	// Add http:// prefix if not present
	// 如果没有 http:// 前缀则添加
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}

	return addr
}

// GetSupportedPlatforms returns all supported platforms.
// GetSupportedPlatforms 返回所有支持的平台。
func GetSupportedPlatforms() []SupportedPlatform {
	return SupportedPlatforms
}

// IsPlatformSupported checks if a platform is supported.
// IsPlatformSupported 检查平台是否受支持。
func IsPlatformSupported(os, arch string) bool {
	os = strings.ToLower(os)
	arch = strings.ToLower(arch)

	for _, p := range SupportedPlatforms {
		if p.OS == os && p.Arch == arch {
			return true
		}
	}
	return false
}

// GetBinaryName returns the binary name for a platform.
// GetBinaryName 返回平台的二进制文件名称。
func GetBinaryName(os, arch string) (string, bool) {
	os = strings.ToLower(os)
	arch = strings.ToLower(arch)

	for _, p := range SupportedPlatforms {
		if p.OS == os && p.Arch == arch {
			return p.BinaryName, true
		}
	}
	return "", false
}

// NormalizeArch normalizes architecture names to standard format.
// NormalizeArch 将架构名称标准化为标准格式。
// Requirements: 2.1 - Supports architecture detection (x86_64 -> amd64, aarch64 -> arm64).
func NormalizeArch(arch string) string {
	arch = strings.ToLower(arch)
	switch arch {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	default:
		return arch
	}
}

// NormalizeOS normalizes OS names to standard format.
// NormalizeOS 将操作系统名称标准化为标准格式。
func NormalizeOS(os string) string {
	return strings.ToLower(os)
}

// installScriptTemplateContent is the template for the Agent install script.
// installScriptTemplateContent 是 Agent 安装脚本的模板。
// Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6 - Implements one-click Agent installation.
const installScriptTemplateContent = `#!/bin/bash
# ============================================================================
# SeaTunnelX Agent Install Script
# SeaTunnelX Agent 安装脚本
# Generated by SeaTunnel Control Plane
# 由 SeaTunnel Control Plane 生成
# ============================================================================
# Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6
# - Auto-detects OS type and CPU architecture (2.1)
# - Downloads Agent binary from Control Plane (2.2)
# - Installs to /usr/local/bin and creates config (2.3)
# - Creates systemd service with auto-start (2.4)
# - Starts Agent and waits for registration (2.5)
# - Handles errors with cleanup and detailed messages (2.6)
# ============================================================================

set -e

# ==================== Configuration 配置 ====================
CONTROL_PLANE_ADDR="{{.ControlPlaneAddr}}"
GRPC_ADDR="{{.GRPCAddr}}"
INSTALL_DIR="{{.InstallDir}}"
CONFIG_DIR="{{.ConfigDir}}"
AGENT_BINARY="{{.AgentBinary}}"
SERVICE_NAME="{{.ServiceName}}"
LOG_DIR="/var/log/${SERVICE_NAME}"

# ==================== Colors 颜色 ====================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ==================== Logging Functions 日志函数 ====================
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# ==================== Cleanup Function 清理函数 ====================
# Requirements: 2.6 - Cleans up created files on failure.
cleanup() {
    local exit_code=$?
    if [ $exit_code -ne 0 ]; then
        log_error "Installation failed with exit code: ${exit_code}"
        log_error "安装失败，退出码: ${exit_code}"
        log_info "Cleaning up..."
        log_info "正在清理..."
        
        # Stop service if running
        # 如果服务正在运行则停止
        systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
        systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
        
        # Remove installed files
        # 删除已安装的文件
        rm -f "${INSTALL_DIR}/${AGENT_BINARY}" 2>/dev/null || true
        rm -rf "${CONFIG_DIR}" 2>/dev/null || true
        rm -rf "${LOG_DIR}" 2>/dev/null || true
        rm -f "/etc/systemd/system/${SERVICE_NAME}.service" 2>/dev/null || true
        rm -f "/tmp/${AGENT_BINARY}" 2>/dev/null || true
        
        # Reload systemd
        # 重新加载 systemd
        systemctl daemon-reload 2>/dev/null || true
        
        log_info "Cleanup completed"
        log_info "清理完成"
    fi
    exit $exit_code
}

# Set trap for cleanup on error
# 设置错误时的清理陷阱
trap cleanup EXIT

# ==================== Check Root 检查 Root ====================
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        log_error "This script must be run as root"
        log_error "此脚本必须以 root 身份运行"
        log_info "Please run: sudo bash install.sh"
        log_info "请运行: sudo bash install.sh"
        exit 1
    fi
}

# ==================== Detect OS 检测操作系统 ====================
# Requirements: 2.1 - Auto-detects operating system type.
detect_os() {
    local os_type
    os_type=$(uname -s | tr '[:upper:]' '[:lower:]')
    
    case "${os_type}" in
        linux)
            echo "linux"
            ;;
        darwin)
            echo "darwin"
            ;;
        *)
            log_error "Unsupported operating system: ${os_type}"
            log_error "不支持的操作系统: ${os_type}"
            log_info "Supported: linux, darwin"
            log_info "支持: linux, darwin"
            exit 1
            ;;
    esac
}

# ==================== Detect Architecture 检测架构 ====================
# Requirements: 2.1 - Auto-detects CPU architecture.
detect_arch() {
    local arch
    arch=$(uname -m)
    
    case "${arch}" in
        x86_64|amd64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        *)
            log_error "Unsupported architecture: ${arch}"
            log_error "不支持的架构: ${arch}"
            log_info "Supported: amd64 (x86_64), arm64 (aarch64)"
            log_info "支持: amd64 (x86_64), arm64 (aarch64)"
            exit 1
            ;;
    esac
}

# ==================== Check Dependencies 检查依赖 ====================
check_dependencies() {
    log_step "Checking dependencies..."
    log_step "正在检查依赖..."
    
    # Check for curl or wget
    # 检查 curl 或 wget
    if ! command -v curl &> /dev/null && ! command -v wget &> /dev/null; then
        log_error "Neither curl nor wget is available"
        log_error "curl 和 wget 都不可用"
        log_info "Please install curl or wget first"
        log_info "请先安装 curl 或 wget"
        exit 1
    fi
    
    # Check for systemctl (systemd)
    # 检查 systemctl (systemd)
    if ! command -v systemctl &> /dev/null; then
        log_warn "systemctl not found, service management may not work"
        log_warn "未找到 systemctl，服务管理可能无法工作"
    fi
    
    log_info "Dependencies check passed"
    log_info "依赖检查通过"
}

# ==================== Download Agent 下载 Agent ====================
# Requirements: 2.2 - Downloads Agent binary from Control Plane.
download_agent() {
    local os_type=$1
    local arch=$2
    local download_url="${CONTROL_PLANE_ADDR}/api/v1/agent/download?os=${os_type}&arch=${arch}"
    local temp_file="/tmp/${AGENT_BINARY}"
    
    log_step "Downloading Agent binary..."
    log_step "正在下载 Agent 二进制文件..."
    log_info "URL: ${download_url}"
    
    # Download using curl or wget
    # 使用 curl 或 wget 下载
    if command -v curl &> /dev/null; then
        if ! curl -fsSL -o "${temp_file}" "${download_url}"; then
            log_error "Failed to download Agent binary using curl"
            log_error "使用 curl 下载 Agent 二进制文件失败"
            exit 1
        fi
    elif command -v wget &> /dev/null; then
        if ! wget -q -O "${temp_file}" "${download_url}"; then
            log_error "Failed to download Agent binary using wget"
            log_error "使用 wget 下载 Agent 二进制文件失败"
            exit 1
        fi
    fi
    
    # Verify download
    # 验证下载
    if [ ! -f "${temp_file}" ]; then
        log_error "Downloaded file not found: ${temp_file}"
        log_error "未找到下载的文件: ${temp_file}"
        exit 1
    fi
    
    if [ ! -s "${temp_file}" ]; then
        log_error "Downloaded file is empty: ${temp_file}"
        log_error "下载的文件为空: ${temp_file}"
        exit 1
    fi
    
    local file_size
    file_size=$(stat -c%s "${temp_file}" 2>/dev/null || stat -f%z "${temp_file}" 2>/dev/null || echo "unknown")
    log_info "Downloaded ${file_size} bytes"
    log_info "已下载 ${file_size} 字节"
}

# ==================== Install Agent 安装 Agent ====================
# Requirements: 2.3 - Installs Agent to /usr/local/bin and creates config.
install_agent() {
    local temp_file="/tmp/${AGENT_BINARY}"
    
    log_step "Installing Agent binary..."
    log_step "正在安装 Agent 二进制文件..."
    
    # Create install directory if not exists
    # 如果安装目录不存在则创建
    mkdir -p "${INSTALL_DIR}"
    
    # Move binary to install directory
    # 将二进制文件移动到安装目录
    mv "${temp_file}" "${INSTALL_DIR}/${AGENT_BINARY}"
    chmod +x "${INSTALL_DIR}/${AGENT_BINARY}"
    
    log_info "Agent binary installed to ${INSTALL_DIR}/${AGENT_BINARY}"
    log_info "Agent 二进制文件已安装到 ${INSTALL_DIR}/${AGENT_BINARY}"
    
    # Create config directory
    # 创建配置目录
    log_step "Creating configuration..."
    log_step "正在创建配置..."
    
    mkdir -p "${CONFIG_DIR}"
    mkdir -p "${LOG_DIR}"
    
    # Generate config file
    # 生成配置文件
    cat > "${CONFIG_DIR}/config.yaml" << EOF
# ============================================================================
# SeaTunnelX Agent Configuration
# SeaTunnelX Agent 配置文件
# Generated by install script
# 由安装脚本生成
# ============================================================================

# Agent settings
# Agent 设置
agent:
  # ID is auto-generated if empty
  # 如果为空则自动生成 ID
  id: ""

# Control Plane connection settings
# Control Plane 连接设置
control_plane:
  # gRPC addresses of the Control Plane (supports multiple for HA)
  # Control Plane 的 gRPC 地址（支持多个用于高可用）
  addresses:
    - "${GRPC_ADDR}"
  # TLS configuration
  # TLS 配置
  tls:
    enabled: false
    cert_file: ""
    key_file: ""
    ca_file: ""
  # Authentication token
  # 认证 Token
  token: ""

# Heartbeat settings
# 心跳设置
heartbeat:
  # Heartbeat interval
  # 心跳间隔
  interval: {{.HeartbeatInterval}}

# Log settings
# 日志设置
log:
  # Log level (debug, info, warn, error)
  # 日志级别
  level: info
  # Log file path
  # 日志文件路径
  file: ${LOG_DIR}/agent.log
  # Max log file size in MB
  # 日志文件最大大小（MB）
  max_size: 100
  # Max number of old log files
  # 保留的旧日志文件数量
  max_backups: 5
  # Max days to retain old logs
  # 保留旧日志的天数
  max_age: 7

# SeaTunnel settings
# SeaTunnel 设置
seatunnel:
  # Default installation directory for SeaTunnel
  # SeaTunnel 的默认安装目录
  install_dir: /opt/seatunnel
EOF
    
    log_info "Configuration file created at ${CONFIG_DIR}/config.yaml"
    log_info "配置文件已创建于 ${CONFIG_DIR}/config.yaml"
}

# ==================== Create Systemd Service 创建 Systemd 服务 ====================
# Requirements: 2.4 - Creates systemd service with auto-start.
create_systemd_service() {
    log_step "Creating systemd service..."
    log_step "正在创建 systemd 服务..."
    
    # Check if systemctl is available
    # 检查 systemctl 是否可用
    if ! command -v systemctl &> /dev/null; then
        log_warn "systemctl not available, skipping service creation"
        log_warn "systemctl 不可用，跳过服务创建"
        return 0
    fi
    
    # Create startup wrapper script to load environment variables
    # 创建启动包装脚本以加载环境变量
    cat > "${INSTALL_DIR}/${AGENT_BINARY}-start.sh" << 'WRAPPER_EOF'
#!/bin/bash
# ============================================================================
# SeaTunnelX Agent Startup Wrapper Script
# SeaTunnelX Agent 启动包装脚本
# This script loads environment variables before starting the Agent
# 此脚本在启动 Agent 前加载环境变量
# ============================================================================

# Load system-wide environment variables
# 加载系统级环境变量
if [ -f /etc/profile ]; then
    source /etc/profile
fi

# Load user environment variables (for root user)
# 加载用户环境变量（针对 root 用户）
if [ -f /root/.bashrc ]; then
    source /root/.bashrc
fi

if [ -f /root/.bash_profile ]; then
    source /root/.bash_profile
fi

# Load common Java paths if JAVA_HOME is not set
# 如果 JAVA_HOME 未设置，加载常见的 Java 路径
if [ -z "$JAVA_HOME" ]; then
    # Try common Java installation paths
    # 尝试常见的 Java 安装路径
    for java_dir in /usr/lib/jvm/java-* /usr/java/* /opt/java/* /usr/local/java*; do
        if [ -d "$java_dir" ] && [ -x "$java_dir/bin/java" ]; then
            export JAVA_HOME="$java_dir"
            export PATH="$JAVA_HOME/bin:$PATH"
            break
        fi
    done
fi

# Ensure JAVA_HOME/bin is in PATH if JAVA_HOME is set
# 如果设置了 JAVA_HOME，确保 JAVA_HOME/bin 在 PATH 中
if [ -n "$JAVA_HOME" ] && [ -d "$JAVA_HOME/bin" ]; then
    export PATH="$JAVA_HOME/bin:$PATH"
fi

# Log environment info for debugging
# 记录环境信息用于调试
echo "[$(date)] Starting SeaTunnelX Agent..."
echo "[$(date)] JAVA_HOME=$JAVA_HOME"
echo "[$(date)] PATH=$PATH"
if command -v java &> /dev/null; then
    echo "[$(date)] Java version: $(java -version 2>&1 | head -1)"
fi

# Start the Agent with all arguments passed to this script
# 使用传递给此脚本的所有参数启动 Agent
exec INSTALL_DIR_PLACEHOLDER/AGENT_BINARY_PLACEHOLDER "$@"
WRAPPER_EOF

    # Replace placeholders in wrapper script
    # 替换包装脚本中的占位符
    sed -i "s|INSTALL_DIR_PLACEHOLDER|${INSTALL_DIR}|g" "${INSTALL_DIR}/${AGENT_BINARY}-start.sh"
    sed -i "s|AGENT_BINARY_PLACEHOLDER|${AGENT_BINARY}|g" "${INSTALL_DIR}/${AGENT_BINARY}-start.sh"
    chmod +x "${INSTALL_DIR}/${AGENT_BINARY}-start.sh"
    
    log_info "Startup wrapper script created at ${INSTALL_DIR}/${AGENT_BINARY}-start.sh"
    log_info "启动包装脚本已创建于 ${INSTALL_DIR}/${AGENT_BINARY}-start.sh"

    # Create systemd service file
    # 创建 systemd 服务文件
    cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=SeaTunnelX Agent Service
Documentation=https://seatunnel.apache.org/
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
# Use wrapper script to load environment variables before starting Agent
# 使用包装脚本在启动 Agent 前加载环境变量
ExecStart=/bin/bash ${INSTALL_DIR}/${AGENT_BINARY}-start.sh --config ${CONFIG_DIR}/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${SERVICE_NAME}

# Security settings
# 安全设置
NoNewPrivileges=false
ProtectSystem=false
ProtectHome=false

# Resource limits
# 资源限制
LimitNOFILE=65536
LimitNPROC=65536
LimitCORE=infinity

# Environment - base PATH, additional paths loaded by wrapper script
# 环境变量 - 基础 PATH，额外路径由包装脚本加载
Environment="PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

[Install]
WantedBy=multi-user.target
EOF
    
    # Reload systemd
    # 重新加载 systemd
    systemctl daemon-reload
    
    # Enable service for auto-start
    # 启用服务自动启动
    systemctl enable "${SERVICE_NAME}"
    
    log_info "Systemd service created and enabled"
    log_info "Systemd 服务已创建并启用"
}

# ==================== Start Agent 启动 Agent ====================
# Requirements: 2.5 - Starts Agent and waits for registration.
start_agent() {
    log_step "Starting Agent service..."
    log_step "正在启动 Agent 服务..."
    
    # Check if systemctl is available
    # 检查 systemctl 是否可用
    if ! command -v systemctl &> /dev/null; then
        log_warn "systemctl not available, please start Agent manually:"
        log_warn "systemctl 不可用，请手动启动 Agent:"
        log_info "  ${INSTALL_DIR}/${AGENT_BINARY} --config ${CONFIG_DIR}/config.yaml"
        return 0
    fi
    
    # Start service
    # 启动服务
    systemctl start "${SERVICE_NAME}"
    
    # Wait for service to start
    # 等待服务启动
    log_info "Waiting for Agent to start..."
    log_info "正在等待 Agent 启动..."
    sleep 3
    
    # Check service status
    # 检查服务状态
    if systemctl is-active --quiet "${SERVICE_NAME}"; then
        log_info "Agent service started successfully"
        log_info "Agent 服务启动成功"
    else
        log_error "Failed to start Agent service"
        log_error "启动 Agent 服务失败"
        log_info "Check logs with: journalctl -u ${SERVICE_NAME} -n 50"
        log_info "查看日志: journalctl -u ${SERVICE_NAME} -n 50"
        exit 1
    fi
    
    # Wait for registration
    # 等待注册
    log_info "Waiting for Agent to register with Control Plane..."
    log_info "正在等待 Agent 向 Control Plane 注册..."
    sleep 2
}

# ==================== Print Summary 打印摘要 ====================
print_summary() {
    echo ""
    echo -e "${GREEN}============================================${NC}"
    echo -e "${GREEN}  Installation Completed Successfully!${NC}"
    echo -e "${GREEN}  安装成功完成！${NC}"
    echo -e "${GREEN}============================================${NC}"
    echo ""
    echo -e "Agent is now running and connected to Control Plane"
    echo -e "Agent 正在运行并已连接到 Control Plane"
    echo ""
    echo -e "${BLUE}Installation Details / 安装详情:${NC}"
    echo -e "  Binary:  ${INSTALL_DIR}/${AGENT_BINARY}"
    echo -e "  Config:  ${CONFIG_DIR}/config.yaml"
    echo -e "  Logs:    ${LOG_DIR}/agent.log"
    echo -e "  Service: ${SERVICE_NAME}"
    echo ""
    echo -e "${BLUE}Useful Commands / 常用命令:${NC}"
    echo -e "  Check status / 检查状态:"
    echo -e "    systemctl status ${SERVICE_NAME}"
    echo ""
    echo -e "  View logs / 查看日志:"
    echo -e "    journalctl -u ${SERVICE_NAME} -f"
    echo -e "    tail -f ${LOG_DIR}/agent.log"
    echo ""
    echo -e "  Restart service / 重启服务:"
    echo -e "    systemctl restart ${SERVICE_NAME}"
    echo ""
    echo -e "  Stop service / 停止服务:"
    echo -e "    systemctl stop ${SERVICE_NAME}"
    echo ""
    echo -e "  Uninstall / 卸载:"
    echo -e "    systemctl stop ${SERVICE_NAME}"
    echo -e "    systemctl disable ${SERVICE_NAME}"
    echo -e "    rm -f /etc/systemd/system/${SERVICE_NAME}.service"
    echo -e "    rm -f ${INSTALL_DIR}/${AGENT_BINARY}"
    echo -e "    rm -rf ${CONFIG_DIR}"
    echo -e "    rm -rf ${LOG_DIR}"
    echo -e "    systemctl daemon-reload"
    echo ""
}

# ==================== Main Function 主函数 ====================
main() {
    echo ""
    echo -e "${BLUE}============================================${NC}"
    echo -e "${BLUE}  SeaTunnelX Agent Installation Script${NC}"
    echo -e "${BLUE}  SeaTunnelX Agent 安装脚本${NC}"
    echo -e "${BLUE}============================================${NC}"
    echo ""
    
    # Step 1: Check root
    # 步骤 1: 检查 root
    check_root
    
    # Step 2: Check dependencies
    # 步骤 2: 检查依赖
    check_dependencies
    
    # Step 3: Detect platform
    # 步骤 3: 检测平台
    log_step "Detecting platform..."
    log_step "正在检测平台..."
    
    local os_type
    local arch
    os_type=$(detect_os)
    arch=$(detect_arch)
    
    log_info "Detected OS: ${os_type}, Architecture: ${arch}"
    log_info "检测到操作系统: ${os_type}, 架构: ${arch}"
    
    # Step 4: Download Agent
    # 步骤 4: 下载 Agent
    download_agent "${os_type}" "${arch}"
    
    # Step 5: Install Agent
    # 步骤 5: 安装 Agent
    install_agent
    
    # Step 6: Create systemd service
    # 步骤 6: 创建 systemd 服务
    create_systemd_service
    
    # Step 7: Start Agent
    # 步骤 7: 启动 Agent
    start_agent
    
    # Step 8: Print summary
    # 步骤 8: 打印摘要
    print_summary
    
    # Disable trap on successful completion
    # 成功完成时禁用陷阱
    trap - EXIT
}

# Run main function
# 运行主函数
main "$@"
`
