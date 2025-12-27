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
	"encoding/base64"
	"encoding/json"
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
	"github.com/seatunnel/seatunnelX/internal/apps/deepwiki"
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

	// Initialize OpenTelemetry tracing (based on config)
	// 初始化 OpenTelemetry 追踪（根据配置）
	otel_trace.Init()
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
			auditRepo := audit.NewRepository(db.DB(context.Background()))
			hostService := host.NewService(hostRepo, clusterRepo, &host.ServiceConfig{
				HeartbeatTimeout: time.Duration(config.Config.GRPC.HeartbeatTimeout) * time.Second,
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

			// Dashboard Overview 仪表盘概览
			// Initialize dashboard overview service and handler
			// 初始化仪表盘概览服务和处理器
			overviewService := dashboard.NewOverviewService(hostRepo, clusterRepo, auditRepo, time.Duration(config.Config.GRPC.HeartbeatTimeout)*time.Second)
			overviewHandler := dashboard.NewOverviewHandler(overviewService)

			overviewRouter := apiV1Router.Group("/dashboard/overview")
			overviewRouter.Use(auth.LoginRequired())
			{
				overviewRouter.GET("", overviewHandler.GetOverviewData)
				overviewRouter.GET("/stats", overviewHandler.GetOverviewStats)
				overviewRouter.GET("/clusters", overviewHandler.GetClusterSummaries)
				overviewRouter.GET("/hosts", overviewHandler.GetHostSummaries)
				overviewRouter.GET("/activities", overviewHandler.GetRecentActivities)
			}

			// Cluster 集群管理
			// Initialize cluster service and handler
			// 初始化集群服务和处理器
			clusterService := cluster.NewService(clusterRepo, hostService, &cluster.ServiceConfig{
				HeartbeatTimeout: time.Duration(config.Config.GRPC.HeartbeatTimeout) * time.Second,
			})

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

				// Node operations 节点操作
				clusterRouter.POST("/:id/nodes/:nodeId/start", clusterHandler.StartNode)
				clusterRouter.POST("/:id/nodes/:nodeId/stop", clusterHandler.StopNode)
				clusterRouter.POST("/:id/nodes/:nodeId/restart", clusterHandler.RestartNode)
				clusterRouter.GET("/:id/nodes/:nodeId/logs", clusterHandler.GetNodeLogs)

				// Cluster operations 集群操作
				clusterRouter.POST("/:id/start", clusterHandler.StartCluster)
				clusterRouter.POST("/:id/stop", clusterHandler.StopCluster)
				clusterRouter.POST("/:id/restart", clusterHandler.RestartCluster)
				clusterRouter.GET("/:id/status", clusterHandler.GetClusterStatus)
			}

			// Agent 分发 API（无需认证，供目标主机下载安装）
			// Agent distribution API (no authentication required, for target hosts to download and install)
			agentHandler := agent.NewHandler(&agent.HandlerConfig{
				ControlPlaneAddr:  config.GetExternalURL(),
				AgentBinaryDir:    "./lib/agent",
				GRPCPort:          fmt.Sprintf("%d", config.GetGRPCPort()),
				HeartbeatInterval: config.Config.GRPC.HeartbeatInterval,
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
			// Initialize audit handler (auditRepo already created above)
			// 初始化审计处理器（auditRepo 已在上面创建）
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
			installerService := installer.NewService("./lib/packages", nil)
			// Set host provider for precheck operations
			// 设置用于预检查操作的主机提供者
			installerService.SetHostProvider(&hostProviderAdapter{hostService: hostService})
			// Inject agent manager if available
			// 如果 Agent Manager 可用，注入
			if agentManager != nil {
				installerService.SetAgentManager(&installerAgentManagerAdapter{
					manager:     agentManager,
					hostService: hostService,
				})
			}
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

			// Inject agent command sender for plugin installation to cluster nodes
			// 注入 Agent 命令发送器用于将插件安装到集群节点
			if agentManager != nil {
				pluginService.SetAgentCommandSender(&pluginAgentCommandSenderAdapter{manager: agentManager})
				pluginService.SetClusterNodeGetter(&clusterNodeGetterAdapter{clusterService: clusterService})
				pluginService.SetHostInfoGetter(&hostInfoGetterAdapter{hostService: hostService})
				log.Println("[API] Agent command sender injected into plugin service / Agent 命令发送器已注入插件服务")

				// Inject plugin transferer into installer service for plugin transfer during installation
				// 将插件传输器注入安装服务，用于安装过程中的插件传输
				installerService.SetPluginTransferer(pluginService)
				log.Println("[API] Plugin transferer injected into installer service / 插件传输器已注入安装服务")

				// Inject node status updater into installer service for updating node status after installation
				// 将节点状态更新器注入安装服务，用于安装后更新节点状态
				installerService.SetNodeStatusUpdater(clusterService)
				log.Println("[API] Node status updater injected into installer service / 节点状态更新器已注入安装服务")

				// Inject node starter into installer service for starting nodes after installation
				// 将节点启动器注入安装服务，用于安装后启动节点
				installerService.SetNodeStarter(clusterService)
				log.Println("[API] Node starter injected into installer service / 节点启动器已注入安装服务")
			}

			pluginHandler := plugin.NewHandler(pluginService)

			// Plugin marketplace routes 插件市场路由
			pluginRouter := apiV1Router.Group("/plugins")
			pluginRouter.Use(auth.LoginRequired())
			{
				// GET /api/v1/plugins - 获取可用插件列表
				// GET /api/v1/plugins - List available plugins
				pluginRouter.GET("", pluginHandler.ListAvailablePlugins)

				// GET /api/v1/plugins/local - 获取已下载的本地插件列表
				// GET /api/v1/plugins/local - List locally downloaded plugins
				pluginRouter.GET("/local", pluginHandler.ListLocalPlugins)

				// GET /api/v1/plugins/downloads - 获取活动下载任务列表
				// GET /api/v1/plugins/downloads - List active download tasks
				pluginRouter.GET("/downloads", pluginHandler.ListActiveDownloads)

				// POST /api/v1/plugins/download-all - 一键下载所有插件
				// POST /api/v1/plugins/download-all - Download all plugins
				pluginRouter.POST("/download-all", pluginHandler.DownloadAllPlugins)

				// GET /api/v1/plugins/:name - 获取插件详情
				// GET /api/v1/plugins/:name - Get plugin info
				pluginRouter.GET("/:name", pluginHandler.GetPluginInfo)

				// POST /api/v1/plugins/:name/download - 下载插件到 Control Plane
				// POST /api/v1/plugins/:name/download - Download plugin to Control Plane
				pluginRouter.POST("/:name/download", pluginHandler.DownloadPlugin)

				// GET /api/v1/plugins/:name/download/status - 获取下载状态
				// GET /api/v1/plugins/:name/download/status - Get download status
				pluginRouter.GET("/:name/download/status", pluginHandler.GetDownloadStatus)

				// DELETE /api/v1/plugins/:name/local - 删除本地插件文件
				// DELETE /api/v1/plugins/:name/local - Delete local plugin file
				pluginRouter.DELETE("/:name/local", pluginHandler.DeleteLocalPlugin)

				// GET /api/v1/plugins/:name/dependencies - 获取插件依赖配置
				// GET /api/v1/plugins/:name/dependencies - Get plugin dependencies
				pluginRouter.GET("/:name/dependencies", pluginHandler.ListDependencies)

				// POST /api/v1/plugins/:name/dependencies - 添加插件依赖
				// POST /api/v1/plugins/:name/dependencies - Add plugin dependency
				pluginRouter.POST("/:name/dependencies", pluginHandler.AddDependency)

				// DELETE /api/v1/plugins/:name/dependencies/:depId - 删除插件依赖
				// DELETE /api/v1/plugins/:name/dependencies/:depId - Delete plugin dependency
				pluginRouter.DELETE("/:name/dependencies/:depId", pluginHandler.DeleteDependency)
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

			// GET /api/v1/clusters/:id/plugins/:name/progress - 获取插件安装进度
			// GET /api/v1/clusters/:id/plugins/:name/progress - Get plugin installation progress
			clusterRouter.GET("/:id/plugins/:name/progress", pluginHandler.GetInstallProgress)

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

			// DeepWiki 文档服务
			// DeepWiki documentation service
			deepwikiService := deepwiki.NewService(deepwiki.ServiceConfig{
				UseMCP:  false, // 使用直接 HTTP 模式 / Use direct HTTP mode
				Timeout: 30 * time.Second,
			})
			deepwikiHandler := deepwiki.NewHandler(deepwikiService)

			deepwikiRouter := apiV1Router.Group("/deepwiki")
			deepwikiRouter.Use(auth.LoginRequired())
			{
				// GET /api/v1/deepwiki/docs - 获取 SeaTunnel 文档
				// GET /api/v1/deepwiki/docs - Get SeaTunnel documentation
				deepwikiRouter.GET("/docs", deepwikiHandler.GetDocs)

				// POST /api/v1/deepwiki/fetch - 获取指定仓库文档
				// POST /api/v1/deepwiki/fetch - Fetch documentation for specific repository
				deepwikiRouter.POST("/fetch", deepwikiHandler.FetchDocs)

				// POST /api/v1/deepwiki/search - 搜索文档
				// POST /api/v1/deepwiki/search - Search documentation
				deepwikiRouter.POST("/search", deepwikiHandler.Search)
			}
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
	case "get_logs":
		return pb.CommandType_COLLECT_LOGS
	default:
		return pb.CommandType_PRECHECK
	}
}

// ==================== Plugin Service Adapters 插件服务适配器 ====================

// pluginAgentCommandSenderAdapter adapts agent.Manager to plugin.AgentCommandSender interface.
// pluginAgentCommandSenderAdapter 将 agent.Manager 适配到 plugin.AgentCommandSender 接口。
type pluginAgentCommandSenderAdapter struct {
	manager *agent.Manager
}

// SendCommand sends a command to an agent and returns the result.
// SendCommand 向 Agent 发送命令并返回结果。
func (a *pluginAgentCommandSenderAdapter) SendCommand(ctx context.Context, agentID string, commandType string, params map[string]string) (bool, string, error) {
	// Convert command type string to pb.CommandType
	// 将命令类型字符串转换为 pb.CommandType
	cmdType := a.stringToCommandType(commandType)

	// Use longer timeout for plugin transfer (5 minutes)
	// 插件传输使用更长的超时时间（5 分钟）
	timeout := 5 * time.Minute
	if commandType == "install_plugin" {
		timeout = 2 * time.Minute
	}

	resp, err := a.manager.SendCommand(ctx, agentID, cmdType, params, timeout)
	if err != nil {
		return false, "", err
	}

	// For transfer_plugin command, RUNNING status means chunk received successfully
	// 对于 transfer_plugin 命令，RUNNING 状态表示块接收成功
	success := resp.Status == pb.CommandStatus_SUCCESS
	if commandType == "transfer_plugin" {
		// Accept both SUCCESS and RUNNING as success for chunk transfer
		// 对于块传输，接受 SUCCESS 和 RUNNING 作为成功
		success = resp.Status == pb.CommandStatus_SUCCESS || resp.Status == pb.CommandStatus_RUNNING
	}

	message := resp.Output
	if resp.Error != "" {
		message = resp.Error
	}

	return success, message, nil
}

// stringToCommandType converts a command type string to pb.CommandType for plugin operations.
// stringToCommandType 将命令类型字符串转换为 pb.CommandType 用于插件操作。
func (a *pluginAgentCommandSenderAdapter) stringToCommandType(cmdType string) pb.CommandType {
	switch cmdType {
	case "transfer_plugin":
		return pb.CommandType_TRANSFER_PLUGIN
	case "install_plugin":
		return pb.CommandType_INSTALL_PLUGIN
	case "uninstall_plugin":
		return pb.CommandType_UNINSTALL_PLUGIN
	case "list_plugins":
		return pb.CommandType_LIST_PLUGINS
	default:
		return pb.CommandType_TRANSFER_PLUGIN
	}
}

// clusterNodeGetterAdapter adapts cluster.Service to plugin.ClusterNodeGetter interface.
// clusterNodeGetterAdapter 将 cluster.Service 适配到 plugin.ClusterNodeGetter 接口。
type clusterNodeGetterAdapter struct {
	clusterService *cluster.Service
}

// GetClusterNodes returns all nodes for a cluster.
// GetClusterNodes 返回集群的所有节点。
func (a *clusterNodeGetterAdapter) GetClusterNodes(ctx context.Context, clusterID uint) ([]plugin.ClusterNodeInfo, error) {
	nodes, err := a.clusterService.GetNodes(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	result := make([]plugin.ClusterNodeInfo, len(nodes))
	for i, node := range nodes {
		result[i] = plugin.ClusterNodeInfo{
			NodeID:     node.ID,
			HostID:     node.HostID,
			InstallDir: node.InstallDir,
		}
	}
	return result, nil
}

// hostInfoGetterAdapter adapts host.Service to plugin.HostInfoGetter interface.
// hostInfoGetterAdapter 将 host.Service 适配到 plugin.HostInfoGetter 接口。
type hostInfoGetterAdapter struct {
	hostService *host.Service
}

// GetHostAgentID returns the Agent ID for a host.
// GetHostAgentID 返回主机的 Agent ID。
func (a *hostInfoGetterAdapter) GetHostAgentID(ctx context.Context, hostID uint) (string, error) {
	h, err := a.hostService.Get(ctx, hostID)
	if err != nil {
		return "", err
	}
	return h.AgentID, nil
}

// ==================== Installer Service Adapters 安装服务适配器 ====================

// hostProviderAdapter adapts host.Service to installer.HostProvider interface.
// hostProviderAdapter 将 host.Service 适配到 installer.HostProvider 接口。
type hostProviderAdapter struct {
	hostService *host.Service
}

// GetHostByID returns host information by ID for installer precheck.
// GetHostByID 根据 ID 返回主机信息，用于安装预检查。
func (a *hostProviderAdapter) GetHostByID(ctx context.Context, hostID uint) (*installer.HostInfo, error) {
	h, err := a.hostService.Get(ctx, hostID)
	if err != nil {
		return nil, err
	}

	return &installer.HostInfo{
		ID:          h.ID,
		AgentID:     h.AgentID,
		AgentStatus: string(h.AgentStatus),
		LastSeen:    h.LastHeartbeat,
	}, nil
}

// installerAgentManagerAdapter adapts agent.Manager to installer.AgentManager interface.
// installerAgentManagerAdapter 将 agent.Manager 适配到 installer.AgentManager 接口。
type installerAgentManagerAdapter struct {
	manager     *agent.Manager
	hostService *host.Service
}

// GetAgentByHostID returns the agent connection for a host.
// GetAgentByHostID 返回主机的 Agent 连接。
func (a *installerAgentManagerAdapter) GetAgentByHostID(hostID uint) (agentID string, connected bool) {
	// Get host info to find agent ID
	// 获取主机信息以找到 agent ID
	ctx := context.Background()
	h, err := a.hostService.Get(ctx, hostID)
	if err != nil || h == nil {
		return "", false
	}

	if h.AgentID == "" {
		return "", false
	}

	// Check if agent is connected in agent manager
	// 检查 agent 是否在 agent manager 中连接
	conn, ok := a.manager.GetAgent(h.AgentID)
	if !ok || conn == nil {
		return "", false
	}

	// Check if agent status is connected
	// 检查 agent 状态是否为已连接
	if conn.GetStatus() != agent.AgentStatusConnected {
		return "", false
	}

	return h.AgentID, true
}

// SendInstallCommand sends an installation command to an agent.
// SendInstallCommand 向 Agent 发送安装命令。
func (a *installerAgentManagerAdapter) SendInstallCommand(ctx context.Context, agentID string, params map[string]string) (commandID string, err error) {
	// Use async command to allow polling for status updates
	// 使用异步命令以允许轮询状态更新
	return a.manager.SendCommandAsync(agentID, pb.CommandType_INSTALL, params, 30*time.Minute)
}

// GetCommandStatus returns the status of a command.
// GetCommandStatus 返回命令的状态。
func (a *installerAgentManagerAdapter) GetCommandStatus(commandID string) (status string, progress int, message string, err error) {
	return a.manager.GetCommandStatus(commandID)
}

// SendCommand sends a command to an agent and returns the result.
// SendCommand 向 Agent 发送命令并返回结果。
func (a *installerAgentManagerAdapter) SendCommand(ctx context.Context, agentID string, commandType string, params map[string]string) (success bool, output string, err error) {
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
	success = resp.Status == pb.CommandStatus_SUCCESS
	message := resp.Output
	if resp.Error != "" {
		message = resp.Error
	}

	return success, message, nil
}

// stringToCommandType converts a command type string to pb.CommandType.
// stringToCommandType 将命令类型字符串转换为 pb.CommandType。
func (a *installerAgentManagerAdapter) stringToCommandType(cmdType string) pb.CommandType {
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
	case "get_logs":
		return pb.CommandType_COLLECT_LOGS
	case "transfer_package":
		return pb.CommandType_TRANSFER_PACKAGE
	default:
		return pb.CommandType_PRECHECK
	}
}

// SendTransferPackageCommand sends a package transfer chunk to an agent.
// SendTransferPackageCommand 向 Agent 发送安装包传输块。
func (a *installerAgentManagerAdapter) SendTransferPackageCommand(ctx context.Context, agentID string, version string, fileName string, chunk []byte, offset int64, totalSize int64, isLast bool, checksum string) (success bool, receivedBytes int64, localPath string, err error) {
	// Build parameters / 构建参数
	params := map[string]string{
		"version":    version,
		"file_name":  fileName,
		"chunk":      base64.StdEncoding.EncodeToString(chunk),
		"offset":     fmt.Sprintf("%d", offset),
		"total_size": fmt.Sprintf("%d", totalSize),
		"is_last":    fmt.Sprintf("%t", isLast),
	}
	if checksum != "" {
		params["checksum"] = checksum
	}

	// Use longer timeout for package transfer (5 minutes per chunk)
	// 安装包传输使用更长的超时时间（每块 5 分钟）
	resp, err := a.manager.SendCommand(ctx, agentID, pb.CommandType_TRANSFER_PACKAGE, params, 5*time.Minute)
	if err != nil {
		return false, 0, "", err
	}

	// Accept both SUCCESS and RUNNING as success for chunk transfer
	// 对于块传输，接受 SUCCESS 和 RUNNING 作为成功
	success = resp.Status == pb.CommandStatus_SUCCESS || resp.Status == pb.CommandStatus_RUNNING

	// Parse response to get received bytes and local path
	// 解析响应获取已接收字节数和本地路径
	if resp.Output != "" {
		var transferResp struct {
			Success       bool   `json:"success"`
			Message       string `json:"message"`
			ReceivedBytes int64  `json:"received_bytes"`
			LocalPath     string `json:"local_path"`
		}
		if jsonErr := json.Unmarshal([]byte(resp.Output), &transferResp); jsonErr == nil {
			receivedBytes = transferResp.ReceivedBytes
			localPath = transferResp.LocalPath
			if !transferResp.Success {
				success = false
			}
		}
	}

	if resp.Error != "" {
		return false, receivedBytes, localPath, fmt.Errorf(resp.Error)
	}

	return success, receivedBytes, localPath, nil
}
