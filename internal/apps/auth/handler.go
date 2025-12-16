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

// Package auth 提供用户认证相关的 HTTP 处理器
package auth

import (
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/seatunnel/seatunnelX/internal/db"
	"github.com/seatunnel/seatunnelX/internal/logger"
)

// 会话键常量
const (
	SessionKeyUserID   = "user_id"
	SessionKeyUsername = "username"
)

// 错误消息常量（不暴露具体是用户名还是密码错误）
const (
	ErrMsgInvalidCredentials = "用户名或密码错误"
	ErrMsgEmptyCredentials   = "用户名和密码不能为空"
	ErrMsgUserInactive       = "用户账户已禁用"
	ErrMsgSessionError       = "会话错误"
	ErrMsgInternalError      = "内部服务器错误"
)

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	ErrorMsg string      `json:"error_msg"`
	Data     interface{} `json:"data"`
}

// UserInfoResponse 用户信息响应
type UserInfoResponse struct {
	ErrorMsg string    `json:"error_msg"`
	Data     *UserInfo `json:"data"`
}

// LogoutResponse 登出响应
type LogoutResponse struct {
	ErrorMsg string      `json:"error_msg"`
	Data     interface{} `json:"data"`
}

// Login 处理用户登录
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "登录请求"
// @Success 200 {object} LoginResponse
// @Router /api/v1/auth/login [post]
func Login(c *gin.Context) {
	var req LoginRequest

	// 绑定请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, LoginResponse{ErrorMsg: ErrMsgEmptyCredentials})
		return
	}

	// 验证用户名和密码不为空（去除空白字符后）
	username := strings.TrimSpace(req.Username)
	password := req.Password // 密码不去除空白，保持原样

	if username == "" || password == "" {
		c.JSON(http.StatusBadRequest, LoginResponse{ErrorMsg: ErrMsgEmptyCredentials})
		return
	}

	// 查找用户
	user, err := FindByUsername(db.GetDB(c.Request.Context()), username)
	if err != nil {
		// 不暴露用户是否存在，统一返回凭证错误
		logger.InfoF(c.Request.Context(), "[Auth] 登录失败 - 用户不存在: %s", username)
		c.JSON(http.StatusUnauthorized, LoginResponse{ErrorMsg: ErrMsgInvalidCredentials})
		return
	}

	// 验证密码
	if !user.CheckPassword(password) {
		logger.InfoF(c.Request.Context(), "[Auth] 登录失败 - 密码错误: %s", username)
		c.JSON(http.StatusUnauthorized, LoginResponse{ErrorMsg: ErrMsgInvalidCredentials})
		return
	}

	// 检查用户是否激活
	if !user.IsActive {
		logger.InfoF(c.Request.Context(), "[Auth] 登录失败 - 用户已禁用: %s", username)
		c.JSON(http.StatusForbidden, LoginResponse{ErrorMsg: ErrMsgUserInactive})
		return
	}

	// 更新最后登录时间
	if err := user.UpdateLastLogin(db.GetDB(c.Request.Context())); err != nil {
		logger.ErrorF(c.Request.Context(), "[Auth] 更新最后登录时间失败: %v", err)
		// 不影响登录流程，继续执行
	}

	// 创建会话
	session := sessions.Default(c)
	session.Set(SessionKeyUserID, user.ID)
	session.Set(SessionKeyUsername, user.Username)
	if err := session.Save(); err != nil {
		logger.ErrorF(c.Request.Context(), "[Auth] 保存会话失败: %v", err)
		c.JSON(http.StatusInternalServerError, LoginResponse{ErrorMsg: ErrMsgSessionError})
		return
	}

	logger.InfoF(c.Request.Context(), "[Auth] 登录成功: %d %s", user.ID, user.Username)
	c.JSON(http.StatusOK, LoginResponse{Data: user.ToUserInfo()})
}

// Logout 处理用户登出
// @Tags auth
// @Produce json
// @Success 200 {object} LogoutResponse
// @Router /api/v1/auth/logout [post]
func Logout(c *gin.Context) {
	session := sessions.Default(c)

	// 获取用户信息用于日志
	userID := session.Get(SessionKeyUserID)
	username := session.Get(SessionKeyUsername)

	// 清除会话
	session.Clear()
	if err := session.Save(); err != nil {
		logger.ErrorF(c.Request.Context(), "[Auth] 清除会话失败: %v", err)
		c.JSON(http.StatusInternalServerError, LogoutResponse{ErrorMsg: ErrMsgSessionError})
		return
	}

	logger.InfoF(c.Request.Context(), "[Auth] 登出成功: %v %v", userID, username)
	c.JSON(http.StatusOK, LogoutResponse{})
}

// GetUserInfo 获取当前登录用户信息
// @Tags auth
// @Produce json
// @Success 200 {object} UserInfoResponse
// @Router /api/v1/auth/user-info [get]
func GetUserInfo(c *gin.Context) {
	// 从上下文获取用户 ID
	userID := GetUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, UserInfoResponse{ErrorMsg: "未登录"})
		return
	}

	// 查询用户信息
	user, err := FindByID(db.GetDB(c.Request.Context()), userID)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[Auth] 获取用户信息失败: %v", err)
		c.JSON(http.StatusInternalServerError, UserInfoResponse{ErrorMsg: ErrMsgInternalError})
		return
	}

	c.JSON(http.StatusOK, UserInfoResponse{Data: user.ToUserInfo()})
}

// GetUserIDFromContext 从 Gin 上下文获取用户 ID
func GetUserIDFromContext(c *gin.Context) uint64 {
	session := sessions.Default(c)
	userID := session.Get(SessionKeyUserID)
	if userID == nil {
		return 0
	}

	// 处理不同类型的转换
	switch v := userID.(type) {
	case uint64:
		return v
	case int64:
		return uint64(v)
	case int:
		return uint64(v)
	case float64:
		return uint64(v)
	default:
		return 0
	}
}

// GetUsernameFromContext 从 Gin 上下文获取用户名
func GetUsernameFromContext(c *gin.Context) string {
	session := sessions.Default(c)
	username := session.Get(SessionKeyUsername)
	if username == nil {
		return ""
	}
	if s, ok := username.(string); ok {
		return s
	}
	return ""
}
