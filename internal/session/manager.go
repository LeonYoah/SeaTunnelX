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

// Package session 提供会话管理功能
package session

import (
	"fmt"
	"log"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-contrib/sessions/redis"
	redisClient "github.com/redis/go-redis/v9"
	"github.com/seatunnel/seatunnelX/internal/config"
)

// Store 全局会话存储实例
var Store SessionStore

// GinStore 全局 Gin 会话存储实例（用于 HTTP 会话）
var GinStore sessions.Store

// StoreType 会话存储类型
type StoreType string

const (
	// StoreTypeMemory 内存存储
	StoreTypeMemory StoreType = "memory"
	// StoreTypeRedis Redis 存储
	StoreTypeRedis StoreType = "redis"
)

// InitSessionStore 根据配置初始化会话存储
// 如果 Redis 启用，使用 Redis 存储；否则使用内存存储
func InitSessionStore() error {
	redisConfig := config.Config.Redis
	appConfig := config.Config.App

	if redisConfig.Enabled {
		// 使用 Redis 存储
		log.Println("[Session] 使用 Redis 会话存储")
		return initRedisStore(redisConfig, appConfig)
	}

	// 使用内存存储
	log.Println("[Session] 使用内存会话存储")
	return initMemoryStore(appConfig)
}

// initRedisStore 初始化 Redis 会话存储
func initRedisStore(redisConfig config.RedisConfig, appConfig config.AppConfig) error {
	// 创建 Redis 客户端用于自定义 SessionStore
	addr := fmt.Sprintf("%s:%d", redisConfig.Host, redisConfig.Port)
	client := redisClient.NewClient(&redisClient.Options{
		Addr:     addr,
		Username: redisConfig.Username,
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
		PoolSize: redisConfig.PoolSize,
	})

	// 初始化自定义 SessionStore
	Store = NewRedisStore(client, "seatunnel:session:")

	// 初始化 Gin 会话存储
	ginStore, err := redis.NewStoreWithDB(
		redisConfig.MinIdleConn,
		"tcp",
		addr,
		redisConfig.Username,
		redisConfig.Password,
		fmt.Sprintf("%d", redisConfig.DB),
		[]byte(appConfig.SessionSecret),
	)
	if err != nil {
		return fmt.Errorf("初始化 Redis Gin 会话存储失败: %w", err)
	}

	// 设置会话选项
	ginStore.Options(sessions.Options{
		Path:     "/",
		Domain:   appConfig.SessionDomain,
		MaxAge:   appConfig.SessionAge,
		HttpOnly: appConfig.SessionHttpOnly,
		Secure:   appConfig.SessionSecure,
	})

	GinStore = ginStore
	return nil
}

// initMemoryStore 初始化内存会话存储
func initMemoryStore(appConfig config.AppConfig) error {
	// 初始化自定义 SessionStore
	Store = NewMemoryStore()

	// 初始化 Gin Cookie 会话存储（内存模式下使用 Cookie 存储）
	ginStore := cookie.NewStore([]byte(appConfig.SessionSecret))

	// 设置会话选项
	ginStore.Options(sessions.Options{
		Path:     "/",
		Domain:   appConfig.SessionDomain,
		MaxAge:   appConfig.SessionAge,
		HttpOnly: appConfig.SessionHttpOnly,
		Secure:   appConfig.SessionSecure,
	})

	GinStore = ginStore
	return nil
}

// GetStoreType 获取当前会话存储类型
func GetStoreType() StoreType {
	if config.Config.Redis.Enabled {
		return StoreTypeRedis
	}
	return StoreTypeMemory
}
