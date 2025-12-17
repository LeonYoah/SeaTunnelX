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
package router

import (
	"context"
	"log"

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
	"github.com/seatunnel/seatunnelX/internal/otel_trace"
	"github.com/seatunnel/seatunnelX/internal/session"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func Serve() {
	defer otel_trace.Shutdown(context.Background())

	// 运行模式
	if config.Config.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化数据库（根据配置自动选择 SQLite、MySQL 或 PostgreSQL）
	if err := db.InitDatabase(); err != nil {
		log.Fatalf("[API] 初始化数据库失败: %v\n", err)
	}

	// 初始化路由
	r := gin.New()
	r.Use(gin.Recovery())

	// 初始化会话存储（根据配置自动选择内存或 Redis）
	if err := session.InitSessionStore(); err != nil {
		log.Fatalf("[API] 初始化会话存储失败: %v\n", err)
	}
	r.Use(sessions.Sessions(config.Config.App.SessionCookieName, session.GinStore))

	// 初始化 OAuth 提供商（GitHub、Google）
	oauth.InitOAuthProviders()

	// 补充中间件
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
				ControlPlaneAddr: config.Config.App.Addr,
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
				clusterRouter.DELETE("/:id/nodes/:nodeId", clusterHandler.RemoveNode)

				// Cluster operations 集群操作
				clusterRouter.POST("/:id/start", clusterHandler.StartCluster)
				clusterRouter.POST("/:id/stop", clusterHandler.StopCluster)
				clusterRouter.POST("/:id/restart", clusterHandler.RestartCluster)
				clusterRouter.GET("/:id/status", clusterHandler.GetClusterStatus)
			}

			// Agent 分发 API（无需认证，供目标主机下载安装）
			// Agent distribution API (no authentication required, for target hosts to download and install)
			agentHandler := agent.NewHandler(&agent.HandlerConfig{
				ControlPlaneAddr: config.Config.App.Addr,
				AgentBinaryDir:   "./lib/agent",
				GRPCPort:         "50051",
			})

			agentRouter := apiV1Router.Group("/agent")
			{
				// GET /api/v1/agent/install.sh - 获取安装脚本
				// GET /api/v1/agent/install.sh - Get install script
				agentRouter.GET("/install.sh", agentHandler.GetInstallScript)

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

			// Host plugin routes 主机插件路由
			// GET /api/v1/hosts/:id/plugins - 获取主机已安装插件
			// GET /api/v1/hosts/:id/plugins - Get host installed plugins
			hostRouter.GET("/:id/plugins", pluginHandler.ListInstalledPlugins)

			// POST /api/v1/hosts/:id/plugins - 安装插件到主机
			// POST /api/v1/hosts/:id/plugins - Install plugin to host
			hostRouter.POST("/:id/plugins", pluginHandler.InstallPlugin)

			// DELETE /api/v1/hosts/:id/plugins/:name - 卸载插件
			// DELETE /api/v1/hosts/:id/plugins/:name - Uninstall plugin
			hostRouter.DELETE("/:id/plugins/:name", pluginHandler.UninstallPlugin)

			// PUT /api/v1/hosts/:id/plugins/:name/enable - 启用插件
			// PUT /api/v1/hosts/:id/plugins/:name/enable - Enable plugin
			hostRouter.PUT("/:id/plugins/:name/enable", pluginHandler.EnablePlugin)

			// PUT /api/v1/hosts/:id/plugins/:name/disable - 禁用插件
			// PUT /api/v1/hosts/:id/plugins/:name/disable - Disable plugin
			hostRouter.PUT("/:id/plugins/:name/disable", pluginHandler.DisablePlugin)

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

	// Serve
	if err := r.Run(config.Config.App.Addr); err != nil {
		log.Fatalf("[API] serve api failed: %v\n", err)
	}
}
