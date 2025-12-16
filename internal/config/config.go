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

package config

import (
	"log"
	"os"

	"github.com/spf13/viper"
)

var Config *configModel

func init() {
	// 加载配置文件路径
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	// 设置配置文件
	viper.SetConfigFile(configPath)
	viper.AutomaticEnv()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("[Config] read config failed: %v\n", err)
	}

	// 解析配置到结构体
	var c configModel
	if err := viper.Unmarshal(&c); err != nil {
		log.Fatalf("[Config] parse config failed: %v\n", err)
	}

	// 设置默认值
	setDefaults(&c)

	// 设置全局配置
	Config = &c
}

// setDefaults 设置配置默认值
func setDefaults(c *configModel) {
	// 数据库默认配置
	if c.Database.Type == "" {
		c.Database.Type = "sqlite"
	}
	if c.Database.SQLitePath == "" {
		c.Database.SQLitePath = "./data/seatunnel.db"
	}

	// 认证默认配置
	if c.Auth.DefaultAdminUsername == "" {
		c.Auth.DefaultAdminUsername = "admin"
	}
	if c.Auth.DefaultAdminPassword == "" {
		c.Auth.DefaultAdminPassword = "admin123"
	}
	if c.Auth.BcryptCost == 0 {
		c.Auth.BcryptCost = 10
	}
}

// GetDatabaseType 获取数据库类型
func GetDatabaseType() string {
	return Config.Database.Type
}

// GetSQLitePath 获取 SQLite 文件路径
func GetSQLitePath() string {
	return Config.Database.SQLitePath
}

// GetAuthConfig 获取认证配置
func GetAuthConfig() authConfig {
	return Config.Auth
}

// IsRedisEnabled 检查 Redis 是否启用
func IsRedisEnabled() bool {
	return Config.Redis.Enabled
}
