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
	App            AppConfig            `mapstructure:"app"`
	ProjectApp     projectAppConfig     `mapstructure:"projectApp"`
	Auth           authConfig           `mapstructure:"auth"`
	OAuth2         OAuth2Config         `mapstructure:"oauth2"`
	OAuthProviders OAuthProvidersConfig `mapstructure:"oauth_providers"`
	Database       DatabaseConfig       `mapstructure:"database"`
	Redis          RedisConfig          `mapstructure:"redis"`
	Storage        StorageConfig        `mapstructure:"storage"`
	GRPC           GRPCConfig           `mapstructure:"grpc"`
	Log            logConfig            `mapstructure:"log"`
	Telemetry      TelemetryConfig      `mapstructure:"telemetry"`
	Schedule       scheduleConfig       `mapstructure:"schedule"`
	Worker         workerConfig         `mapstructure:"worker"`
	ClickHouse     clickHouseConfig     `mapstructure:"clickhouse"`
	Legacy         legacyConfig         `mapstructure:"legacy"`
}

// legacyConfig 旧版配置（保留用于兼容）
type legacyConfig struct {
	ApiKey string `mapstructure:"api_key"`
}

// OAuth2Config OAuth2认证配置（保留用于兼容旧配置）
type OAuth2Config struct {
	ClientID              string `mapstructure:"client_id"`
	ClientSecret          string `mapstructure:"client_secret"`
	RedirectURI           string `mapstructure:"redirect_uri"`
	AuthorizationEndpoint string `mapstructure:"authorization_endpoint"`
	TokenEndpoint         string `mapstructure:"token_endpoint"`
	UserEndpoint          string `mapstructure:"user_endpoint"`
}

// OAuthProviderConfig 单个 OAuth 提供商配置
type OAuthProviderConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURI  string `mapstructure:"redirect_uri"`
}

// OAuthProvidersConfig 多 OAuth 提供商配置
type OAuthProvidersConfig struct {
	GitHub OAuthProviderConfig `mapstructure:"github"`
	Google OAuthProviderConfig `mapstructure:"google"`
}

// AppConfig 应用基本配置（导出供其他包使用）
// AppConfig holds basic application configuration (exported for other packages)
type AppConfig struct {
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

	// ExternalURL is the external URL for accessing the Control Plane.
	// ExternalURL 是访问 Control Plane 的外部 URL。
	// This is used for generating Agent install commands and other external references.
	// 用于生成 Agent 安装命令和其他外部引用。
	// Example: "http://192.168.1.100:8000" or "https://seatunnel.example.com"
	// 示例: "http://192.168.1.100:8000" 或 "https://seatunnel.example.com"
	ExternalURL string `mapstructure:"external_url"`
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

// DatabaseConfig 数据库配置（导出供其他包使用）
type DatabaseConfig struct {
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

// RedisConfig Redis配置（导出供其他包使用）
type RedisConfig struct {
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

// GRPCConfig gRPC 服务器配置
// GRPCConfig holds configuration for the gRPC server
type GRPCConfig struct {
	// Enabled indicates whether gRPC server is enabled
	// Enabled 表示是否启用 gRPC 服务器
	Enabled bool `mapstructure:"enabled"`

	// Port is the port number for the gRPC server (default: 9000)
	// Port 是 gRPC 服务器的端口号（默认：9000）
	Port int `mapstructure:"port"`

	// TLSEnabled indicates whether TLS is enabled
	// TLSEnabled 表示是否启用 TLS
	TLSEnabled bool `mapstructure:"tls_enabled"`

	// CertFile is the path to the TLS certificate file
	// CertFile 是 TLS 证书文件的路径
	CertFile string `mapstructure:"cert_file"`

	// KeyFile is the path to the TLS key file
	// KeyFile 是 TLS 密钥文件的路径
	KeyFile string `mapstructure:"key_file"`

	// CAFile is the path to the CA certificate file for client verification
	// CAFile 是用于客户端验证的 CA 证书文件路径
	CAFile string `mapstructure:"ca_file"`

	// MaxRecvMsgSize is the maximum receive message size in MB (default: 16)
	// MaxRecvMsgSize 是最大接收消息大小（MB，默认：16）
	MaxRecvMsgSize int `mapstructure:"max_recv_msg_size"`

	// MaxSendMsgSize is the maximum send message size in MB (default: 16)
	// MaxSendMsgSize 是最大发送消息大小（MB，默认：16）
	MaxSendMsgSize int `mapstructure:"max_send_msg_size"`

	// HeartbeatInterval is the heartbeat interval to send to Agents (seconds, default: 10)
	// HeartbeatInterval 是发送给 Agent 的心跳间隔（秒，默认：10）
	HeartbeatInterval int `mapstructure:"heartbeat_interval"`

	// HeartbeatTimeout is the timeout for considering an Agent offline (seconds, default: 30)
	// HeartbeatTimeout 是判断 Agent 离线的超时时间（秒，默认：30）
	HeartbeatTimeout int `mapstructure:"heartbeat_timeout"`
}

// StorageConfig 存储配置（本地文件存储目录）
type StorageConfig struct {
	// BaseDir 基础存储目录，其他目录默认相对于此目录
	// BaseDir is the base storage directory, other directories are relative to this
	BaseDir string `mapstructure:"base_dir"`

	// PackagesDir SeaTunnel 安装包存储目录
	// PackagesDir is the directory for SeaTunnel installation packages
	PackagesDir string `mapstructure:"packages_dir"`

	// PluginsDir 插件包存储目录（connector jars, lib dependencies）
	// PluginsDir is the directory for plugin packages
	PluginsDir string `mapstructure:"plugins_dir"`

	// TempDir 临时文件目录（下载中的文件等）
	// TempDir is the directory for temporary files
	TempDir string `mapstructure:"temp_dir"`

	// MaxPackageSize 最大安装包大小（MB），默认 2048MB (2GB)
	// MaxPackageSize is the maximum package size in MB
	MaxPackageSize int64 `mapstructure:"max_package_size"`

	// CleanupIntervalHours 临时文件清理间隔（小时），默认 24
	// CleanupIntervalHours is the interval for cleaning up temp files
	CleanupIntervalHours int `mapstructure:"cleanup_interval_hours"`
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

// TelemetryConfig 遥测配置
// TelemetryConfig holds telemetry/tracing configuration
type TelemetryConfig struct {
	// Enabled indicates whether OpenTelemetry tracing is enabled
	// Enabled 表示是否启用 OpenTelemetry 追踪
	Enabled bool `mapstructure:"enabled"`

	// Endpoint is the OTLP collector endpoint (default: localhost:4317)
	// Endpoint 是 OTLP 收集器端点（默认：localhost:4317）
	Endpoint string `mapstructure:"endpoint"`
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
