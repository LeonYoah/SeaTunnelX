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

// Package router 提供 HTTP 路由配置
// Package router provides HTTP routing configuration
package router

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	_ "github.com/seatunnel/seatunnelX/docs"
	"github.com/seatunnel/seatunnelX/internal/apps/admin"
	"github.com/seatunnel/seatunnelX/internal/apps/agent"
	"github.com/seatunnel/seatunnelX/internal/apps/audit"
	"github.com/seatunnel/seatunnelX/internal/apps/auth"
	"github.com/seatunnel/seatunnelX/internal/apps/cluster"
	"github.com/seatunnel/seatunnelX/internal/apps/dashboard"
	"github.com/seatunnel/seatunnelX/internal/apps/health"
	"github.com/seatunnel/seatunnelX/internal/apps/host"
	"github.com/seatunnel/seatunnelX/internal/apps/installer"
	"github.com/seatunnel/seatunnelX/internal/apps/oauth"
	"github.com/seatunnel/seatunnelX/internal/apps/plugin"
	"github.com/seatunnel/seatunnelX/internal/apps/project"
	"github.com/seatunnel/seatunnelX/internal/apps/task"
	"github.com/seatunnel/seatunnelX/internal/config"
	"github.com/seatunnel/seatunnelX/internal/db"
	grpcServer "github.com/seatunnel/seatunnelX/internal/grpc"
	"github.com/seatunnel/seatunnelX/internal/otel_trace"
	pb "github.com/seatunnel/seatunnelX/internal/proto/agent"
	"github.com/seatunnel/seatunnelX/internal/session"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
)

func Serve() {
	ctx := context.Background()
	defer otel_trace.Shutdown(ctx)

	// 运行模式
	// Set run mode
	if config.Config.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化数据库（根据配置自动选择 SQLite、MySQL 或 PostgreSQL）
	// Initialize database (auto-select SQLite, MySQL or PostgreSQL based on config)
	if err := db.InitDatabase(); err != nil {
		log.Fatalf("[API] 初始化数据库失败: %v\n", err)
	}

	// 初始化 gRPC 服务器（如果启用）
	// Initialize gRPC server (if enabled)
	// Requirements: 1.1, 3.4 - Starts gRPC server and heartbeat timeout detection
	var grpcSrv *grpcServer.Server
	var agentManager *agent.Manager
	if config.IsGRPCEnabled() {
		grpcSrv, agentManager = initGRPCServer(ctx)
		if grpcSrv != nil {
			defer grpcSrv.Stop()
		}
		if agentManager != nil {
			defer agentManager.Stop()
		}
	} else {
		log.Println("[API] gRPC 服务器已禁用 / gRPC server is disabled")
	}

	// 初始化路由
	// Initialize router
	r := gin.New()
	r.Use(gin.Recovery())

	// 初始化会话存储（根据配置自动选择内存或 Redis）
	// Initialize session store (auto-select memory or Redis based on config)
	if err := session.InitSessionStore(); err != nil {
		log.Fatalf("[API] 初始化会话存储失败: %v\n", err)
	}
	r.Use(sessions.Sessions(config.Config.App.SessionCookieName, session.GinStore))

	// 初始化 OAuth 提供商（GitHub、Google）
	// Initialize OAuth providers (GitHub, Google)
	oauth.InitOAuthProviders()

	// 补充中间件
	// Add middleware
	r.Use(otelgin.Middleware(config.Config.App.AppName), loggerMiddleware())

	apiGroup := r.Group(config.Config.App.APIPrefix)
	{
		if config.Config.App.Env == "development" {
			// Swagger
			apiGroup.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
		}

		// API V1
		apiV1Router := apiGroup.Group("/v1")
		{
			// Health
			apiV1Router.GET("/health", health.Health)

			// Auth（统一认证接口，支持密码登录和 OAuth 登录）
			apiV1Router.POST("/auth/login", auth.Login)
			apiV1Router.POST("/auth/logout", auth.LoginRequired(), auth.Logout)
			apiV1Router.GET("/auth/user-info", auth.LoginRequired(), auth.GetUserInfo)

			// OAuth（备选登录方式：GitHub、Google）
			apiV1Router.GET("/oauth/providers", oauth.GetEnabledProvidersHandler)
			apiV1Router.GET("/oauth/login", oauth.GetLoginURL)
			apiV1Router.POST("/oauth/callback", oauth.Callback)

			// Project
			projectRouter := apiV1Router.Group("/projects")
			projectRouter.Use(auth.LoginRequired())
			{
				projectRouter.GET("/mine", project.ListMyProjects)
				projectRouter.GET("", project.ListProjects)
				projectRouter.POST("", project.ProjectCreateRateLimitMiddleware(), project.CreateProject)
				projectRouter.PUT("/:id", project.ProjectCreatorPermMiddleware(), project.UpdateProject)
				projectRouter.DELETE("/:id", project.ProjectCreatorPermMiddleware(), project.DeleteProject)
				projectRouter.GET("/:id/receivers", project.ProjectCreatorPermMiddleware(), project.ListProjectReceivers)
				projectRouter.POST("/:id/receive", project.ReceiveProjectMiddleware(), project.ReceiveProject)
				projectRouter.POST("/:id/report", project.ReportProject)
				projectRouter.GET("/received", project.ListReceiveHistory)
				projectRouter.GET("/:id", project.GetProject)
			}

			// Tag
			tagRouter := apiV1Router.Group("/tags")
			tagRouter.Use(auth.LoginRequired())
			{
				tagRouter.GET("", project.ListTags)
			}

			// Dashboard
			dashboardRouter := apiV1Router.Group("/dashboard")
			dashboardRouter.Use(auth.LoginRequired())
			{
				dashboardRouter.GET("/stats/all", dashboard.GetAllStats)
			}

			// Admin
			adminRouter := apiV1Router.Group("/admin")
			adminRouter.Use(auth.LoginRequired(), admin.LoginAdminRequired())
			{
				// Project
				projectAdminRouter := adminRouter.Group("/projects")
				{
					projectAdminRouter.GET("", admin.GetProjectsList)
					projectAdminRouter.PUT("/:id/review", admin.ReviewProject)
				}

				// User 用户管理
				userAdminRouter := adminRouter.Group("/users")
				{
					userAdminRouter.GET("", admin.ListUsersHandler)
					userAdminRouter.POST("", admin.CreateUserHandler)
					userAdminRouter.GET("/:id", admin.GetUserHandler)
					userAdminRouter.PUT("/:id", admin.UpdateUserHandler)
					userAdminRouter.DELETE("/:id", admin.DeleteUserHandler)
				}
			}

			// Host 主机管理
			// Initialize host service and handler
			// 初始化主机服务和处理器
			hostRepo := host.NewRepository(db.DB(context.Background()))
			clusterRepo := cluster.NewRepository(db.DB(context.Background()))
			hostService := host.NewService(hostRepo, clusterRepo, &host.ServiceConfig{
				ControlPlaneAddr: config.GetExternalURL(),
			})
			hostHandler := host.NewHandler(hostService)

			hostRouter := apiV1Router.Group("/hosts")
			hostRouter.Use(auth.LoginRequired())
			{
				hostRouter.POST("", hostHandler.CreateHost)
				hostRouter.GET("", hostHandler.ListHosts)
				hostRouter.GET("/:id", hostHandler.GetHost)
				hostRouter.PUT("/:id", hostHandler.UpdateHost)
				hostRouter.DELETE("/:id", hostHandler.DeleteHost)
				hostRouter.GET("/:id/install-command", hostHandler.GetInstallCommand)
			}

			// Cluster 集群管理
			// Initialize cluster service and handler
			// 初始化集群服务和处理器
			clusterService := cluster.NewService(clusterRepo, hostService, &cluster.ServiceConfig{})

			// Inject agent command sender if agent manager is available
			// 如果 Agent Manager 可用，注入 Agent 命令发送器
			if agentManager != nil {
				clusterService.SetAgentCommandSender(&agentCommandSenderAdapter{manager: agentManager})
				log.Println("[API] Agent command sender injected into cluster service / Agent 命令发送器已注入集群服务")
			}

			clusterHandler := cluster.NewHandler(clusterService)

			clusterRouter := apiV1Router.Group("/clusters")
			clusterRouter.Use(auth.LoginRequired())
			{
				// Cluster CRUD 集群增删改查
				clusterRouter.POST("", clusterHandler.CreateCluster)
				clusterRouter.GET("", clusterHandler.ListClusters)
				clusterRouter.GET("/:id", clusterHandler.GetCluster)
				clusterRouter.PUT("/:id", clusterHandler.UpdateCluster)
				clusterRouter.DELETE("/:id", clusterHandler.DeleteCluster)

				// Node management 节点管理
				clusterRouter.POST("/:id/nodes", clusterHandler.AddNode)
				clusterRouter.GET("/:id/nodes", clusterHandler.GetNodes)
				clusterRouter.PUT("/:id/nodes/:nodeId", clusterHandler.UpdateNode)
				clusterRouter.DELETE("/:id/nodes/:nodeId", clusterHandler.RemoveNode)
				clusterRouter.POST("/:id/nodes/precheck", clusterHandler.PrecheckNode)

				// Cluster operations 集群操作
				clusterRouter.POST("/:id/start", clusterHandler.StartCluster)
				clusterRouter.POST("/:id/stop", clusterHandler.StopCluster)
				clusterRouter.POST("/:id/restart", clusterHandler.RestartCluster)
				clusterRouter.GET("/:id/status", clusterHandler.GetClusterStatus)
			}

			// Agent 分发 API（无需认证，供目标主机下载安装）
			// Agent distribution API (no authentication required, for target hosts to download and install)
			agentHandler := agent.NewHandler(&agent.HandlerConfig{
				ControlPlaneAddr: config.GetExternalURL(),
				AgentBinaryDir:   "./lib/agent",
				GRPCPort:         fmt.Sprintf("%d", config.GetGRPCPort()),
			})

			agentRouter := apiV1Router.Group("/agent")
			{
				// GET /api/v1/agent/install.sh - 获取安装脚本
				// GET /api/v1/agent/install.sh - Get install script
				agentRouter.GET("/install.sh", agentHandler.GetInstallScript)

				// GET /api/v1/agent/uninstall.sh - 获取卸载脚本
				// GET /api/v1/agent/uninstall.sh - Get uninstall script
				agentRouter.GET("/uninstall.sh", agentHandler.GetUninstallScript)

				// GET /api/v1/agent/download - 下载 Agent 二进制文件
				// GET /api/v1/agent/download - Download Agent binary
				agentRouter.GET("/download", agentHandler.DownloadAgent)
			}

			// Audit 审计日志 API
			// Audit log API
			// Initialize audit repository and handler
			// 初始化审计仓库和处理器
			auditRepo := audit.NewRepository(db.DB(context.Background()))
			auditHandler := audit.NewHandler(auditRepo)

			// Command logs 命令日志
			commandRouter := apiV1Router.Group("/commands")
			commandRouter.Use(auth.LoginRequired())
			{
				// GET /api/v1/commands - 获取命令日志列表
				// GET /api/v1/commands - Get command logs list
				commandRouter.GET("", auditHandler.ListCommandLogs)

				// GET /api/v1/commands/:id - 获取命令日志详情
				// GET /api/v1/commands/:id - Get command log details
				commandRouter.GET("/:id", auditHandler.GetCommandLog)
			}

			// Audit logs 审计日志
			auditLogRouter := apiV1Router.Group("/audit-logs")
			auditLogRouter.Use(auth.LoginRequired())
			{
				// GET /api/v1/audit-logs - 获取审计日志列表
				// GET /api/v1/audit-logs - Get audit logs list
				auditLogRouter.GET("", auditHandler.ListAuditLogs)

				// GET /api/v1/audit-logs/:id - 获取审计日志详情
				// GET /api/v1/audit-logs/:id - Get audit log details
				auditLogRouter.GET("/:id", auditHandler.GetAuditLog)
			}

			// Installer SeaTunnel 安装管理
			// Initialize installer service and handler
			// 初始化安装服务和处理器
			installerService := installer.NewService("./lib/packages")
			installerHandler := installer.NewHandler(installerService)

			// Package management routes 安装包管理路由
			packageRouter := apiV1Router.Group("/packages")
			packageRouter.Use(auth.LoginRequired())
			{
				// GET /api/v1/packages - 获取可用安装包列表
				// GET /api/v1/packages - List available packages
				packageRouter.GET("", installerHandler.ListPackages)

				// POST /api/v1/packages/versions/refresh - 刷新版本列表
				// POST /api/v1/packages/versions/refresh - Refresh version list
				packageRouter.POST("/versions/refresh", installerHandler.RefreshVersions)

				// GET /api/v1/packages/:version - 获取安装包信息
				// GET /api/v1/packages/:version - Get package info
				packageRouter.GET("/:version", installerHandler.GetPackageInfo)

				// POST /api/v1/packages/upload - 上传安装包
				// POST /api/v1/packages/upload - Upload package
				packageRouter.POST("/upload", installerHandler.UploadPackage)

				// DELETE /api/v1/packages/:version - 删除本地安装包
				// DELETE /api/v1/packages/:version - Delete local package
				packageRouter.DELETE("/:version", installerHandler.DeletePackage)

				// POST /api/v1/packages/download - 开始下载安装包到服务器
				// POST /api/v1/packages/download - Start downloading package to server
				packageRouter.POST("/download", installerHandler.StartDownload)

				// GET /api/v1/packages/downloads - 获取所有下载任务
				// GET /api/v1/packages/downloads - List all download tasks
				packageRouter.GET("/downloads", installerHandler.ListDownloads)

				// GET /api/v1/packages/download/:version - 获取下载状态
				// GET /api/v1/packages/download/:version - Get download status
				packageRouter.GET("/download/:version", installerHandler.GetDownloadStatus)

				// POST /api/v1/packages/download/:version/cancel - 取消下载
				// POST /api/v1/packages/download/:version/cancel - Cancel download
				packageRouter.POST("/download/:version/cancel", installerHandler.CancelDownload)
			}

			// Task 任务管理
			// Initialize task manager and handler
			// 初始化任务管理器和处理器
			taskManager := task.NewManager()
			taskHandler := task.NewHandler(taskManager)

			// Task management routes 任务管理路由
			taskRouter := apiV1Router.Group("/tasks")
			taskRouter.Use(auth.LoginRequired())
			{
				// POST /api/v1/tasks - 创建任务
				// POST /api/v1/tasks - Create task
				taskRouter.POST("", taskHandler.CreateTask)

				// GET /api/v1/tasks - 获取任务列表
				// GET /api/v1/tasks - List tasks
				taskRouter.GET("", taskHandler.ListTasks)

				// GET /api/v1/tasks/:id - 获取任务详情
				// GET /api/v1/tasks/:id - Get task details
				taskRouter.GET("/:id", taskHandler.GetTask)

				// POST /api/v1/tasks/:id/start - 开始执行任务
				// POST /api/v1/tasks/:id/start - Start task
				taskRouter.POST("/:id/start", taskHandler.StartTask)

				// POST /api/v1/tasks/:id/cancel - 取消任务
				// POST /api/v1/tasks/:id/cancel - Cancel task
				taskRouter.POST("/:id/cancel", taskHandler.CancelTask)

				// POST /api/v1/tasks/:id/retry - 重试任务
				// POST /api/v1/tasks/:id/retry - Retry task
				taskRouter.POST("/:id/retry", taskHandler.RetryTask)
			}

			// Host tasks route 主机任务路由
			// GET /api/v1/hosts/:id/tasks - 获取主机任务列表
			// GET /api/v1/hosts/:id/tasks - List host tasks
			hostRouter.GET("/:id/tasks", taskHandler.ListHostTasks)

			// Plugin 插件市场管理
			// Initialize plugin repository, service and handler
			// 初始化插件仓库、服务和处理器
			pluginRepo := plugin.NewRepository(db.DB(context.Background()))
			pluginService := plugin.NewService(pluginRepo)
			// Inject cluster service for version validation
			// 注入集群服务用于版本校验
			pluginService.SetClusterGetter(clusterService)
			pluginHandler := plugin.NewHandler(pluginService)

			// Plugin marketplace routes 插件市场路由
			pluginRouter := apiV1Router.Group("/plugins")
			pluginRouter.Use(auth.LoginRequired())
			{
				// GET /api/v1/plugins - 获取可用插件列表
				// GET /api/v1/plugins - List available plugins
				pluginRouter.GET("", pluginHandler.ListAvailablePlugins)

				// GET /api/v1/plugins/:name - 获取插件详情
				// GET /api/v1/plugins/:name - Get plugin info
				pluginRouter.GET("/:name", pluginHandler.GetPluginInfo)
			}

			// Cluster plugin routes 集群插件路由
			// GET /api/v1/clusters/:id/plugins - 获取集群已安装插件
			// GET /api/v1/clusters/:id/plugins - Get cluster installed plugins
			clusterRouter.GET("/:id/plugins", pluginHandler.ListInstalledPlugins)

			// POST /api/v1/clusters/:id/plugins - 安装插件到集群
			// POST /api/v1/clusters/:id/plugins - Install plugin to cluster
			clusterRouter.POST("/:id/plugins", pluginHandler.InstallPlugin)

			// DELETE /api/v1/clusters/:id/plugins/:name - 卸载插件
			// DELETE /api/v1/clusters/:id/plugins/:name - Uninstall plugin
			clusterRouter.DELETE("/:id/plugins/:name", pluginHandler.UninstallPlugin)

			// PUT /api/v1/clusters/:id/plugins/:name/enable - 启用插件
			// PUT /api/v1/clusters/:id/plugins/:name/enable - Enable plugin
			clusterRouter.PUT("/:id/plugins/:name/enable", pluginHandler.EnablePlugin)

			// PUT /api/v1/clusters/:id/plugins/:name/disable - 禁用插件
			// PUT /api/v1/clusters/:id/plugins/:name/disable - Disable plugin
			clusterRouter.PUT("/:id/plugins/:name/disable", pluginHandler.DisablePlugin)

			// Installation routes on hosts 主机安装路由
			// POST /api/v1/hosts/:id/precheck - 运行预检查
			// POST /api/v1/hosts/:id/precheck - Run precheck
			hostRouter.POST("/:id/precheck", installerHandler.RunPrecheck)

			// POST /api/v1/hosts/:id/install - 开始安装
			// POST /api/v1/hosts/:id/install - Start installation
			hostRouter.POST("/:id/install", installerHandler.StartInstallation)

			// GET /api/v1/hosts/:id/install/status - 获取安装状态
			// GET /api/v1/hosts/:id/install/status - Get installation status
			hostRouter.GET("/:id/install/status", installerHandler.GetInstallationStatus)

			// POST /api/v1/hosts/:id/install/retry - 重试失败步骤
			// POST /api/v1/hosts/:id/install/retry - Retry failed step
			hostRouter.POST("/:id/install/retry", installerHandler.RetryStep)

			// POST /api/v1/hosts/:id/install/cancel - 取消安装
			// POST /api/v1/hosts/:id/install/cancel - Cancel installation
			hostRouter.POST("/:id/install/cancel", installerHandler.CancelInstallation)
		}
	}

	// Serve HTTP API
	// 启动 HTTP API 服务
	log.Printf("[API] HTTP 服务器启动于 %s / HTTP server starting on %s\n", config.Config.App.Addr, config.Config.App.Addr)
	if err := r.Run(config.Config.App.Addr); err != nil {
		log.Fatalf("[API] serve api failed: %v\n", err)
	}
}

// initGRPCServer initializes and starts the gRPC server for Agent communication.
// initGRPCServer 初始化并启动用于 Agent 通信的 gRPC 服务器。
// Requirements: 1.1, 3.4 - Starts gRPC server and heartbeat timeout detection.
func initGRPCServer(ctx context.Context) (*grpcServer.Server, *agent.Manager) {
	grpcConfig := config.GetGRPCConfig()

	// 创建 logger
	// Create logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Printf("[gRPC] 创建 logger 失败: %v / Failed to create logger: %v\n", err, err)
		logger, _ = zap.NewDevelopment()
	}

	// 初始化 Agent Manager
	// Initialize Agent Manager
	// Requirements: 3.4 - Starts heartbeat timeout detection goroutine
	agentManager := agent.NewManager(&agent.ManagerConfig{
		HeartbeatInterval: time.Duration(grpcConfig.HeartbeatInterval) * time.Second,
		HeartbeatTimeout:  time.Duration(grpcConfig.HeartbeatTimeout) * time.Second,
		CheckInterval:     5 * time.Second,
	})

	// 初始化 Host Service 用于 Agent 状态更新
	// Initialize Host Service for Agent status updates
	hostRepo := host.NewRepository(db.DB(ctx))
	clusterRepo := cluster.NewRepository(db.DB(ctx))
	hostService := host.NewService(hostRepo, clusterRepo, &host.ServiceConfig{
		HeartbeatTimeout: time.Duration(grpcConfig.HeartbeatTimeout) * time.Second,
		ControlPlaneAddr: config.GetExternalURL(),
	})

	// 设置 Host 状态更新器
	// Set Host status updater
	agentManager.SetHostUpdater(&hostStatusUpdaterAdapter{hostService: hostService})

	// 初始化 Audit Repository 用于日志记录
	// Initialize Audit Repository for logging
	auditRepo := audit.NewRepository(db.DB(ctx))

	// 创建 gRPC 服务器配置
	// Create gRPC server configuration
	serverConfig := &grpcServer.ServerConfig{
		Port:              grpcConfig.Port,
		TLSEnabled:        grpcConfig.TLSEnabled,
		CertFile:          grpcConfig.CertFile,
		KeyFile:           grpcConfig.KeyFile,
		CAFile:            grpcConfig.CAFile,
		MaxRecvMsgSize:    grpcConfig.MaxRecvMsgSize * 1024 * 1024, // MB to bytes
		MaxSendMsgSize:    grpcConfig.MaxSendMsgSize * 1024 * 1024, // MB to bytes
		HeartbeatInterval: grpcConfig.HeartbeatInterval,
	}

	// 创建并启动 gRPC 服务器
	// Create and start gRPC server
	srv := grpcServer.NewServer(serverConfig, agentManager, hostService, auditRepo, logger)

	if err := srv.Start(ctx); err != nil {
		log.Printf("[gRPC] 启动 gRPC 服务器失败: %v / Failed to start gRPC server: %v\n", err, err)
		return nil, nil
	}

	log.Printf("[gRPC] gRPC 服务器启动于端口 %d / gRPC server started on port %d\n", grpcConfig.Port, grpcConfig.Port)

	// 启动 Agent Manager 后台任务（心跳超时检测）
	// Start Agent Manager background tasks (heartbeat timeout detection)
	if err := agentManager.Start(ctx); err != nil {
		log.Printf("[gRPC] 启动 Agent Manager 失败: %v / Failed to start Agent Manager: %v\n", err, err)
	}

	return srv, agentManager
}

// hostStatusUpdaterAdapter adapts host.Service to agent.HostStatusUpdater interface.
// hostStatusUpdaterAdapter 将 host.Service 适配到 agent.HostStatusUpdater 接口。
type hostStatusUpdaterAdapter struct {
	hostService *host.Service
}

// UpdateAgentStatus updates the agent status for a host by IP address.
// UpdateAgentStatus 根据 IP 地址更新主机的 Agent 状态。
func (a *hostStatusUpdaterAdapter) UpdateAgentStatus(ctx context.Context, ipAddress string, agentID string, version string, systemInfo *agent.SystemInfo) (hostID uint, err error) {
	var sysInfo *host.SystemInfo
	if systemInfo != nil {
		sysInfo = &host.SystemInfo{
			OSType:      systemInfo.OSType,
			Arch:        systemInfo.Arch,
			CPUCores:    systemInfo.CPUCores,
			TotalMemory: systemInfo.TotalMemory,
			TotalDisk:   systemInfo.TotalDisk,
		}
	}

	h, err := a.hostService.UpdateAgentStatus(ctx, ipAddress, agentID, version, sysInfo)
	if err != nil {
		return 0, err
	}
	return h.ID, nil
}

// UpdateHeartbeat updates the heartbeat data for a host.
// UpdateHeartbeat 更新主机的心跳数据。
func (a *hostStatusUpdaterAdapter) UpdateHeartbeat(ctx context.Context, agentID string, cpuUsage, memoryUsage, diskUsage float64) error {
	return a.hostService.UpdateHeartbeat(ctx, agentID, cpuUsage, memoryUsage, diskUsage)
}

// MarkHostOffline marks a host as offline by agent ID.
// MarkHostOffline 根据 Agent ID 将主机标记为离线。
func (a *hostStatusUpdaterAdapter) MarkHostOffline(ctx context.Context, agentID string) error {
	h, err := a.hostService.GetByAgentID(ctx, agentID)
	if err != nil {
		return err
	}
	return a.hostService.UpdateAgentStatusByID(ctx, h.ID, host.AgentStatusOffline, agentID, h.AgentVersion)
}

// agentCommandSenderAdapter adapts agent.Manager to cluster.AgentCommandSender interface.
// agentCommandSenderAdapter 将 agent.Manager 适配到 cluster.AgentCommandSender 接口。
type agentCommandSenderAdapter struct {
	manager *agent.Manager
}

// SendCommand sends a command to an agent and returns the result.
// SendCommand 向 Agent 发送命令并返回结果。
func (a *agentCommandSenderAdapter) SendCommand(ctx context.Context, agentID string, commandType string, params map[string]string) (bool, string, error) {
	// Convert command type string to pb.CommandType
	// 将命令类型字符串转换为 pb.CommandType
	cmdType := a.stringToCommandType(commandType)

	// Add sub_command parameter for precheck commands
	// 为预检查命令添加 sub_command 参数
	if cmdType == pb.CommandType_PRECHECK && params["sub_command"] == "" {
		params["sub_command"] = commandType
	}

	// Send command with 30 second timeout
	// 使用 30 秒超时发送命令
	resp, err := a.manager.SendCommand(ctx, agentID, cmdType, params, 30*time.Second)
	if err != nil {
		return false, "", err
	}

	// Convert response to (bool, string, error)
	// 将响应转换为 (bool, string, error)
	success := resp.Status == pb.CommandStatus_SUCCESS
	message := resp.Output
	if resp.Error != "" {
		message = resp.Error
	}

	return success, message, nil
}

// stringToCommandType converts a command type string to pb.CommandType.
// stringToCommandType 将命令类型字符串转换为 pb.CommandType。
func (a *agentCommandSenderAdapter) stringToCommandType(cmdType string) pb.CommandType {
	switch cmdType {
	case "check_port", "check_directory", "check_http", "check_process", "full":
		return pb.CommandType_PRECHECK
	case "install":
		return pb.CommandType_INSTALL
	case "uninstall":
		return pb.CommandType_UNINSTALL
	case "upgrade":
		return pb.CommandType_UPGRADE
	case "start":
		return pb.CommandType_START
	case "stop":
		return pb.CommandType_STOP
	case "restart":
		return pb.CommandType_RESTART
	case "status":
		return pb.CommandType_STATUS
	default:
		return pb.CommandType_PRECHECK
	}
}
