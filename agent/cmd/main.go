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
	"syscall"

	"github.com/seatunnel/seatunnelX/agent/internal/config"
	"github.com/spf13/cobra"
)

// Version information, set at build time
// 版本信息，在构建时设置
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// Agent represents the main agent service
// Agent 表示主要的 Agent 服务
type Agent struct {
	config *config.Config
	ctx    context.Context
	cancel context.CancelFunc
}

// NewAgent creates a new Agent instance
// NewAgent 创建一个新的 Agent 实例
func NewAgent(cfg *config.Config) *Agent {
	ctx, cancel := context.WithCancel(context.Background())
	return &Agent{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Run starts the Agent service
// Run 启动 Agent 服务
func (a *Agent) Run() error {
	// TODO: Implement in Phase 6
	// TODO: 在 Phase 6 中实现
	// 1. Initialize gRPC client / 初始化 gRPC 客户端
	// 2. Register with Control Plane / 向 Control Plane 注册
	// 3. Start heartbeat goroutine / 启动心跳 goroutine
	// 4. Start command stream listener / 启动指令流监听
	// 5. Start metrics collector / 启动指标采集器

	fmt.Println("Agent service started")
	fmt.Printf("Version: %s, Commit: %s, Build: %s\n", Version, GitCommit, BuildTime)
	fmt.Printf("Control Plane: %v\n", a.config.ControlPlane.Addresses)

	// Wait for context cancellation
	// 等待上下文取消
	<-a.ctx.Done()
	return nil
}

// Shutdown gracefully stops the Agent service
// Shutdown 优雅地停止 Agent 服务
func (a *Agent) Shutdown() {
	fmt.Println("Shutting down Agent service...")
	a.cancel()
	// TODO: Implement graceful shutdown in Phase 6
	// TODO: 在 Phase 6 中实现优雅关闭
	// 1. Complete running tasks / 完成正在执行的任务
	// 2. Send offline notification / 发送下线通知
	// 3. Close gRPC connection / 关闭 gRPC 连接
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
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate configuration
	// 验证配置
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Create and run agent
	// 创建并运行 Agent
	agent := NewAgent(cfg)

	// Setup signal handling for graceful shutdown
	// 设置信号处理以实现优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

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
		fmt.Printf("Received signal: %v\n", sig)
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
