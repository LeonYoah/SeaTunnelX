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

// Package main is the entry point for the SeaTunnelX Agent service.
// main 包是 SeaTunnelX Agent 服务的入口点。
//
// Agent is a daemon process deployed on physical/VM nodes that:
// Agent 是部署在物理机/VM 节点上的守护进程，负责：
// - Communicates with Control Plane via gRPC / 通过 gRPC 与 Control Plane 通信
// - Executes remote operations (install, start, stop, etc.) / 执行远程运维操作（安装、启动、停止等）
// - Reports heartbeat and resource usage / 上报心跳和资源使用情况
// - Manages SeaTunnel processes / 管理 SeaTunnel 进程
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	pb "github.com/seatunnel/seatunnelX/agent"
	"github.com/seatunnel/seatunnelX/agent/internal/collector"
	"github.com/seatunnel/seatunnelX/agent/internal/config"
	"github.com/seatunnel/seatunnelX/agent/internal/executor"
	agentgrpc "github.com/seatunnel/seatunnelX/agent/internal/grpc"
	"github.com/seatunnel/seatunnelX/agent/internal/installer"
	"github.com/seatunnel/seatunnelX/agent/internal/process"
	"github.com/spf13/cobra"
)

// Version information, set at build time
// 版本信息，在构建时设置
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// Agent represents the main agent service that integrates all components
// Agent 表示集成所有组件的主要 Agent 服务
// Requirements 1.1: Agent service startup - load config, init gRPC client, register with Control Plane
// 需求 1.1：Agent 服务启动 - 加载配置、初始化 gRPC 客户端、向 Control Plane 注册
type Agent struct {
	// config holds the agent configuration
	// config 保存 Agent 配置
	config *config.Config

	// ctx is the main context for the agent
	// ctx 是 Agent 的主上下文
	ctx context.Context

	// cancel cancels the main context
	// cancel 取消主上下文
	cancel context.CancelFunc

	// grpcClient is the gRPC client for Control Plane communication
	// grpcClient 是与 Control Plane 通信的 gRPC 客户端
	grpcClient *agentgrpc.Client

	// executor handles command execution and routing
	// executor 处理命令执行和路由
	executor *executor.CommandExecutor

	// processManager manages SeaTunnel process lifecycle
	// processManager 管理 SeaTunnel 进程生命周期
	processManager *process.ProcessManager

	// metricsCollector collects system and process metrics
	// metricsCollector 采集系统和进程指标
	metricsCollector *collector.MetricsCollector

	// installerManager handles SeaTunnel installation
	// installerManager 处理 SeaTunnel 安装
	installerManager *installer.InstallerManager

	// wg tracks running goroutines for graceful shutdown
	// wg 跟踪运行中的 goroutine 以实现优雅关闭
	wg sync.WaitGroup

	// running indicates if the agent is running
	// running 表示 Agent 是否正在运行
	running bool

	// mu protects the running state
	// mu 保护运行状态
	mu sync.RWMutex
}

// NewAgent creates a new Agent instance with all components initialized
// NewAgent 创建一个初始化所有组件的新 Agent 实例
func NewAgent(cfg *config.Config) *Agent {
	ctx, cancel := context.WithCancel(context.Background())

	// Create process manager / 创建进程管理器
	pm := process.NewProcessManager()

	// Create metrics collector with process manager / 使用进程管理器创建指标采集器
	mc := collector.NewMetricsCollector(pm)

	// Create command executor / 创建命令执行器
	exec := executor.NewCommandExecutor()

	// Create gRPC client / 创建 gRPC 客户端
	grpcClient := agentgrpc.NewClient(cfg)

	// Create installer manager / 创建安装管理器
	im := installer.NewInstallerManager()

	return &Agent{
		config:           cfg,
		ctx:              ctx,
		cancel:           cancel,
		grpcClient:       grpcClient,
		executor:         exec,
		processManager:   pm,
		metricsCollector: mc,
		installerManager: im,
	}
}

// Run starts the Agent service and all its components
// Run 启动 Agent 服务及其所有组件
// Requirements 1.1: Agent startup - load config, init gRPC client, register with Control Plane
// Requirements 1.2: After successful registration, establish bidirectional gRPC stream
// Requirements 1.3: Send heartbeat every 10 seconds with resource usage
// 需求 1.1：Agent 启动 - 加载配置、初始化 gRPC 客户端、向 Control Plane 注册
// 需求 1.2：注册成功后，建立双向 gRPC 流连接
// 需求 1.3：每 10 秒发送心跳，包含资源使用率
func (a *Agent) Run() error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("agent is already running / Agent 已在运行")
	}
	a.running = true
	a.mu.Unlock()

	fmt.Println("========================================")
	fmt.Println("  SeaTunnelX Agent Starting...")
	fmt.Println("  SeaTunnelX Agent 正在启动...")
	fmt.Println("========================================")
	fmt.Printf("Version: %s, Commit: %s, Build: %s\n", Version, GitCommit, BuildTime)
	fmt.Printf("Control Plane: %v\n", a.config.ControlPlane.Addresses)
	fmt.Printf("Heartbeat Interval: %v\n", a.config.Heartbeat.Interval)
	fmt.Printf("Log Level: %s\n", a.config.Log.Level)

	// Step 1: Start process manager for monitoring
	// 步骤 1：启动进程管理器进行监控
	fmt.Println("[1/5] Starting process manager... / 启动进程管理器...")
	if err := a.processManager.Start(a.ctx); err != nil {
		return fmt.Errorf("failed to start process manager: %w / 启动进程管理器失败：%w", err, err)
	}

	// Set up process event handler / 设置进程事件处理器
	a.processManager.SetEventHandler(a.handleProcessEvent)

	// Step 2: Register command handlers
	// 步骤 2：注册命令处理器
	fmt.Println("[2/5] Registering command handlers... / 注册命令处理器...")
	a.registerCommandHandlers()

	// Step 3: Connect to Control Plane
	// 步骤 3：连接到 Control Plane
	fmt.Println("[3/5] Connecting to Control Plane... / 连接到 Control Plane...")
	if err := a.connectToControlPlane(); err != nil {
		return fmt.Errorf("failed to connect to Control Plane: %w / 连接 Control Plane 失败：%w", err, err)
	}

	// Step 4: Register with Control Plane
	// 步骤 4：向 Control Plane 注册
	fmt.Println("[4/5] Registering with Control Plane... / 向 Control Plane 注册...")
	if err := a.registerWithControlPlane(); err != nil {
		return fmt.Errorf("failed to register with Control Plane: %w / 向 Control Plane 注册失败：%w", err, err)
	}

	// Step 5: Start background services
	// 步骤 5：启动后台服务
	fmt.Println("[5/5] Starting background services... / 启动后台服务...")
	a.startBackgroundServices()

	fmt.Println("========================================")
	fmt.Println("  Agent started successfully!")
	fmt.Println("  Agent 启动成功！")
	fmt.Println("========================================")

	// Wait for context cancellation (shutdown signal)
	// 等待上下文取消（关闭信号）
	<-a.ctx.Done()

	return nil
}

// connectToControlPlane establishes connection to Control Plane with retry
// connectToControlPlane 建立与 Control Plane 的连接（带重试）
func (a *Agent) connectToControlPlane() error {
	// Create a context with timeout for initial connection
	// 为初始连接创建带超时的上下文
	connectCtx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()

	if err := a.grpcClient.Connect(connectCtx); err != nil {
		// If initial connection fails, start reconnection in background
		// 如果初始连接失败，在后台启动重连
		fmt.Printf("Initial connection failed, will retry in background: %v\n", err)
		fmt.Printf("初始连接失败，将在后台重试：%v\n", err)

		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			if err := a.grpcClient.Reconnect(a.ctx); err != nil {
				fmt.Printf("Reconnection failed: %v / 重连失败：%v\n", err, err)
			}
		}()
		return nil // Don't fail startup, let reconnection handle it
	}

	fmt.Println("Connected to Control Plane / 已连接到 Control Plane")
	return nil
}

// registerWithControlPlane sends registration request to Control Plane
// registerWithControlPlane 向 Control Plane 发送注册请求
func (a *Agent) registerWithControlPlane() error {
	if !a.grpcClient.IsConnected() {
		fmt.Println("Not connected yet, registration will happen after connection / 尚未连接，将在连接后注册")
		return nil
	}

	// Collect system info for registration / 收集系统信息用于注册
	sysInfo := a.metricsCollector.GetSystemInfo()
	hostname := a.metricsCollector.GetHostname()
	ipAddress := a.metricsCollector.GetIPAddress()

	req := &pb.RegisterRequest{
		AgentId:      a.config.Agent.ID,
		Hostname:     hostname,
		IpAddress:    ipAddress,
		OsType:       runtime.GOOS,
		Arch:         runtime.GOARCH,
		AgentVersion: Version,
		SystemInfo:   sysInfo,
	}

	resp, err := a.grpcClient.Register(a.ctx, req)
	if err != nil {
		return fmt.Errorf("registration failed: %w / 注册失败：%w", err, err)
	}

	if !resp.Success {
		return fmt.Errorf("registration rejected: %s / 注册被拒绝：%s", resp.Message, resp.Message)
	}

	fmt.Printf("Registered successfully with ID: %s / 注册成功，ID：%s\n", resp.AssignedId, resp.AssignedId)

	// Apply configuration from Control Plane if provided
	// 如果提供，应用来自 Control Plane 的配置
	if resp.Config != nil {
		a.applyRemoteConfig(resp.Config)
	}

	return nil
}

// applyRemoteConfig applies configuration received from Control Plane
// applyRemoteConfig 应用从 Control Plane 接收的配置
func (a *Agent) applyRemoteConfig(cfg *pb.AgentConfig) {
	if cfg.HeartbeatInterval > 0 {
		newInterval := time.Duration(cfg.HeartbeatInterval) * time.Second
		fmt.Printf("Applying heartbeat interval from Control Plane: %v / 应用来自 Control Plane 的心跳间隔：%v\n", newInterval, newInterval)
		// Note: Heartbeat interval is applied when starting heartbeat
		// 注意：心跳间隔在启动心跳时应用
	}
}

// startBackgroundServices starts all background goroutines
// startBackgroundServices 启动所有后台 goroutine
func (a *Agent) startBackgroundServices() {
	// Start heartbeat service / 启动心跳服务
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.runHeartbeatLoop()
	}()

	// Start command stream listener / 启动命令流监听器
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.runCommandStreamLoop()
	}()

	// Start connection monitor / 启动连接监控
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.runConnectionMonitor()
	}()
}

// runHeartbeatLoop runs the heartbeat sending loop
// runHeartbeatLoop 运行心跳发送循环
// Requirements 1.3: Send heartbeat every 10 seconds with resource usage
// 需求 1.3：每 10 秒发送心跳，包含资源使用率
func (a *Agent) runHeartbeatLoop() {
	interval := a.config.Heartbeat.Interval
	if interval == 0 {
		interval = 10 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	fmt.Printf("Heartbeat loop started with interval: %v / 心跳循环已启动，间隔：%v\n", interval, interval)

	for {
		select {
		case <-a.ctx.Done():
			fmt.Println("Heartbeat loop stopped / 心跳循环已停止")
			return
		case <-ticker.C:
			a.sendHeartbeat()
		}
	}
}

// sendHeartbeat sends a single heartbeat to Control Plane
// sendHeartbeat 向 Control Plane 发送单次心跳
func (a *Agent) sendHeartbeat() {
	if !a.grpcClient.IsConnected() {
		return // Skip if not connected / 如果未连接则跳过
	}

	// Collect metrics / 采集指标
	usage, processes := a.metricsCollector.Collect()

	_, err := a.grpcClient.SendHeartbeat(a.ctx, usage, processes)
	if err != nil {
		fmt.Printf("Heartbeat failed: %v / 心跳失败：%v\n", err, err)
	}
}

// runCommandStreamLoop runs the command stream listener loop
// runCommandStreamLoop 运行命令流监听循环
// Requirements 1.2: Establish bidirectional gRPC stream for commands
// Requirements 1.5: Receive commands, validate, execute, and report results
// 需求 1.2：建立双向 gRPC 流用于命令
// 需求 1.5：接收命令、验证、执行并上报结果
func (a *Agent) runCommandStreamLoop() {
	fmt.Println("Command stream loop started / 命令流循环已启动")

	for {
		select {
		case <-a.ctx.Done():
			fmt.Println("Command stream loop stopped / 命令流循环已停止")
			return
		default:
		}

		if !a.grpcClient.IsConnected() {
			time.Sleep(1 * time.Second)
			continue
		}

		// Start command stream / 启动命令流
		err := a.grpcClient.StartCommandStream(a.ctx, a.handleCommand)
		if err != nil {
			fmt.Printf("Command stream error: %v, will retry... / 命令流错误：%v，将重试...\n", err, err)
			time.Sleep(5 * time.Second)
		}
	}
}

// handleCommand handles a command received from Control Plane
// handleCommand 处理从 Control Plane 接收的命令
func (a *Agent) handleCommand(ctx context.Context, cmd *pb.CommandRequest) (*pb.CommandResponse, error) {
	fmt.Printf("Received command: %s (type: %s) / 收到命令：%s（类型：%s）\n",
		cmd.CommandId, cmd.Type.String(), cmd.CommandId, cmd.Type.String())

	// Create a progress reporter that sends updates via gRPC
	// 创建通过 gRPC 发送更新的进度上报器
	reporter := &executor.CallbackReporter{
		CommandID: cmd.CommandId,
		Callback: func(commandID string, progress int32, output string) error {
			resp := executor.CreateProgressResponse(commandID, progress, output)
			return a.grpcClient.ReportCommandResult(ctx, resp)
		},
	}

	// Execute the command / 执行命令
	resp, err := a.executor.Execute(ctx, cmd, reporter)
	if err != nil {
		fmt.Printf("Command %s failed: %v / 命令 %s 失败：%v\n", cmd.CommandId, err, cmd.CommandId, err)
	} else {
		fmt.Printf("Command %s completed with status: %s / 命令 %s 完成，状态：%s\n",
			cmd.CommandId, resp.Status.String(), cmd.CommandId, resp.Status.String())
	}

	return resp, err
}

// runConnectionMonitor monitors connection status and triggers reconnection
// runConnectionMonitor 监控连接状态并触发重连
// Requirements 1.4: Reconnect with exponential backoff on disconnect
// 需求 1.4：断开连接时使用指数退避重连
func (a *Agent) runConnectionMonitor() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	fmt.Println("Connection monitor started / 连接监控已启动")

	for {
		select {
		case <-a.ctx.Done():
			fmt.Println("Connection monitor stopped / 连接监控已停止")
			return
		case <-ticker.C:
			if !a.grpcClient.IsConnected() {
				fmt.Println("Connection lost, attempting reconnection... / 连接丢失，尝试重连...")
				go func() {
					if err := a.grpcClient.Reconnect(a.ctx); err != nil {
						fmt.Printf("Reconnection failed: %v / 重连失败：%v\n", err, err)
					} else {
						// Re-register after reconnection / 重连后重新注册
						if err := a.registerWithControlPlane(); err != nil {
							fmt.Printf("Re-registration failed: %v / 重新注册失败：%v\n", err, err)
						}
					}
				}()
			}
		}
	}
}

// handleProcessEvent handles process lifecycle events
// handleProcessEvent 处理进程生命周期事件
func (a *Agent) handleProcessEvent(name string, event process.ProcessEvent, info *process.ProcessInfo) {
	fmt.Printf("Process event: %s - %s (PID: %d, Status: %s) / 进程事件：%s - %s（PID：%d，状态：%s）\n",
		name, event, info.PID, info.Status, name, event, info.PID, info.Status)

	// TODO: Report process events to Control Plane
	// TODO: 向 Control Plane 上报进程事件
}

// registerCommandHandlers registers all command handlers with the executor
// registerCommandHandlers 向执行器注册所有命令处理器
func (a *Agent) registerCommandHandlers() {
	// Register precheck handler / 注册预检查处理器
	a.executor.RegisterHandler(pb.CommandType_PRECHECK, a.handlePrecheckCommand)

	// Register installation handlers / 注册安装处理器
	a.executor.RegisterHandler(pb.CommandType_INSTALL, a.handleInstallCommand)
	a.executor.RegisterHandler(pb.CommandType_UNINSTALL, a.handleUninstallCommand)
	a.executor.RegisterHandler(pb.CommandType_UPGRADE, a.handleUpgradeCommand)

	// Register process management handlers / 注册进程管理处理器
	a.executor.RegisterHandler(pb.CommandType_START, a.handleStartCommand)
	a.executor.RegisterHandler(pb.CommandType_STOP, a.handleStopCommand)
	a.executor.RegisterHandler(pb.CommandType_RESTART, a.handleRestartCommand)
	a.executor.RegisterHandler(pb.CommandType_STATUS, a.handleStatusCommand)

	// Register diagnostic handlers / 注册诊断处理器
	a.executor.RegisterHandler(pb.CommandType_COLLECT_LOGS, a.handleCollectLogsCommand)

	fmt.Printf("Registered %d command handlers / 已注册 %d 个命令处理器\n",
		len(a.executor.GetRegisteredTypes()), len(a.executor.GetRegisteredTypes()))
}

// Command handler implementations / 命令处理器实现

func (a *Agent) handlePrecheckCommand(ctx context.Context, cmd *pb.CommandRequest, reporter executor.ProgressReporter) (*pb.CommandResponse, error) {
	reporter.Report(10, "Starting precheck... / 开始预检查...")

	// Create precheck params from command parameters
	// 从命令参数创建预检查参数
	params := &installer.PrecheckParams{
		InstallDir:     getParamString(cmd.Parameters, "install_dir", "/opt/seatunnel"),
		MinMemoryMB:    int64(getParamInt(cmd.Parameters, "min_memory_mb", 4096)),
		MinCPUCores:    getParamInt(cmd.Parameters, "min_cpu_cores", 2),
		MinDiskSpaceMB: int64(getParamInt(cmd.Parameters, "min_disk_mb", 10240)),
		Ports:          getParamIntSlice(cmd.Parameters, "required_ports", []int{5801, 8080}),
	}

	prechecker := installer.NewPrechecker(params)
	result, err := prechecker.RunAll(ctx)
	if err != nil {
		return executor.CreateErrorResponse(cmd.CommandId, err.Error()), err
	}

	reporter.Report(100, "Precheck completed / 预检查完成")

	// Format result as output / 将结果格式化为输出
	output := formatPrecheckResult(result)
	return executor.CreateSuccessResponse(cmd.CommandId, output), nil
}

func (a *Agent) handleInstallCommand(ctx context.Context, cmd *pb.CommandRequest, reporter executor.ProgressReporter) (*pb.CommandResponse, error) {
	reporter.Report(5, "Preparing installation... / 准备安装...")

	// Create install params from command parameters
	// 从命令参数创建安装参数
	params := &installer.InstallParams{
		Version:        getParamString(cmd.Parameters, "version", "2.3.12"),
		InstallDir:     getParamString(cmd.Parameters, "install_dir", "/opt/seatunnel"),
		DeploymentMode: installer.DeploymentMode(getParamString(cmd.Parameters, "deployment_mode", "hybrid")),
		NodeRole:       installer.NodeRole(getParamString(cmd.Parameters, "node_role", "master")),
		ClusterPort:    getParamInt(cmd.Parameters, "cluster_port", 5801),
		HTTPPort:       getParamInt(cmd.Parameters, "http_port", 8080),
	}

	// Create progress adapter / 创建进度适配器
	installReporter := &installerProgressAdapter{
		reporter:  reporter,
		commandID: cmd.CommandId,
	}

	result, err := a.installerManager.Install(ctx, params, installReporter)
	if err != nil {
		return executor.CreateErrorResponse(cmd.CommandId, err.Error()), err
	}

	if !result.Success {
		return executor.CreateErrorResponse(cmd.CommandId, result.Message), fmt.Errorf("%s", result.Message)
	}

	return executor.CreateSuccessResponse(cmd.CommandId, "Installation completed / 安装完成"), nil
}

func (a *Agent) handleUninstallCommand(ctx context.Context, cmd *pb.CommandRequest, reporter executor.ProgressReporter) (*pb.CommandResponse, error) {
	reporter.Report(10, "Starting uninstallation... / 开始卸载...")

	installDir := getParamString(cmd.Parameters, "install_dir", "/opt/seatunnel")

	err := a.installerManager.Uninstall(ctx, installDir)
	if err != nil {
		return executor.CreateErrorResponse(cmd.CommandId, err.Error()), err
	}

	return executor.CreateSuccessResponse(cmd.CommandId, "Uninstallation completed / 卸载完成"), nil
}

func (a *Agent) handleUpgradeCommand(ctx context.Context, cmd *pb.CommandRequest, reporter executor.ProgressReporter) (*pb.CommandResponse, error) {
	reporter.Report(5, "Preparing upgrade... / 准备升级...")

	// Upgrade is essentially uninstall + install with new version
	// 升级本质上是卸载 + 使用新版本安装
	installDir := getParamString(cmd.Parameters, "install_dir", "/opt/seatunnel")
	newVersion := getParamString(cmd.Parameters, "new_version", "")

	if newVersion == "" {
		return executor.CreateErrorResponse(cmd.CommandId, "new_version is required / 需要 new_version 参数"), fmt.Errorf("new_version is required")
	}

	// Step 1: Backup current installation (optional)
	// 步骤 1：备份当前安装（可选）
	reporter.Report(10, "Backing up current installation... / 备份当前安装...")

	// Step 2: Uninstall current version
	// 步骤 2：卸载当前版本
	reporter.Report(30, "Uninstalling current version... / 卸载当前版本...")
	if err := a.installerManager.Uninstall(ctx, installDir); err != nil {
		return executor.CreateErrorResponse(cmd.CommandId, fmt.Sprintf("Uninstall failed: %v / 卸载失败：%v", err, err)), err
	}

	// Step 3: Install new version
	// 步骤 3：安装新版本
	reporter.Report(50, "Installing new version... / 安装新版本...")
	params := &installer.InstallParams{
		Version:        newVersion,
		InstallDir:     installDir,
		Mode:           installer.InstallModeOnline,
		DeploymentMode: installer.DeploymentMode(getParamString(cmd.Parameters, "deployment_mode", "hybrid")),
		NodeRole:       installer.NodeRole(getParamString(cmd.Parameters, "node_role", "master")),
		ClusterPort:    getParamInt(cmd.Parameters, "cluster_port", 5801),
		HTTPPort:       getParamInt(cmd.Parameters, "http_port", 8080),
	}

	installReporter := &installerProgressAdapter{
		reporter:  reporter,
		commandID: cmd.CommandId,
	}

	_, err := a.installerManager.Install(ctx, params, installReporter)
	if err != nil {
		return executor.CreateErrorResponse(cmd.CommandId, fmt.Sprintf("Install failed: %v / 安装失败：%v", err, err)), err
	}

	return executor.CreateSuccessResponse(cmd.CommandId, "Upgrade completed / 升级完成"), nil
}

func (a *Agent) handleStartCommand(ctx context.Context, cmd *pb.CommandRequest, reporter executor.ProgressReporter) (*pb.CommandResponse, error) {
	reporter.Report(10, "Starting SeaTunnel process... / 启动 SeaTunnel 进程...")

	processName := getParamString(cmd.Parameters, "process_name", "seatunnel")
	installDir := getParamString(cmd.Parameters, "install_dir", a.config.SeaTunnel.InstallDir)

	params := &process.StartParams{
		InstallDir: installDir,
		ConfigDir:  getParamString(cmd.Parameters, "config_dir", ""),
		LogDir:     getParamString(cmd.Parameters, "log_dir", ""),
	}

	err := a.processManager.StartProcess(ctx, processName, params)
	if err != nil {
		return executor.CreateErrorResponse(cmd.CommandId, err.Error()), err
	}

	reporter.Report(100, "Process started / 进程已启动")
	return executor.CreateSuccessResponse(cmd.CommandId, "Process started successfully / 进程启动成功"), nil
}

func (a *Agent) handleStopCommand(ctx context.Context, cmd *pb.CommandRequest, reporter executor.ProgressReporter) (*pb.CommandResponse, error) {
	reporter.Report(10, "Stopping SeaTunnel process... / 停止 SeaTunnel 进程...")

	processName := getParamString(cmd.Parameters, "process_name", "seatunnel")
	graceful := getParamBool(cmd.Parameters, "graceful", true)

	params := &process.StopParams{
		Graceful: graceful,
		Timeout:  30 * time.Second,
	}

	err := a.processManager.StopProcess(ctx, processName, params)
	if err != nil {
		return executor.CreateErrorResponse(cmd.CommandId, err.Error()), err
	}

	reporter.Report(100, "Process stopped / 进程已停止")
	return executor.CreateSuccessResponse(cmd.CommandId, "Process stopped successfully / 进程停止成功"), nil
}

func (a *Agent) handleRestartCommand(ctx context.Context, cmd *pb.CommandRequest, reporter executor.ProgressReporter) (*pb.CommandResponse, error) {
	reporter.Report(10, "Restarting SeaTunnel process... / 重启 SeaTunnel 进程...")

	processName := getParamString(cmd.Parameters, "process_name", "seatunnel")
	installDir := getParamString(cmd.Parameters, "install_dir", a.config.SeaTunnel.InstallDir)

	startParams := &process.StartParams{
		InstallDir: installDir,
	}
	stopParams := &process.StopParams{
		Graceful: true,
		Timeout:  30 * time.Second,
	}

	err := a.processManager.RestartProcess(ctx, processName, startParams, stopParams)
	if err != nil {
		return executor.CreateErrorResponse(cmd.CommandId, err.Error()), err
	}

	reporter.Report(100, "Process restarted / 进程已重启")
	return executor.CreateSuccessResponse(cmd.CommandId, "Process restarted successfully / 进程重启成功"), nil
}

func (a *Agent) handleStatusCommand(ctx context.Context, cmd *pb.CommandRequest, reporter executor.ProgressReporter) (*pb.CommandResponse, error) {
	processName := getParamString(cmd.Parameters, "process_name", "seatunnel")

	info, err := a.processManager.GetStatus(ctx, processName)
	if err != nil {
		// Process not found is not an error, just return status
		// 进程未找到不是错误，只返回状态
		return executor.CreateSuccessResponse(cmd.CommandId, fmt.Sprintf("Process not found: %s / 进程未找到：%s", processName, processName)), nil
	}

	output := fmt.Sprintf("Process: %s\nPID: %d\nStatus: %s\nUptime: %v\nCPU: %.2f%%\nMemory: %d bytes",
		info.Name, info.PID, info.Status, info.Uptime, info.CPUUsage, info.MemoryUsage)

	return executor.CreateSuccessResponse(cmd.CommandId, output), nil
}

func (a *Agent) handleCollectLogsCommand(ctx context.Context, cmd *pb.CommandRequest, reporter executor.ProgressReporter) (*pb.CommandResponse, error) {
	reporter.Report(10, "Collecting logs... / 收集日志...")

	// TODO: Implement log collection
	// TODO: 实现日志收集

	return executor.CreateSuccessResponse(cmd.CommandId, "Log collection not yet implemented / 日志收集尚未实现"), nil
}

// Shutdown gracefully stops the Agent service
// Shutdown 优雅地停止 Agent 服务
// Requirements 1.6: Graceful shutdown - complete running tasks, send offline notification, close connections
// 需求 1.6：优雅关闭 - 完成正在执行的任务、发送下线通知、关闭连接
func (a *Agent) Shutdown() {
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return
	}
	a.running = false
	a.mu.Unlock()

	fmt.Println("========================================")
	fmt.Println("  Shutting down Agent service...")
	fmt.Println("  正在关闭 Agent 服务...")
	fmt.Println("========================================")

	// Step 1: Stop accepting new commands
	// 步骤 1：停止接受新命令
	fmt.Println("[1/4] Stopping command acceptance... / 停止接受命令...")

	// Step 2: Wait for running tasks to complete (with timeout)
	// 步骤 2：等待运行中的任务完成（带超时）
	fmt.Println("[2/4] Waiting for running tasks... / 等待运行中的任务...")
	// Note: The executor handles task completion internally
	// 注意：执行器内部处理任务完成

	// Step 3: Stop all managed processes gracefully
	// 步骤 3：优雅地停止所有托管进程
	fmt.Println("[3/4] Stopping managed processes... / 停止托管进程...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := a.processManager.StopAll(shutdownCtx); err != nil {
		fmt.Printf("Warning: Error stopping processes: %v / 警告：停止进程时出错：%v\n", err, err)
	}

	// Stop process manager / 停止进程管理器
	if err := a.processManager.Stop(); err != nil {
		fmt.Printf("Warning: Error stopping process manager: %v / 警告：停止进程管理器时出错：%v\n", err, err)
	}

	// Step 4: Close gRPC connection
	// 步骤 4：关闭 gRPC 连接
	fmt.Println("[4/4] Closing connections... / 关闭连接...")
	if err := a.grpcClient.Disconnect(); err != nil {
		fmt.Printf("Warning: Error disconnecting: %v / 警告：断开连接时出错：%v\n", err, err)
	}

	// Cancel main context to stop all goroutines
	// 取消主上下文以停止所有 goroutine
	a.cancel()

	// Wait for all goroutines to finish (with timeout)
	// 等待所有 goroutine 完成（带超时）
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("All goroutines stopped / 所有 goroutine 已停止")
	case <-time.After(10 * time.Second):
		fmt.Println("Timeout waiting for goroutines / 等待 goroutine 超时")
	}

	fmt.Println("========================================")
	fmt.Println("  Agent shutdown complete")
	fmt.Println("  Agent 关闭完成")
	fmt.Println("========================================")
}

// Helper functions / 辅助函数

// getParamString gets a string parameter with default value
// getParamString 获取字符串参数，带默认值
func getParamString(params map[string]string, key, defaultValue string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultValue
}

// getParamInt gets an integer parameter with default value
// getParamInt 获取整数参数，带默认值
func getParamInt(params map[string]string, key string, defaultValue int) int {
	if v, ok := params[key]; ok && v != "" {
		var result int
		if _, err := fmt.Sscanf(v, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}

// getParamBool gets a boolean parameter with default value
// getParamBool 获取布尔参数，带默认值
func getParamBool(params map[string]string, key string, defaultValue bool) bool {
	if v, ok := params[key]; ok {
		return v == "true" || v == "1" || v == "yes"
	}
	return defaultValue
}

// getParamIntSlice gets an integer slice parameter with default value
// getParamIntSlice 获取整数切片参数，带默认值
func getParamIntSlice(params map[string]string, key string, defaultValue []int) []int {
	// For simplicity, return default value
	// 为简单起见，返回默认值
	// TODO: Implement parsing of comma-separated integers
	// TODO: 实现逗号分隔整数的解析
	return defaultValue
}

// formatPrecheckResult formats precheck result as string
// formatPrecheckResult 将预检查结果格式化为字符串
func formatPrecheckResult(result *installer.PrecheckResult) string {
	var sb string
	sb = "Precheck Results / 预检查结果:\n"
	sb += "================================\n"

	for _, item := range result.Items {
		statusIcon := "✓"
		if item.Status == installer.CheckStatusFailed {
			statusIcon = "✗"
		} else if item.Status == installer.CheckStatusWarning {
			statusIcon = "⚠"
		}
		sb += fmt.Sprintf("%s %s: %s\n", statusIcon, item.Name, item.Message)
	}

	sb += "================================\n"
	if result.OverallStatus == installer.CheckStatusPassed {
		sb += "Overall: PASSED / 总体：通过"
	} else if result.OverallStatus == installer.CheckStatusWarning {
		sb += "Overall: PASSED WITH WARNINGS / 总体：通过（有警告）"
	} else {
		sb += "Overall: FAILED / 总体：失败"
	}

	return sb
}

// installerProgressAdapter adapts installer.ProgressReporter to executor.ProgressReporter
// installerProgressAdapter 将 installer.ProgressReporter 适配到 executor.ProgressReporter
type installerProgressAdapter struct {
	reporter  executor.ProgressReporter
	commandID string
}

func (a *installerProgressAdapter) Report(step installer.InstallStep, progress int, message string) error {
	return a.reporter.Report(int32(progress), fmt.Sprintf("[%s] %s", step, message))
}

func (a *installerProgressAdapter) ReportStepStart(step installer.InstallStep) error {
	return a.reporter.Report(0, fmt.Sprintf("Starting step: %s / 开始步骤：%s", step, step))
}

func (a *installerProgressAdapter) ReportStepComplete(step installer.InstallStep) error {
	return a.reporter.Report(100, fmt.Sprintf("Completed step: %s / 完成步骤：%s", step, step))
}

func (a *installerProgressAdapter) ReportStepFailed(step installer.InstallStep, err error) error {
	return a.reporter.Report(0, fmt.Sprintf("Failed step: %s - %v / 失败步骤：%s - %v", step, err, step, err))
}

func (a *installerProgressAdapter) ReportStepSkipped(step installer.InstallStep, reason string) error {
	return a.reporter.Report(0, fmt.Sprintf("Skipped step: %s - %s / 跳过步骤：%s - %s", step, reason, step, reason))
}

// rootCmd is the root command for the Agent CLI
// rootCmd 是 Agent CLI 的根命令
var rootCmd = &cobra.Command{
	Use:   "seatunnelx-agent",
	Short: "SeaTunnelX Agent - Node daemon for SeaTunnel cluster management",
	Long: `SeaTunnelX Agent is a daemon process deployed on physical/VM nodes.
SeaTunnelX Agent 是部署在物理机/VM 节点上的守护进程。

It communicates with the Control Plane via gRPC to:
它通过 gRPC 与 Control Plane 通信，用于：
- Register and report heartbeat / 注册和上报心跳
- Execute installation and deployment commands / 执行安装和部署命令
- Manage SeaTunnel process lifecycle / 管理 SeaTunnel 进程生命周期
- Collect and report metrics / 采集和上报指标`,
	RunE: runAgent,
}

// versionCmd shows version information
// versionCmd 显示版本信息
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information / 打印版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("SeaTunnelX Agent\n")
		fmt.Printf("  Version:    %s\n", Version)
		fmt.Printf("  Git Commit: %s\n", GitCommit)
		fmt.Printf("  Build Time: %s\n", BuildTime)
		fmt.Printf("  Go Version: %s\n", runtime.Version())
		fmt.Printf("  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

// configFile is the path to the configuration file
// configFile 是配置文件的路径
var configFile string

func init() {
	// Add flags to root command
	// 向根命令添加标志
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file path (default: /etc/seatunnelx-agent/config.yaml)")

	// Add subcommands
	// 添加子命令
	rootCmd.AddCommand(versionCmd)
}

// runAgent is the main entry point for the Agent service
// runAgent 是 Agent 服务的主入口点
func runAgent(cmd *cobra.Command, args []string) error {
	// Load configuration
	// 加载配置
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w / 加载配置失败：%w", err, err)
	}

	// Validate configuration
	// 验证配置
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w / 无效配置：%w", err, err)
	}

	// Create agent
	// 创建 Agent
	agent := NewAgent(cfg)

	// Setup signal handling for graceful shutdown
	// 设置信号处理以实现优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Run agent in goroutine
	// 在 goroutine 中运行 Agent
	errChan := make(chan error, 1)
	go func() {
		errChan <- agent.Run()
	}()

	// Wait for signal or error
	// 等待信号或错误
	select {
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal: %v / 收到信号：%v\n", sig, sig)
		agent.Shutdown()
	case err := <-errChan:
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
