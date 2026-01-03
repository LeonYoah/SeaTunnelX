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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	pb "github.com/seatunnel/seatunnelX/agent"
	"github.com/seatunnel/seatunnelX/agent/internal/collector"
	"github.com/seatunnel/seatunnelX/agent/internal/config"
	"github.com/seatunnel/seatunnelX/agent/internal/discovery"
	"github.com/seatunnel/seatunnelX/agent/internal/executor"
	agentgrpc "github.com/seatunnel/seatunnelX/agent/internal/grpc"
	"github.com/seatunnel/seatunnelX/agent/internal/installer"
	"github.com/seatunnel/seatunnelX/agent/internal/monitor"
	"github.com/seatunnel/seatunnelX/agent/internal/process"
	"github.com/seatunnel/seatunnelX/agent/internal/restart"
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

	// processMonitor monitors SeaTunnel process status
	// processMonitor 监控 SeaTunnel 进程状态
	processMonitor *monitor.ProcessMonitor

	// autoRestarter handles automatic process restart
	// autoRestarter 处理自动进程重启
	autoRestarter *restart.AutoRestarter

	// eventReporter handles process event reporting
	// eventReporter 处理进程事件上报
	eventReporter *monitor.EventReporter

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

	// Create process monitor / 创建进程监控器
	pmon := monitor.NewProcessMonitor()

	// Create auto restarter / 创建自动重启器
	ar := restart.NewAutoRestarter(pm)

	// Create event reporter / 创建事件上报器
	er := monitor.NewEventReporter(nil) // Will set report func later / 稍后设置上报函数

	return &Agent{
		config:           cfg,
		ctx:              ctx,
		cancel:           cancel,
		grpcClient:       grpcClient,
		executor:         exec,
		processManager:   pm,
		metricsCollector: mc,
		installerManager: im,
		processMonitor:   pmon,
		autoRestarter:    ar,
		eventReporter:    er,
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
	fmt.Println("[1/8] Starting process manager... / 启动进程管理器...")
	if err := a.processManager.Start(a.ctx); err != nil {
		return fmt.Errorf("failed to start process manager: %w / 启动进程管理器失败：%w", err, err)
	}

	// Set up process event handler / 设置进程事件处理器
	a.processManager.SetEventHandler(a.handleProcessEvent)

	// Step 2: Start process monitor / 启动进程监控器
	fmt.Println("[2/8] Starting process monitor... / 启动进程监控器...")
	a.setupProcessMonitor()
	if err := a.processMonitor.Start(a.ctx); err != nil {
		fmt.Printf("Warning: failed to start process monitor: %v / 警告：启动进程监控器失败：%v\n", err, err)
	}

	// Step 3: Initialize process discovery (simplified, no auto-scan)
	// 步骤 3：初始化进程发现（简化版，无自动扫描）
	fmt.Println("[3/8] Initializing process discovery... / 初始化进程发现...")

	// Step 4: Start event reporter / 启动事件上报器
	fmt.Println("[4/8] Starting event reporter... / 启动事件上报器...")
	a.eventReporter.Start()

	// Step 5: Register command handlers
	// 步骤 5：注册命令处理器
	fmt.Println("[5/8] Registering command handlers... / 注册命令处理器...")
	a.registerCommandHandlers()

	// Step 6: Connect to Control Plane
	// 步骤 6：连接到 Control Plane
	fmt.Println("[6/8] Connecting to Control Plane... / 连接到 Control Plane...")
	if err := a.connectToControlPlane(); err != nil {
		return fmt.Errorf("failed to connect to Control Plane: %w / 连接 Control Plane 失败：%w", err, err)
	}

	// Step 7: Register with Control Plane
	// 步骤 7：向 Control Plane 注册
	fmt.Println("[7/8] Registering with Control Plane... / 向 Control Plane 注册...")
	if err := a.registerWithControlPlane(); err != nil {
		return fmt.Errorf("failed to register with Control Plane: %w / 向 Control Plane 注册失败：%w", err, err)
	}

	// Step 8: Start background services
	// 步骤 8：启动后台服务
	fmt.Println("[8/8] Starting background services... / 启动后台服务...")
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

// setupProcessMonitor sets up the process monitor with callbacks
// setupProcessMonitor 设置进程监控器的回调
func (a *Agent) setupProcessMonitor() {
	// Set event handler / 设置事件处理器
	a.processMonitor.SetEventHandler(func(event *monitor.ProcessEvent) {
		fmt.Printf("[Agent] Process event: type=%s, name=%s, pid=%d / 进程事件：类型=%s，名称=%s，PID=%d\n",
			event.Type, event.Name, event.PID, event.Type, event.Name, event.PID)

		// Report event to Control Plane via gRPC / 通过 gRPC 向 Control Plane 上报事件
		go a.reportProcessEvent(event)
	})

	// Set crash handler to trigger auto restart / 设置崩溃处理器以触发自动重启
	a.processMonitor.SetCrashHandler(func(proc *monitor.TrackedProcess) {
		fmt.Printf("[Agent] Process crashed: %s (PID: %d), triggering auto restart / 进程崩溃：%s（PID：%d），触发自动重启\n",
			proc.Name, proc.PID, proc.Name, proc.PID)
		if err := a.autoRestarter.OnProcessCrashed(proc); err != nil {
			fmt.Printf("[Agent] Auto restart failed: %v / 自动重启失败：%v\n", err, err)
		}
	})

	// Set restart callback to update PID and report event / 设置重启回调以更新 PID 并上报事件
	a.autoRestarter.SetCallback(func(processName string, success bool, err error) {
		if !success {
			// Report restart failed event / 上报重启失败事件
			proc := a.processMonitor.GetTrackedProcess(processName)
			if proc != nil {
				event := &monitor.ProcessEvent{
					Type:      monitor.EventRestartFailed,
					PID:       proc.PID,
					Name:      processName,
					Timestamp: time.Now(),
					Details: map[string]interface{}{
						"install_dir": proc.InstallDir,
						"role":        proc.Role,
						"error":       err.Error(),
					},
				}
				go a.reportProcessEvent(event)
			}
			return
		}

		// Get new PID from process manager / 从进程管理器获取新 PID
		info, err := a.processManager.GetStatus(a.ctx, processName)
		if err != nil || info.PID <= 0 {
			fmt.Printf("[Agent] Failed to get new PID after restart: %v / 重启后获取新 PID 失败：%v\n", err, err)
			return
		}

		// Update process monitor with new PID / 使用新 PID 更新进程监控器
		a.processMonitor.UpdateProcessPID(processName, info.PID)

		// Get tracked process for details / 获取跟踪进程的详细信息
		proc := a.processMonitor.GetTrackedProcess(processName)
		installDir := ""
		role := ""
		if proc != nil {
			installDir = proc.InstallDir
			role = proc.Role
		}

		// Report restarted event with new PID / 上报带有新 PID 的重启事件
		event := &monitor.ProcessEvent{
			Type:      monitor.EventRestarted,
			PID:       info.PID,
			Name:      processName,
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"install_dir": installDir,
				"role":        role,
			},
		}
		go a.reportProcessEvent(event)

		fmt.Printf("[Agent] Process restarted: %s (new PID: %d) / 进程已重启：%s（新 PID：%d）\n",
			processName, info.PID, processName, info.PID)
	})
}

// reportProcessEvent reports a process event to Control Plane via gRPC.
// reportProcessEvent 通过 gRPC 向 Control Plane 上报进程事件。
func (a *Agent) reportProcessEvent(event *monitor.ProcessEvent) {
	if !a.grpcClient.IsConnected() {
		fmt.Printf("[Agent] Not connected, caching event / 未连接，缓存事件\n")
		a.eventReporter.ReportEvent(event)
		return
	}

	// Convert monitor.ProcessEvent to pb.ProcessEventReport
	// 将 monitor.ProcessEvent 转换为 pb.ProcessEventReport
	var eventType pb.ProcessEventType
	switch event.Type {
	case monitor.EventStarted:
		eventType = pb.ProcessEventType_PROCESS_STARTED
	case monitor.EventStopped:
		eventType = pb.ProcessEventType_PROCESS_STOPPED
	case monitor.EventCrashed:
		eventType = pb.ProcessEventType_PROCESS_CRASHED
	case monitor.EventRestarted:
		eventType = pb.ProcessEventType_PROCESS_RESTARTED
	case monitor.EventRestartFailed:
		eventType = pb.ProcessEventType_PROCESS_RESTART_FAILED
	}

	installDir := ""
	role := ""
	if event.Details != nil {
		if dir, ok := event.Details["install_dir"].(string); ok {
			installDir = dir
		}
		if r, ok := event.Details["role"].(string); ok {
			role = r
		}
	}

	report := &pb.ProcessEventReport{
		AgentId:     a.grpcClient.GetAgentID(),
		EventType:   eventType,
		Pid:         int32(event.PID),
		ProcessName: event.Name,
		InstallDir:  installDir,
		Role:        role,
		Timestamp:   event.Timestamp.UnixMilli(),
	}

	if err := a.grpcClient.ReportProcessEvent(a.ctx, report); err != nil {
		fmt.Printf("[Agent] Failed to report event, caching: %v / 上报事件失败，缓存：%v\n", err, err)
		a.eventReporter.ReportEvent(event)
		return
	}

	fmt.Printf("[Agent] Event reported to Control Plane: type=%s, name=%s, pid=%d / 事件已上报到 Control Plane：类型=%s，名称=%s，PID=%d\n",
		event.Type, event.Name, event.PID, event.Type, event.Name, event.PID)
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

	// Save the assigned agent ID / 保存分配的 Agent ID
	a.grpcClient.SetAgentID(resp.AssignedId)

	fmt.Printf("Registered successfully with ID: %s / 注册成功，ID：%s\n", resp.AssignedId, resp.AssignedId)

	// Set up event reporter with gRPC report function / 设置事件上报器的 gRPC 上报函数
	a.setupEventReporter()

	// Apply configuration from Control Plane if provided
	// 如果提供，应用来自 Control Plane 的配置
	if resp.Config != nil {
		a.applyRemoteConfig(resp.Config)
	}

	return nil
}

// setupEventReporter sets up the event reporter with gRPC report function.
// setupEventReporter 设置事件上报器的 gRPC 上报函数。
func (a *Agent) setupEventReporter() {
	agentID := a.grpcClient.GetAgentID()

	// Create a report function that sends events via gRPC
	// 创建一个通过 gRPC 发送事件的上报函数
	reportFunc := func(events []*monitor.ProcessEvent) error {
		for _, event := range events {
			// Convert monitor.ProcessEvent to pb.ProcessEventReport
			// 将 monitor.ProcessEvent 转换为 pb.ProcessEventReport
			var eventType pb.ProcessEventType
			switch event.Type {
			case monitor.EventStarted:
				eventType = pb.ProcessEventType_PROCESS_STARTED
			case monitor.EventStopped:
				eventType = pb.ProcessEventType_PROCESS_STOPPED
			case monitor.EventCrashed:
				eventType = pb.ProcessEventType_PROCESS_CRASHED
			case monitor.EventRestarted:
				eventType = pb.ProcessEventType_PROCESS_RESTARTED
			case monitor.EventRestartFailed:
				eventType = pb.ProcessEventType_PROCESS_RESTART_FAILED
			}

			installDir := ""
			role := ""
			if event.Details != nil {
				if dir, ok := event.Details["install_dir"].(string); ok {
					installDir = dir
				}
				if r, ok := event.Details["role"].(string); ok {
					role = r
				}
			}

			report := &pb.ProcessEventReport{
				AgentId:     agentID,
				EventType:   eventType,
				Pid:         int32(event.PID),
				ProcessName: event.Name,
				InstallDir:  installDir,
				Role:        role,
				Timestamp:   event.Timestamp.UnixMilli(),
			}

			if err := a.grpcClient.ReportProcessEvent(a.ctx, report); err != nil {
				fmt.Printf("[Agent] Failed to report event: %v / 上报事件失败：%v\n", err, err)
				return err
			}
			fmt.Printf("[Agent] Event reported: type=%s, name=%s, pid=%d / 事件已上报：类型=%s，名称=%s，PID=%d\n",
				event.Type, event.Name, event.PID, event.Type, event.Name, event.PID)
		}
		return nil
	}

	// Note: EventReporter doesn't have a SetReportFunc method, so we need to recreate it
	// 注意：EventReporter 没有 SetReportFunc 方法，所以我们需要重新创建它
	// For now, we'll report events directly in the event handler
	// 目前，我们将在事件处理器中直接上报事件
	_ = reportFunc // Suppress unused variable warning / 抑制未使用变量警告
	fmt.Println("[Agent] Event reporter configured / 事件上报器已配置")
}

// applyRemoteConfig applies configuration received from Control Plane
// applyRemoteConfig 应用从 Control Plane 接收的配置
func (a *Agent) applyRemoteConfig(cfg *pb.AgentConfig) {
	fmt.Printf("Received remote config from Control Plane: HeartbeatInterval=%d seconds / 收到来自 Control Plane 的远程配置：HeartbeatInterval=%d 秒\n", cfg.HeartbeatInterval, cfg.HeartbeatInterval)
	fmt.Printf("Current local heartbeat interval: %v / 当前本地心跳间隔：%v\n", a.config.Heartbeat.Interval, a.config.Heartbeat.Interval)

	configChanged := false

	if cfg.HeartbeatInterval > 0 {
		newInterval := time.Duration(cfg.HeartbeatInterval) * time.Second
		if a.config.Heartbeat.Interval != newInterval {
			a.config.Heartbeat.Interval = newInterval
			configChanged = true
			fmt.Printf("Applied heartbeat interval from Control Plane: %v / 已应用来自 Control Plane 的心跳间隔：%v\n", newInterval, newInterval)
		}
	} else {
		fmt.Printf("Remote HeartbeatInterval is 0 or negative, keeping local config / 远程 HeartbeatInterval 为 0 或负数，保持本地配置\n")
	}

	// Persist config changes to local file / 将配置变更持久化到本地文件
	if configChanged {
		if err := a.persistConfigToFile(); err != nil {
			fmt.Printf("Warning: Failed to persist config to file: %v / 警告：持久化配置到文件失败：%v\n", err, err)
		} else {
			fmt.Printf("Config persisted to local file / 配置已持久化到本地文件\n")
		}
	}
}

// persistConfigToFile saves the current config to the local config file
// persistConfigToFile 将当前配置保存到本地配置文件
func (a *Agent) persistConfigToFile() error {
	configPath := config.DefaultConfigPath

	// Read existing config file content
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Update heartbeat interval in the config content
	// Format: "interval: 10s" -> "interval: 60s"
	lines := strings.Split(string(content), "\n")
	inHeartbeatSection := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Detect heartbeat section
		if strings.Contains(strings.ToLower(trimmed), "heartbeat") {
			inHeartbeatSection = true
			continue
		}
		// Detect next section (non-indented line that's not empty or comment)
		if inHeartbeatSection && len(trimmed) > 0 && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inHeartbeatSection = false
		}
		// Update interval in heartbeat section
		if inHeartbeatSection && strings.HasPrefix(trimmed, "interval:") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = fmt.Sprintf("%sinterval: %s", indent, a.config.Heartbeat.Interval.String())
			fmt.Printf("Updated config line: %s\n", lines[i])
			break
		}
	}

	// Write back to file
	newContent := strings.Join(lines, "\n")
	if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
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
	usage, _ := a.metricsCollector.Collect()

	// Collect tracked process status / 采集跟踪进程状态
	trackedProcesses := a.processMonitor.GetAllTrackedProcesses()
	processes := make([]*pb.ProcessStatus, 0, len(trackedProcesses))
	for _, proc := range trackedProcesses {
		// Check if process is alive and get metrics / 检查进程是否存活并获取指标
		status := "stopped"
		if isProcessAlive(proc.PID) {
			status = "running"
		}
		cpuUsage, memUsage := getProcessMetrics(proc.PID)

		processes = append(processes, &pb.ProcessStatus{
			Name:        proc.Name,
			Pid:         int32(proc.PID),
			Status:      status,
			CpuUsage:    cpuUsage,
			MemoryUsage: memUsage,
		})
	}

	_, err := a.grpcClient.SendHeartbeat(a.ctx, usage, processes)
	if err != nil {
		fmt.Printf("Heartbeat failed: %v / 心跳失败：%v\n", err, err)

		// Check if agent needs to re-register (Control Plane restarted)
		// 检查是否需要重新注册（Control Plane 重启了）
		if isNotFoundError(err) {
			fmt.Println("Agent not found on Control Plane, re-registering... / Agent 在 Control Plane 上未找到，重新注册...")
			go func() {
				if regErr := a.registerWithControlPlane(); regErr != nil {
					fmt.Printf("Re-registration failed: %v / 重新注册失败：%v\n", regErr, regErr)
				}
			}()
		}
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

			// Check if agent needs to re-register (Control Plane restarted)
			// 检查是否需要重新注册（Control Plane 重启了）
			if isNotFoundError(err) {
				fmt.Println("Agent not found on Control Plane, re-registering... / Agent 在 Control Plane 上未找到，重新注册...")
				if regErr := a.registerWithControlPlane(); regErr != nil {
					fmt.Printf("Re-registration failed: %v / 重新注册失败：%v\n", regErr, regErr)
				}
			}

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

	// Register cluster discovery and monitoring handlers / 注册集群发现和监控处理器
	a.executor.RegisterHandler(pb.CommandType_DISCOVER_CLUSTERS, a.handleDiscoverClustersCommand)
	a.executor.RegisterHandler(pb.CommandType_UPDATE_MONITOR_CONFIG, a.handleUpdateMonitorConfigCommand)

	// Initialize plugin manager and register plugin handlers / 初始化插件管理器并注册插件处理器
	executor.InitPluginManager(a.config.SeaTunnel.InstallDir)
	executor.RegisterPluginHandlers(a.executor)

	// Register package transfer handlers / 注册安装包传输处理器
	executor.RegisterPackageHandlers(a.executor)

	// Register config handlers / 注册配置处理器
	configHandlers := executor.NewConfigHandlers()
	configHandlers.RegisterHandlers(a.executor)

	fmt.Printf("Registered %d command handlers / 已注册 %d 个命令处理器\n",
		len(a.executor.GetRegisteredTypes()), len(a.executor.GetRegisteredTypes()))
}

// Command handler implementations / 命令处理器实现

func (a *Agent) handlePrecheckCommand(ctx context.Context, cmd *pb.CommandRequest, reporter executor.ProgressReporter) (*pb.CommandResponse, error) {
	// Check if sub_command is specified for specific precheck operations
	// 检查是否指定了 sub_command 用于特定的预检查操作
	subCommand := cmd.Parameters["sub_command"]
	if subCommand != "" && subCommand != "full" {
		// Delegate to specific precheck handlers
		// 委托给特定的预检查处理器
		return executor.HandlePrecheckCommand(ctx, cmd, reporter)
	}

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
		ClusterID:      getParamString(cmd.Parameters, "cluster_id", ""),
	}

	// Parse JVM config / 解析 JVM 配置
	jvmHybridHeap := getParamInt(cmd.Parameters, "jvm_hybrid_heap", 0)
	jvmMasterHeap := getParamInt(cmd.Parameters, "jvm_master_heap", 0)
	jvmWorkerHeap := getParamInt(cmd.Parameters, "jvm_worker_heap", 0)
	fmt.Printf("[Install] JVM params received: hybrid=%d, master=%d, worker=%d\n", jvmHybridHeap, jvmMasterHeap, jvmWorkerHeap)
	if jvmHybridHeap > 0 || jvmMasterHeap > 0 || jvmWorkerHeap > 0 {
		params.JVM = &installer.JVMConfig{
			HybridHeapSize: jvmHybridHeap,
			MasterHeapSize: jvmMasterHeap,
			WorkerHeapSize: jvmWorkerHeap,
		}
		fmt.Printf("[Install] JVM config created: %+v\n", params.JVM)
	} else {
		fmt.Println("[Install] JVM config not created (all values are 0)")
	}

	// Parse checkpoint config / 解析检查点配置
	checkpointStorageType := getParamString(cmd.Parameters, "checkpoint_storage_type", "")
	checkpointNamespace := getParamString(cmd.Parameters, "checkpoint_namespace", "")
	if checkpointStorageType != "" {
		params.Checkpoint = &installer.CheckpointConfig{
			StorageType: installer.CheckpointStorageType(checkpointStorageType),
			Namespace:   checkpointNamespace,
		}
		// HDFS config
		hdfsHost := getParamString(cmd.Parameters, "checkpoint_hdfs_host", "")
		if hdfsHost != "" {
			params.Checkpoint.HDFSNameNodeHost = hdfsHost
			params.Checkpoint.HDFSNameNodePort = getParamInt(cmd.Parameters, "checkpoint_hdfs_port", 8020)
		}
		// HDFS Kerberos config / HDFS Kerberos 配置
		kerberosPrincipal := getParamString(cmd.Parameters, "checkpoint_kerberos_principal", "")
		if kerberosPrincipal != "" {
			params.Checkpoint.KerberosPrincipal = kerberosPrincipal
		}
		kerberosKeytabPath := getParamString(cmd.Parameters, "checkpoint_kerberos_keytab_path", "")
		if kerberosKeytabPath != "" {
			params.Checkpoint.KerberosKeytabFilePath = kerberosKeytabPath
		}
		// HDFS HA config / HDFS HA 配置
		hdfsHAEnabled := getParamString(cmd.Parameters, "checkpoint_hdfs_ha_enabled", "")
		if hdfsHAEnabled == "true" {
			params.Checkpoint.HDFSHAEnabled = true
			params.Checkpoint.HDFSNameServices = getParamString(cmd.Parameters, "checkpoint_hdfs_name_services", "")
			params.Checkpoint.HDFSHANamenodes = getParamString(cmd.Parameters, "checkpoint_hdfs_ha_namenodes", "")
			params.Checkpoint.HDFSNamenodeRPCAddress1 = getParamString(cmd.Parameters, "checkpoint_hdfs_namenode_rpc_address_1", "")
			params.Checkpoint.HDFSNamenodeRPCAddress2 = getParamString(cmd.Parameters, "checkpoint_hdfs_namenode_rpc_address_2", "")
			params.Checkpoint.HDFSFailoverProxyProvider = getParamString(cmd.Parameters, "checkpoint_hdfs_failover_proxy_provider", "")
		}
		// OSS/S3 config
		storageEndpoint := getParamString(cmd.Parameters, "checkpoint_storage_endpoint", "")
		if storageEndpoint != "" {
			params.Checkpoint.StorageEndpoint = storageEndpoint
			params.Checkpoint.StorageBucket = getParamString(cmd.Parameters, "checkpoint_storage_bucket", "")
			params.Checkpoint.StorageAccessKey = getParamString(cmd.Parameters, "checkpoint_storage_access_key", "")
			params.Checkpoint.StorageSecretKey = getParamString(cmd.Parameters, "checkpoint_storage_secret_key", "")
		}
		fmt.Printf("[Install] Checkpoint config created: type=%s, namespace=%s\n", checkpointStorageType, checkpointNamespace)
	}

	// Parse master addresses / 解析 master 地址列表
	masterAddressesStr := getParamString(cmd.Parameters, "master_addresses", "")
	if masterAddressesStr != "" {
		params.MasterAddresses = strings.Split(masterAddressesStr, ",")
	}

	// Parse worker addresses (for separated mode) / 解析 worker 地址列表（分离模式）
	workerAddressesStr := getParamString(cmd.Parameters, "worker_addresses", "")
	if workerAddressesStr != "" {
		params.WorkerAddresses = strings.Split(workerAddressesStr, ",")
	}

	// Parse install mode / 解析安装模式
	installMode := getParamString(cmd.Parameters, "install_mode", "online")
	if installMode == "offline" {
		params.Mode = installer.InstallModeOffline
	} else {
		params.Mode = installer.InstallModeOnline
	}

	// Parse package path (from gRPC transfer or local) / 解析安装包路径（来自 gRPC 传输或本地）
	packagePath := getParamString(cmd.Parameters, "package_path", "")
	if packagePath != "" {
		params.PackagePath = packagePath
		params.Mode = installer.InstallModeOffline // Use offline mode when package path is provided
	}

	// Parse mirror source / 解析镜像源
	mirror := getParamString(cmd.Parameters, "mirror", "")
	if mirror != "" {
		params.Mirror = installer.MirrorSource(mirror)
	}

	// Create progress adapter / 创建进度适配器
	installReporter := &installerProgressAdapter{
		reporter:  reporter,
		commandID: cmd.CommandId,
	}

	// Use InstallStepByStep for complete installation including JVM configuration
	// 使用 InstallStepByStep 进行完整安装，包括 JVM 配置
	result, err := a.installerManager.InstallStepByStep(ctx, params, installReporter)
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

	// Use InstallStepByStep for complete installation including JVM configuration
	// 使用 InstallStepByStep 进行完整安装，包括 JVM 配置
	_, err := a.installerManager.InstallStepByStep(ctx, params, installReporter)
	if err != nil {
		return executor.CreateErrorResponse(cmd.CommandId, fmt.Sprintf("Install failed: %v / 安装失败：%v", err, err)), err
	}

	return executor.CreateSuccessResponse(cmd.CommandId, "Upgrade completed / 升级完成"), nil
}

func (a *Agent) handleStartCommand(ctx context.Context, cmd *pb.CommandRequest, reporter executor.ProgressReporter) (*pb.CommandResponse, error) {
	reporter.Report(10, "Starting SeaTunnel process... / 启动 SeaTunnel 进程...")

	role := getParamString(cmd.Parameters, "role", "")
	installDir := getParamString(cmd.Parameters, "install_dir", a.config.SeaTunnel.InstallDir)

	// Use role as process name for tracking / 使用角色作为进程名进行跟踪
	// For hybrid mode (empty, "hybrid", or "master/worker"), use "seatunnel"
	// 对于混合模式（空、"hybrid" 或 "master/worker"），使用 "seatunnel"
	processName := "seatunnel"
	if role != "" && role != "hybrid" && role != "master/worker" {
		processName = "seatunnel-" + role
	}

	params := &process.StartParams{
		InstallDir: installDir,
		Role:       role,
		ConfigDir:  getParamString(cmd.Parameters, "config_dir", ""),
		LogDir:     getParamString(cmd.Parameters, "log_dir", ""),
	}

	// Check if auto-restart is enabled to avoid conflict
	// 检查是否启用了自动重启以避免冲突
	if a.autoRestarter.IsEnabled() {
		// Auto-restart enabled: register process with PID=0, let auto-restarter handle the actual start
		// 自动重启已启用：用 PID=0 注册进程，让自动重启器处理实际的启动
		a.processMonitor.TrackProcessSilent(processName, 0, installDir, role, params)
		a.autoRestarter.ResetRestartCount(processName)
		fmt.Printf("[Agent] Process registered for auto-start: %s (auto-restart will handle startup) / 进程已注册等待自动启动：%s（自动重启将处理启动）\n",
			processName, processName)
		reporter.Report(100, "Process registered for auto-start / 进程已注册等待自动启动")
		return executor.CreateSuccessResponse(cmd.CommandId, fmt.Sprintf("Process registered for auto-start (role: %s) / 进程已注册等待自动启动（角色：%s）", role, role)), nil
	}

	// Auto-restart disabled: start process directly
	// 自动重启已禁用：直接启动进程
	err := a.processManager.StartProcess(ctx, processName, params)
	if err != nil {
		return executor.CreateErrorResponse(cmd.CommandId, err.Error()), err
	}

	// Track the process / 跟踪进程
	// Get the PID from process manager / 从进程管理器获取 PID
	if info, err := a.processManager.GetStatus(ctx, processName); err == nil && info.PID > 0 {
		a.processMonitor.TrackProcess(processName, info.PID, installDir, role, params)
		fmt.Printf("[Agent] Process started and tracked: %s (PID: %d) / 进程已启动并跟踪：%s（PID：%d）\n",
			processName, info.PID, processName, info.PID)
	}

	reporter.Report(100, "Process started / 进程已启动")
	return executor.CreateSuccessResponse(cmd.CommandId, fmt.Sprintf("Process started successfully (role: %s) / 进程启动成功（角色：%s）", role, role)), nil
}

func (a *Agent) handleStopCommand(ctx context.Context, cmd *pb.CommandRequest, reporter executor.ProgressReporter) (*pb.CommandResponse, error) {
	reporter.Report(10, "Stopping SeaTunnel process... / 停止 SeaTunnel 进程...")

	role := getParamString(cmd.Parameters, "role", "")
	installDir := getParamString(cmd.Parameters, "install_dir", a.config.SeaTunnel.InstallDir)
	graceful := getParamBool(cmd.Parameters, "graceful", true)

	// Use role as process name for tracking / 使用角色作为进程名进行跟踪
	// For hybrid mode (empty, "hybrid", or "master/worker"), use "seatunnel"
	// 对于混合模式（空、"hybrid" 或 "master/worker"），使用 "seatunnel"
	processName := "seatunnel"
	if role != "" && role != "hybrid" && role != "master/worker" {
		processName = "seatunnel-" + role
	}

	params := &process.StopParams{
		Graceful:   graceful,
		Timeout:    30 * time.Second,
		InstallDir: installDir,
		Role:       role,
	}

	err := a.processManager.StopProcess(ctx, processName, params)
	if err != nil {
		return executor.CreateErrorResponse(cmd.CommandId, err.Error()), err
	}

	// Check if auto-restart is enabled to decide whether to untrack or set PID=0
	// 检查是否启用了自动重启，以决定是取消跟踪还是设置 PID=0
	if a.autoRestarter.IsEnabled() {
		// Auto-restart enabled: set PID=0, auto-restart will start it again
		// 自动重启已启用：设置 PID=0，自动重启会重新启动它
		a.processMonitor.UpdateProcessPID(processName, 0)
		fmt.Printf("[Agent] Process stopped, PID set to 0 (auto-restart enabled): %s / 进程已停止，PID 设为 0（自动重启已启用）：%s\n",
			processName, processName)
	} else {
		// Auto-restart disabled: untrack the process completely
		// 自动重启已禁用：完全取消跟踪进程
		a.processMonitor.UntrackProcess(processName)
		fmt.Printf("[Agent] Process stopped and untracked (auto-restart disabled): %s / 进程已停止并取消跟踪（自动重启已禁用）：%s\n",
			processName, processName)
	}

	reporter.Report(100, "Process stopped / 进程已停止")
	return executor.CreateSuccessResponse(cmd.CommandId, fmt.Sprintf("Process stopped successfully (role: %s) / 进程停止成功（角色：%s）", role, role)), nil
}

func (a *Agent) handleRestartCommand(ctx context.Context, cmd *pb.CommandRequest, reporter executor.ProgressReporter) (*pb.CommandResponse, error) {
	reporter.Report(10, "Restarting SeaTunnel process... / 重启 SeaTunnel 进程...")

	role := getParamString(cmd.Parameters, "role", "")
	installDir := getParamString(cmd.Parameters, "install_dir", a.config.SeaTunnel.InstallDir)

	// Use role as process name for tracking / 使用角色作为进程名进行跟踪
	// For hybrid mode (empty, "hybrid", or "master/worker"), use "seatunnel"
	// 对于混合模式（空、"hybrid" 或 "master/worker"），使用 "seatunnel"
	processName := "seatunnel"
	if role != "" && role != "hybrid" && role != "master/worker" {
		processName = "seatunnel-" + role
	}

	startParams := &process.StartParams{
		InstallDir: installDir,
		Role:       role,
	}
	stopParams := &process.StopParams{
		Graceful:   true,
		Timeout:    30 * time.Second,
		InstallDir: installDir,
		Role:       role,
	}

	// Check if auto-restart is enabled to avoid conflict
	// 检查是否启用了自动重启以避免冲突
	if a.autoRestarter.IsEnabled() {
		// Auto-restart enabled: stop process, then set PID=0 to let auto-restarter handle the start
		// 自动重启已启用：停止进程，然后设置 PID=0 让自动重启器处理启动
		err := a.processManager.StopProcess(ctx, processName, stopParams)
		if err != nil {
			return executor.CreateErrorResponse(cmd.CommandId, err.Error()), err
		}

		// Register process with PID=0, auto-restarter will start it
		// 用 PID=0 注册进程，自动重启器会启动它
		a.processMonitor.TrackProcessSilent(processName, 0, installDir, role, startParams)
		a.autoRestarter.ResetRestartCount(processName)
		fmt.Printf("[Agent] Process stopped, registered for auto-restart: %s / 进程已停止，已注册等待自动重启：%s\n",
			processName, processName)
		reporter.Report(100, "Process stopped, auto-restart will start it / 进程已停止，自动重启将启动它")
		return executor.CreateSuccessResponse(cmd.CommandId, fmt.Sprintf("Process stopped, auto-restart will start it (role: %s) / 进程已停止，自动重启将启动它（角色：%s）", role, role)), nil
	}

	// Auto-restart disabled: restart process directly
	// 自动重启已禁用：直接重启进程
	err := a.processManager.RestartProcess(ctx, processName, startParams, stopParams)
	if err != nil {
		return executor.CreateErrorResponse(cmd.CommandId, err.Error()), err
	}

	// Update tracking with new PID / 使用新 PID 更新跟踪
	if info, err := a.processManager.GetStatus(ctx, processName); err == nil && info.PID > 0 {
		a.processMonitor.TrackProcess(processName, info.PID, installDir, role, startParams)
		fmt.Printf("[Agent] Process restarted and tracked: %s (PID: %d) / 进程已重启并跟踪：%s（PID：%d）\n",
			processName, info.PID, processName, info.PID)
	}

	reporter.Report(100, "Process restarted / 进程已重启")
	return executor.CreateSuccessResponse(cmd.CommandId, fmt.Sprintf("Process restarted successfully (role: %s) / 进程重启成功（角色：%s）", role, role)), nil
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

	logFile := getParamString(cmd.Parameters, "log_file", "")
	if logFile == "" {
		return executor.CreateErrorResponse(cmd.CommandId, "log_file parameter is required / 需要 log_file 参数"), nil
	}

	// Parameters / 参数
	lines := getParamInt(cmd.Parameters, "lines", 100)
	if lines <= 0 {
		lines = 100
	}
	if lines > 2000 {
		lines = 2000 // Max 2000 lines / 最多 2000 行
	}

	// mode: "tail" (default), "head", "all"
	// mode: "tail"（默认）, "head", "all"
	mode := getParamString(cmd.Parameters, "mode", "tail")

	// filter: grep pattern (optional)
	// filter: grep 过滤模式（可选）
	filter := getParamString(cmd.Parameters, "filter", "")

	// date: specific date for rolling log files (e.g., "2025-12-18")
	// date: 滚动日志文件的特定日期（如 "2025-12-18"）
	date := getParamString(cmd.Parameters, "date", "")

	// If date is specified, try to find the dated log file
	// 如果指定了日期，尝试查找带日期的日志文件
	actualLogFile := logFile
	if date != "" {
		// Try common rolling log patterns / 尝试常见的滚动日志模式
		// Pattern 1: seatunnel-engine-master.log.2025-12-18
		// Pattern 2: seatunnel-engine-worker.log.2025-11-12-1 (with sequence number)
		// User may input full suffix like "2025-11-12-1" directly
		// 用户可能直接输入完整后缀如 "2025-11-12-1"
		datedFile := logFile + "." + date
		if _, err := os.Stat(datedFile); err == nil {
			actualLogFile = datedFile
		} else {
			// Try glob pattern to find matching files / 尝试 glob 模式查找匹配文件
			// This handles cases like user input "2025-11-12" and we find "2025-11-12-1"
			// 这处理用户输入 "2025-11-12" 而我们找到 "2025-11-12-1" 的情况
			matches, _ := filepath.Glob(logFile + "." + date + "*")
			if len(matches) > 0 {
				// Use the first match (or latest if multiple)
				// 使用第一个匹配（如果有多个则使用最新的）
				actualLogFile = matches[len(matches)-1]
			} else {
				// Still not found, keep the dated file path for error message
				// 仍未找到，保留带日期的文件路径用于错误消息
				actualLogFile = datedFile
			}
		}
	}

	// Check if file exists / 检查文件是否存在
	if _, err := os.Stat(actualLogFile); os.IsNotExist(err) {
		// List available log files / 列出可用的日志文件
		dir := filepath.Dir(logFile)
		base := filepath.Base(logFile)
		files, _ := filepath.Glob(filepath.Join(dir, base+"*"))
		availableFiles := strings.Join(files, ", ")
		return executor.CreateErrorResponse(cmd.CommandId, fmt.Sprintf("Log file not found: %s. Available files: %s / 日志文件不存在: %s。可用文件: %s", actualLogFile, availableFiles, actualLogFile, availableFiles)), nil
	}

	var output []byte
	var err error

	switch mode {
	case "head":
		// Read first N lines / 读取前 N 行
		headCmd := exec.CommandContext(ctx, "head", "-n", strconv.Itoa(lines), actualLogFile)
		output, err = headCmd.Output()
		if err != nil {
			output, err = readFirstNLines(actualLogFile, lines)
		}
	case "all":
		// Read entire file (limited) / 读取整个文件（有限制）
		output, err = os.ReadFile(actualLogFile)
		if err == nil && len(output) > 500*1024 {
			// Limit to 500KB / 限制为 500KB
			output = output[:500*1024]
			output = append(output, []byte("\n... [truncated / 已截断] ...")...)
		}
	default: // tail
		// Read last N lines / 读取最后 N 行
		tailCmd := exec.CommandContext(ctx, "tail", "-n", strconv.Itoa(lines), actualLogFile)
		output, err = tailCmd.Output()
		if err != nil {
			output, err = readLastNLines(actualLogFile, lines)
		}
	}

	if err != nil {
		return executor.CreateErrorResponse(cmd.CommandId, fmt.Sprintf("Failed to read log file: %v / 读取日志文件失败: %v", err, err)), nil
	}

	// Apply filter if specified / 如果指定了过滤器则应用
	if filter != "" {
		filteredLines := []string{}
		for _, line := range strings.Split(string(output), "\n") {
			if strings.Contains(line, filter) {
				filteredLines = append(filteredLines, line)
			}
		}
		output = []byte(strings.Join(filteredLines, "\n"))
	}

	reporter.Report(100, "Logs collected / 日志收集完成")
	return executor.CreateSuccessResponse(cmd.CommandId, string(output)), nil
}

// readLastNLines reads the last N lines from a file
// readLastNLines 从文件中读取最后 N 行
func readLastNLines(filename string, n int) ([]byte, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")
	startIdx := len(lines) - n
	if startIdx < 0 {
		startIdx = 0
	}
	return []byte(strings.Join(lines[startIdx:], "\n")), nil
}

// readFirstNLines reads the first N lines from a file
// readFirstNLines 从文件中读取前 N 行
func readFirstNLines(filename string, n int) ([]byte, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return []byte(strings.Join(lines, "\n")), nil
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
	fmt.Println("[1/6] Stopping command acceptance... / 停止接受命令...")

	// Step 2: Stop process monitor / 停止进程监控器
	fmt.Println("[2/6] Stopping process monitor... / 停止进程监控器...")
	if err := a.processMonitor.Stop(); err != nil {
		fmt.Printf("Warning: Error stopping process monitor: %v / 警告：停止进程监控器时出错：%v\n", err, err)
	}

	// Step 3: Stop event reporter / 停止事件上报器
	fmt.Println("[3/6] Stopping event reporter... / 停止事件上报器...")
	a.eventReporter.Stop()

	// Step 4: Wait for running tasks to complete (with timeout)
	// 步骤 4：等待运行中的任务完成（带超时）
	fmt.Println("[4/6] Waiting for running tasks... / 等待运行中的任务...")
	// Note: The executor handles task completion internally
	// 注意：执行器内部处理任务完成

	// Step 5: Keep SeaTunnel processes running (do NOT stop them)
	// 步骤 5：保持 SeaTunnel 进程运行（不停止它们）
	// Agent restart should not affect running SeaTunnel processes
	// Agent 重启不应影响正在运行的 SeaTunnel 进程
	fmt.Println("[5/6] Keeping SeaTunnel processes running... / 保持 SeaTunnel 进程运行...")
	// Note: We intentionally do NOT call processManager.StopAll() here
	// 注意：我们故意不在这里调用 processManager.StopAll()

	// Stop process manager (just cleanup internal state, not the processes)
	// 停止进程管理器（只清理内部状态，不停止进程）
	if err := a.processManager.Stop(); err != nil {
		fmt.Printf("Warning: Error stopping process manager: %v / 警告：停止进程管理器时出错：%v\n", err, err)
	}

	// Step 6: Close gRPC connection
	// 步骤 6：关闭 gRPC 连接
	fmt.Println("[6/6] Closing connections... / 关闭连接...")
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

// isNotFoundError checks if the error indicates agent not found on Control Plane
// isNotFoundError 检查错误是否表示 Agent 在 Control Plane 上未找到
// This typically happens when Control Plane restarts and loses in-memory agent state
// 这通常发生在 Control Plane 重启并丢失内存中的 Agent 状态时
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "NotFound") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "not registered") ||
		strings.Contains(errStr, "re-register")
}

// handleDiscoverClustersCommand handles the DISCOVER_CLUSTERS command (simplified)
// handleDiscoverClustersCommand 处理 DISCOVER_CLUSTERS 命令（简化版）
// Only scans for running SeaTunnel processes, returns PID, role, install_dir
// 只扫描运行中的 SeaTunnel 进程，返回 PID、角色、安装目录
func (a *Agent) handleDiscoverClustersCommand(ctx context.Context, cmd *pb.CommandRequest, reporter executor.ProgressReporter) (*pb.CommandResponse, error) {
	reporter.Report(10, "Scanning for SeaTunnel processes... / 正在扫描 SeaTunnel 进程...")

	// Use simplified process discovery / 使用简化的进程发现
	processDiscovery := discovery.NewProcessDiscovery()
	processes, err := processDiscovery.DiscoverProcesses()
	if err != nil {
		return executor.CreateErrorResponse(cmd.CommandId, err.Error()), err
	}

	reporter.Report(80, fmt.Sprintf("Found %d process(es) / 发现 %d 个进程", len(processes), len(processes)))

	// Format output as JSON for easier parsing / 格式化输出为 JSON 以便于解析
	// Include all discovered fields: PID, role, install_dir, version, hazelcast_port, api_port
	// 包含所有发现的字段：PID、角色、安装目录、版本、hazelcast端口、api端口
	type ProcessInfo struct {
		PID           int    `json:"pid"`
		Role          string `json:"role"`
		InstallDir    string `json:"install_dir"`
		Version       string `json:"version"`
		HazelcastPort int    `json:"hazelcast_port"`
		APIPort       int    `json:"api_port"`
	}
	type DiscoveryResult struct {
		Success   bool          `json:"success"`
		Message   string        `json:"message"`
		Processes []ProcessInfo `json:"processes"`
	}

	result := DiscoveryResult{
		Success:   true,
		Message:   fmt.Sprintf("Discovered %d SeaTunnel process(es) / 发现 %d 个 SeaTunnel 进程", len(processes), len(processes)),
		Processes: make([]ProcessInfo, 0, len(processes)),
	}
	for _, p := range processes {
		result.Processes = append(result.Processes, ProcessInfo{
			PID:           p.PID,
			Role:          p.Role,
			InstallDir:    p.InstallDir,
			Version:       p.Version,
			HazelcastPort: p.HazelcastPort,
			APIPort:       p.APIPort,
		})
	}

	jsonOutput, err := json.Marshal(result)
	if err != nil {
		return executor.CreateErrorResponse(cmd.CommandId, err.Error()), err
	}

	reporter.Report(100, "Process discovery completed / 进程发现完成")
	return executor.CreateSuccessResponse(cmd.CommandId, string(jsonOutput)), nil
}

// handleUpdateMonitorConfigCommand handles the UPDATE_MONITOR_CONFIG command
// handleUpdateMonitorConfigCommand 处理 UPDATE_MONITOR_CONFIG 命令
// Requirements 5.5: Apply new config immediately without restart
// 需求 5.5：立即应用新配置，无需重启
func (a *Agent) handleUpdateMonitorConfigCommand(ctx context.Context, cmd *pb.CommandRequest, reporter executor.ProgressReporter) (*pb.CommandResponse, error) {
	reporter.Report(10, "Updating monitor config... / 更新监控配置...")

	// Parse config from parameters / 从参数解析配置
	autoRestartEnabled := getParamBool(cmd.Parameters, "auto_restart", true)
	config := &restart.RestartConfig{
		Enabled:        autoRestartEnabled,
		RestartDelay:   time.Duration(getParamInt(cmd.Parameters, "restart_delay", 10)) * time.Second,
		MaxRestarts:    getParamInt(cmd.Parameters, "max_restarts", 3),
		TimeWindow:     time.Duration(getParamInt(cmd.Parameters, "time_window", 300)) * time.Second,
		CooldownPeriod: time.Duration(getParamInt(cmd.Parameters, "cooldown_period", 1800)) * time.Second,
	}

	// Update auto restarter config / 更新自动重启器配置
	a.autoRestarter.UpdateConfig(config)

	// Update process monitor interval if specified / 如果指定则更新进程监控间隔
	if monitorInterval := getParamInt(cmd.Parameters, "monitor_interval", 0); monitorInterval > 0 {
		a.processMonitor.SetMonitorInterval(time.Duration(monitorInterval) * time.Second)
	}

	// If auto-restart is disabled, untrack all processes immediately
	// 如果禁用了自动重启，静默取消跟踪所有进程（不发送事件，因为进程仍在运行）
	if !autoRestartEnabled {
		trackedProcesses := a.processMonitor.GetAllTrackedProcesses()
		for _, proc := range trackedProcesses {
			// Use silent untrack - process is still running, we just stop monitoring
			// 使用静默取消跟踪 - 进程仍在运行，我们只是停止监控
			a.processMonitor.UntrackProcessSilent(proc.Name)
			fmt.Printf("[Agent] Auto-restart disabled, stopped monitoring process: %s / 自动重启已禁用，停止监控进程：%s\n",
				proc.Name, proc.Name)
		}
		reporter.Report(100, "Monitor config updated (auto-restart disabled, stopped monitoring) / 监控配置已更新（自动重启已禁用，停止监控）")
		return executor.CreateSuccessResponse(cmd.CommandId, "Monitor config updated, auto-restart disabled / 监控配置已更新，自动重启已禁用"), nil
	}

	// Auto-restart is enabled, parse and track processes from Control Plane
	// 自动重启已启用，解析并跟踪来自 Control Plane 的进程
	trackedProcessesJSON := getParamString(cmd.Parameters, "tracked_processes", "")
	if trackedProcessesJSON != "" {
		var trackedProcesses []struct {
			PID        int    `json:"pid"`
			Name       string `json:"name"`
			InstallDir string `json:"install_dir"`
			Role       string `json:"role"`
		}
		if err := json.Unmarshal([]byte(trackedProcessesJSON), &trackedProcesses); err != nil {
			fmt.Printf("[Agent] Failed to parse tracked_processes: %v / 解析 tracked_processes 失败：%v\n", err, err)
		} else {
			fmt.Printf("[Agent] Received %d processes to track / 收到 %d 个需要跟踪的进程\n", len(trackedProcesses), len(trackedProcesses))
			for _, proc := range trackedProcesses {
				// Create start params for potential restart / 创建启动参数用于可能的重启
				startParams := &process.StartParams{
					InstallDir: proc.InstallDir,
					Role:       proc.Role,
				}

				if proc.PID > 0 {
					// Track running process silently - no started event since process was already running
					// 静默跟踪运行中的进程 - 不发送 started 事件，因为进程已经在运行
					a.processMonitor.TrackProcessSilent(proc.Name, proc.PID, proc.InstallDir, proc.Role, startParams)
					fmt.Printf("[Agent] Tracking running process (silent): %s (PID: %d, Role: %s, Dir: %s) / 静默跟踪运行中的进程：%s（PID：%d，角色：%s，目录：%s）\n",
						proc.Name, proc.PID, proc.Role, proc.InstallDir, proc.Name, proc.PID, proc.Role, proc.InstallDir)
				} else {
					// For stopped processes, register with PID 0 - auto-restart will start them if enabled
					// 对于已停止的进程，用 PID 0 注册 - 如果启用了自动重启，会自动启动它们
					a.processMonitor.TrackProcessSilent(proc.Name, 0, proc.InstallDir, proc.Role, startParams)
					fmt.Printf("[Agent] Registered stopped process (will auto-restart): %s (Role: %s, Dir: %s) / 注册已停止的进程（将自动重启）：%s（角色：%s，目录：%s）\n",
						proc.Name, proc.Role, proc.InstallDir, proc.Name, proc.Role, proc.InstallDir)
				}
			}
		}
	}

	reporter.Report(100, "Monitor config updated / 监控配置已更新")
	return executor.CreateSuccessResponse(cmd.CommandId, "Monitor config updated successfully / 监控配置更新成功"), nil
}

// isProcessAlive checks if a process with the given PID is alive
// isProcessAlive 检查给定 PID 的进程是否存活
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0 to check
	// 在 Unix 上，FindProcess 总是成功，所以我们需要发送信号 0 来检查
	if runtime.GOOS != "windows" {
		err = process.Signal(syscall.Signal(0))
		return err == nil
	}

	// On Windows, use tasklist / 在 Windows 上使用 tasklist
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), strconv.Itoa(pid))
}

// getProcessMetrics gets CPU and memory usage for a process
// getProcessMetrics 获取进程的 CPU 和内存使用率
func getProcessMetrics(pid int) (cpuUsage float64, memoryUsage int64) {
	if pid <= 0 {
		return 0, 0
	}

	switch runtime.GOOS {
	case "linux":
		// Read /proc/[pid]/statm for memory info
		// 读取 /proc/[pid]/statm 获取内存信息
		statmPath := fmt.Sprintf("/proc/%d/statm", pid)
		statmData, err := os.ReadFile(statmPath)
		if err != nil {
			return 0, 0
		}
		statmFields := strings.Fields(string(statmData))
		if len(statmFields) >= 2 {
			// RSS is in pages, convert to bytes (assuming 4KB pages)
			// RSS 以页为单位，转换为字节（假设 4KB 页）
			rss, _ := strconv.ParseInt(statmFields[1], 10, 64)
			memoryUsage = rss * 4096
		}
		return 0, memoryUsage

	case "darwin":
		cmd := exec.Command("ps", "-o", "rss=,pcpu=", "-p", strconv.Itoa(pid))
		output, err := cmd.Output()
		if err != nil {
			return 0, 0
		}
		fields := strings.Fields(string(output))
		if len(fields) >= 2 {
			rss, _ := strconv.ParseInt(fields[0], 10, 64)
			memoryUsage = rss * 1024
			cpu, _ := strconv.ParseFloat(fields[1], 64)
			cpuUsage = cpu
		}
		return cpuUsage, memoryUsage

	case "windows":
		cmd := exec.Command("wmic", "process", "where", fmt.Sprintf("ProcessId=%d", pid), "get", "WorkingSetSize", "/value")
		output, err := cmd.Output()
		if err != nil {
			return 0, 0
		}
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "WorkingSetSize=") {
				value := strings.TrimPrefix(line, "WorkingSetSize=")
				value = strings.TrimSpace(value)
				mem, _ := strconv.ParseInt(value, 10, 64)
				memoryUsage = mem
			}
		}
		return 0, memoryUsage

	default:
		return 0, 0
	}
}
