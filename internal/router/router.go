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
	"github.com/seatunnel/seatunnelX/internal/apps/auth"
	"github.com/seatunnel/seatunnelX/internal/apps/cluster"
	"github.com/seatunnel/seatunnelX/internal/apps/dashboard"
	"github.com/seatunnel/seatunnelX/internal/apps/health"
	"github.com/seatunnel/seatunnelX/internal/apps/host"
	"github.com/seatunnel/seatunnelX/internal/apps/oauth"
	"github.com/seatunnel/seatunnelX/internal/apps/project"
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
		}
	}

	// Serve
	if err := r.Run(config.Config.App.Addr); err != nil {
		log.Fatalf("[API] serve api failed: %v\n", err)
	}
}
