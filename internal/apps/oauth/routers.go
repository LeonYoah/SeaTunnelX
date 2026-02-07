/*
 * MIT License
 *
 * Copyright (c) 2025 linux.do
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

package oauth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/seatunnel/seatunnelX/internal/apps/auth"
	"github.com/seatunnel/seatunnelX/internal/db"
	"github.com/seatunnel/seatunnelX/internal/logger"
	"github.com/seatunnel/seatunnelX/internal/session"
)

type GetLoginURLResponse struct {
	ErrorMsg string `json:"error_msg"`
	Data     string `json:"data"`
}

// GetLoginURL godoc
// @Tags oauth
// @Param provider query string false "OAuth provider (github, google)"
// @Produce json
// @Success 200 {object} GetLoginURLResponse
// @Router /api/v1/oauth/login [get]
func GetLoginURL(c *gin.Context) {
	// 获取 provider 参数
	providerStr := c.Query("provider")
	if providerStr == "" {
		providerStr = "github" // 默认使用 GitHub
	}

	provider := OAuthProvider(strings.ToLower(providerStr))

	// 检查提供商是否启用
	if !IsProviderEnabled(provider) {
		c.JSON(http.StatusBadRequest, GetLoginURLResponse{
			ErrorMsg: fmt.Sprintf("OAuth provider '%s' is not enabled", provider),
		})
		return
	}

	// 获取提供商配置
	oauthConfig, err := GetProvider(provider)
	if err != nil {
		c.JSON(http.StatusBadRequest, GetLoginURLResponse{ErrorMsg: err.Error()})
		return
	}

	// 生成 state，包含 provider 信息
	state := fmt.Sprintf("%s:%s", provider, uuid.NewString())
	key := fmt.Sprintf(OAuthStateCacheKeyFormat, state)

	// 存储 State 到会话存储（支持内存或 Redis）
	err = session.Store.Set(c.Request.Context(), key, state, OAuthStateCacheKeyExpiration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, GetLoginURLResponse{ErrorMsg: err.Error()})
		return
	}

	// 构造登录 URL
	c.JSON(http.StatusOK, GetLoginURLResponse{Data: oauthConfig.AuthCodeURL(state)})
}

type CallbackRequest struct {
	State string `json:"state"`
	Code  string `json:"code"`
}

type CallbackResponse struct {
	ErrorMsg string `json:"error_msg"`
	Data     any    `json:"data"`
}

// Callback godoc
// @Tags oauth
// @Param request body CallbackRequest true "request body"
// @Produce json
// @Success 200 {object} CallbackResponse
// @Router /api/v1/oauth/callback [post]
func Callback(c *gin.Context) {
	// init req
	var req CallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, CallbackResponse{ErrorMsg: err.Error()})
		return
	}

	// check state（使用会话存储，支持内存或 Redis）
	key := fmt.Sprintf(OAuthStateCacheKeyFormat, req.State)
	storedState, err := session.Store.Get(c.Request.Context(), key)
	if err != nil || storedState != req.State {
		c.JSON(http.StatusBadRequest, CallbackResponse{ErrorMsg: InvalidState})
		return
	}

	// 删除已使用的 state
	_ = session.Store.Delete(c.Request.Context(), key)

	// 从 state 中解析 provider
	parts := strings.SplitN(req.State, ":", 2)
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, CallbackResponse{ErrorMsg: "Invalid state format"})
		return
	}
	provider := OAuthProvider(parts[0])

	// 获取提供商配置
	oauthConfig, err := GetProvider(provider)
	if err != nil {
		c.JSON(http.StatusBadRequest, CallbackResponse{ErrorMsg: err.Error()})
		return
	}

	// 交换 code 获取 token
	token, err := oauthConfig.Exchange(c.Request.Context(), req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CallbackResponse{
			ErrorMsg: fmt.Sprintf("Failed to exchange token: %v", err),
		})
		return
	}

	// 获取用户信息
	userInfo, err := FetchUserInfo(c.Request.Context(), provider, token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CallbackResponse{
			ErrorMsg: fmt.Sprintf("Failed to fetch user info: %v", err),
		})
		return
	}

	// 查找或创建用户
	user, err := findOrCreateOAuthUser(c.Request.Context(), userInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CallbackResponse{ErrorMsg: err.Error()})
		return
	}

	// bind to session
	ginSession := sessions.Default(c)
	ginSession.Set(UserIDKey, user.ID)
	ginSession.Set(UserNameKey, user.Username)
	if err := ginSession.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, CallbackResponse{ErrorMsg: err.Error()})
		return
	}

	// response
	c.JSON(http.StatusOK, CallbackResponse{})
	logger.InfoF(c.Request.Context(), "[OAuthCallback] provider=%s user_id=%d username=%s", provider, user.ID, user.Username)
}

// findOrCreateOAuthUser 查找或创建 OAuth 用户（使用统一的 auth.User 表）
func findOrCreateOAuthUser(ctx context.Context, info *OAuthUserInfo) (*auth.User, error) {
	var user auth.User

	// 先尝试通过 OAuth ID 查找
	oauthID := fmt.Sprintf("%s:%s", info.Provider, info.ID)
	tx := db.DB(ctx).Where("oauth_id = ?", oauthID).First(&user)

	if tx.Error == nil {
		// 用户已存在，更新信息
		user.AvatarURL = info.AvatarURL
		if info.Name != "" {
			user.Nickname = info.Name
		}
		db.DB(ctx).Save(&user)
		return &user, nil
	}

	// 尝试通过用户名查找
	tx = db.DB(ctx).Where("username = ?", info.Username).First(&user)
	if tx.Error == nil {
		// 用户名已存在，关联 OAuth
		user.OAuthID = oauthID
		user.AvatarURL = info.AvatarURL
		db.DB(ctx).Save(&user)
		return &user, nil
	}

	// 创建新用户
	user = auth.User{
		Username:  info.Username,
		Nickname:  info.Name,
		OAuthID:   oauthID,
		AvatarURL: info.AvatarURL,
		IsActive:  true,
	}

	if err := db.DB(ctx).Create(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &user, nil
}

// GetEnabledProvidersHandler 获取启用的 OAuth 提供商列表
// @Tags oauth
// @Produce json
// @Success 200 {object} map[string]any
// @Router /api/v1/oauth/providers [get]
func GetEnabledProvidersHandler(c *gin.Context) {
	providers := GetEnabledProviders()
	providerNames := make([]string, len(providers))
	for i, p := range providers {
		providerNames[i] = string(p)
	}
	c.JSON(http.StatusOK, gin.H{
		"error_msg": "",
		"data":      providerNames,
	})
}
