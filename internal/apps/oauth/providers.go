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

package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/seatunnel/seatunnelX/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// OAuthProvider OAuth 提供商类型
type OAuthProvider string

const (
	ProviderGitHub OAuthProvider = "github"
	ProviderGoogle OAuthProvider = "google"
)

// OAuthProviderManager OAuth 提供商管理器
type OAuthProviderManager struct {
	providers map[OAuthProvider]*oauth2.Config
}

// 全局提供商管理器
var providerManager *OAuthProviderManager

// InitOAuthProviders 初始化 OAuth 提供商
func InitOAuthProviders() {
	providerManager = &OAuthProviderManager{
		providers: make(map[OAuthProvider]*oauth2.Config),
	}

	// 初始化 GitHub OAuth
	githubConfig := config.Config.OAuthProviders.GitHub
	if githubConfig.Enabled && githubConfig.ClientID != "" {
		providerManager.providers[ProviderGitHub] = &oauth2.Config{
			ClientID:     githubConfig.ClientID,
			ClientSecret: githubConfig.ClientSecret,
			RedirectURL:  githubConfig.RedirectURI,
			Scopes:       []string{"user:email", "read:user"},
			Endpoint:     github.Endpoint,
		}
	}

	// 初始化 Google OAuth
	googleConfig := config.Config.OAuthProviders.Google
	if googleConfig.Enabled && googleConfig.ClientID != "" {
		providerManager.providers[ProviderGoogle] = &oauth2.Config{
			ClientID:     googleConfig.ClientID,
			ClientSecret: googleConfig.ClientSecret,
			RedirectURL:  googleConfig.RedirectURI,
			Scopes:       []string{"openid", "profile", "email"},
			Endpoint:     google.Endpoint,
		}
	}
}

// GetProvider 获取指定提供商的 OAuth 配置
func GetProvider(provider OAuthProvider) (*oauth2.Config, error) {
	if providerManager == nil {
		return nil, errors.New("OAuth providers not initialized")
	}

	conf, ok := providerManager.providers[provider]
	if !ok {
		return nil, fmt.Errorf("OAuth provider '%s' not configured or disabled", provider)
	}

	return conf, nil
}

// IsProviderEnabled 检查提供商是否启用
func IsProviderEnabled(provider OAuthProvider) bool {
	if providerManager == nil {
		return false
	}
	_, ok := providerManager.providers[provider]
	return ok
}

// GetEnabledProviders 获取所有启用的提供商
func GetEnabledProviders() []OAuthProvider {
	if providerManager == nil {
		return nil
	}

	providers := make([]OAuthProvider, 0, len(providerManager.providers))
	for p := range providerManager.providers {
		providers = append(providers, p)
	}
	return providers
}

// OAuthUserInfo OAuth 用户信息（统一格式）
type OAuthUserInfo struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Provider  string `json:"provider"`
}

// FetchUserInfo 获取用户信息
func FetchUserInfo(ctx context.Context, provider OAuthProvider, token *oauth2.Token) (*OAuthUserInfo, error) {
	switch provider {
	case ProviderGitHub:
		return fetchGitHubUserInfo(ctx, token)
	case ProviderGoogle:
		return fetchGoogleUserInfo(ctx, token)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// fetchGitHubUserInfo 获取 GitHub 用户信息
func fetchGitHubUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("failed to get GitHub user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %s", string(body))
	}

	var githubUser struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&githubUser); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub user info: %w", err)
	}

	return &OAuthUserInfo{
		ID:        fmt.Sprintf("%d", githubUser.ID),
		Username:  githubUser.Login,
		Email:     githubUser.Email,
		Name:      githubUser.Name,
		AvatarURL: githubUser.AvatarURL,
		Provider:  string(ProviderGitHub),
	}, nil
}

// fetchGoogleUserInfo 获取 Google 用户信息
func fetchGoogleUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get Google user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Google API error: %s", string(body))
	}

	var googleUser struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		VerifiedEmail bool   `json:"verified_email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, fmt.Errorf("failed to decode Google user info: %w", err)
	}

	// Google 没有 username，使用 email 前缀
	username := googleUser.Email
	if idx := len(googleUser.Email); idx > 0 {
		for i, c := range googleUser.Email {
			if c == '@' {
				username = googleUser.Email[:i]
				break
			}
		}
	}

	return &OAuthUserInfo{
		ID:        googleUser.ID,
		Username:  username,
		Email:     googleUser.Email,
		Name:      googleUser.Name,
		AvatarURL: googleUser.Picture,
		Provider:  string(ProviderGoogle),
	}, nil
}
