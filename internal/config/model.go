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

type configModel struct {
	App        appConfig        `mapstructure:"app"`
	ProjectApp projectAppConfig `mapstructure:"projectApp"`
	Auth       authConfig       `mapstructure:"auth"`
	OAuth2     OAuth2Config     `mapstructure:"oauth2"`
	Database   databaseConfig   `mapstructure:"database"`
	Redis      redisConfig      `mapstructure:"redis"`
	Log        logConfig        `mapstructure:"log"`
	Schedule   scheduleConfig   `mapstructure:"schedule"`
	Worker     workerConfig     `mapstructure:"worker"`
	ClickHouse clickHouseConfig `mapstructure:"clickhouse"`
	LinuxDo    linuxDoConfig    `mapstructure:"linuxdo"`
}

// linuxDoConfig LinuxDo 配置（保留用于兼容）
type linuxDoConfig struct {
	ApiKey string `mapstructure:"api_key"`
}

// OAuth2Config OAuth2认证配置（保留用于兼容）
type OAuth2Config struct {
	ClientID              string `mapstructure:"client_id"`
	ClientSecret          string `mapstructure:"client_secret"`
	RedirectURI           string `mapstructure:"redirect_uri"`
	AuthorizationEndpoint string `mapstructure:"authorization_endpoint"`
	TokenEndpoint         string `mapstructure:"token_endpoint"`
	UserEndpoint          string `mapstructure:"user_endpoint"`
}

// appConfig 应用基本配置
type appConfig struct {
	AppName           string `mapstructure:"app_name"`
	Env               string `mapstructure:"env"`
	Addr              string `mapstructure:"addr"`
	APIPrefix         string `mapstructure:"api_prefix"`
	SessionCookieName string `mapstructure:"session_cookie_name"`
	SessionSecret     string `mapstructure:"session_secret"`
	SessionDomain     string `mapstructure:"session_domain"`
	SessionAge        int    `mapstructure:"session_age"`
	SessionHttpOnly   bool   `mapstructure:"session_http_only"`
	SessionSecure     bool   `mapstructure:"session_secure"`
}

// projectAppConfig 项目相关配置
type projectAppConfig struct {
	HiddenThreshold        uint8 `mapstructure:"hidden_threshold"`
	DeductionPerOffense    uint8 `mapstructure:"deduction_per_offense"`
	CreateProjectRateLimit []struct {
		IntervalSeconds int `mapstructure:"interval_seconds"`
		MaxCount        int `mapstructure:"max_count"`
	} `mapstructure:"create_project_rate_limit"`
}

// authConfig 认证配置
type authConfig struct {
	DefaultAdminUsername string `mapstructure:"default_admin_username"`
	DefaultAdminPassword string `mapstructure:"default_admin_password"`
	BcryptCost           int    `mapstructure:"bcrypt_cost"`
}

// databaseConfig 数据库配置
type databaseConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	Type            string `mapstructure:"type"`        // sqlite, mysql, postgres
	SQLitePath      string `mapstructure:"sqlite_path"` // SQLite 文件路径
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	Database        string `mapstructure:"database"`
	MaxIdleConn     int    `mapstructure:"max_idle_conn"`
	MaxOpenConn     int    `mapstructure:"max_open_conn"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
	LogLevel        string `mapstructure:"log_level"`
}

// clickHouseConfig ClickHouse 配置
type clickHouseConfig struct {
	Enabled         bool     `mapstructure:"enabled"`
	Hosts           []string `mapstructure:"hosts"`
	Username        string   `mapstructure:"username"`
	Password        string   `mapstructure:"password"`
	Database        string   `mapstructure:"database"`
	MaxIdleConn     int      `mapstructure:"max_idle_conn"`
	MaxOpenConn     int      `mapstructure:"max_open_conn"`
	ConnMaxLifetime int      `mapstructure:"conn_max_lifetime"`
	DialTimeout     int      `mapstructure:"dial_timeout"`
}

// redisConfig Redis配置
type redisConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConn  int    `mapstructure:"min_idle_conn"`
	DialTimeout  int    `mapstructure:"dial_timeout"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

// logConfig 日志配置
type logConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	FilePath   string `mapstructure:"file_path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxAge     int    `mapstructure:"max_age"`
	MaxBackups int    `mapstructure:"max_backups"`
	Compress   bool   `mapstructure:"compress"`
}

// scheduleConfig 定时任务配置
type scheduleConfig struct {
	UserBadgeScoreDispatchIntervalSeconds int    `mapstructure:"user_badge_score_dispatch_interval_seconds"`
	UpdateUserBadgeScoresTaskCron         string `mapstructure:"update_user_badges_scores_task_cron"`
	UpdateAllBadgesTaskCron               string `mapstructure:"update_all_badges_task_cron"`
}

// workerConfig 工作配置
type workerConfig struct {
	Concurrency int `mapstructure:"concurrency"`
}
