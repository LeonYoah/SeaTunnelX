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

package plugin

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Common service errors / 常见服务错误
var (
	ErrInvalidVersion      = errors.New("invalid version / 无效的版本号")
	ErrInvalidMirror       = errors.New("invalid mirror source / 无效的镜像源")
	ErrPluginNotAvailable  = errors.New("plugin not available / 插件不可用")
	ErrClusterNotFound     = errors.New("cluster not found / 集群未找到")
	ErrPluginAlreadyExists = errors.New("plugin already installed / 插件已安装")
	ErrVersionMismatch     = errors.New("plugin version does not match cluster version / 插件版本与集群版本不匹配")
	ErrClusterVersionEmpty = errors.New("cluster version is not set / 集群版本未设置")
)

// SeaTunnel documentation URLs for fetching plugin lists
// SeaTunnel 文档 URL，用于获取插件列表
const (
	SeaTunnelDocsBaseURL = "https://seatunnel.apache.org/docs"
	PluginCacheDuration  = 1 * time.Hour
)

// ClusterGetter is an interface for getting cluster information.
// ClusterGetter 是获取集群信息的接口。
type ClusterGetter interface {
	GetClusterVersion(ctx context.Context, clusterID uint) (string, error)
}

// ClusterNodeInfo represents node information needed for plugin installation.
// ClusterNodeInfo 表示插件安装所需的节点信息。
type ClusterNodeInfo struct {
	NodeID     uint   // Node ID / 节点 ID
	HostID     uint   // Host ID / 主机 ID
	InstallDir string // SeaTunnel installation directory / SeaTunnel 安装目录
}

// ClusterNodeGetter is an interface for getting cluster nodes.
// ClusterNodeGetter 是获取集群节点的接口。
type ClusterNodeGetter interface {
	GetClusterNodes(ctx context.Context, clusterID uint) ([]ClusterNodeInfo, error)
}

// HostInfoGetter is an interface for getting host information.
// HostInfoGetter 是获取主机信息的接口。
type HostInfoGetter interface {
	GetHostAgentID(ctx context.Context, hostID uint) (string, error)
}

// Service provides plugin management functionality.
// Service 提供插件管理功能。
type Service struct {
	repo          *Repository
	clusterGetter ClusterGetter
	downloader    *Downloader

	// agentCommandSender is used to send commands to agents for plugin installation
	// agentCommandSender 用于向 Agent 发送命令进行插件安装
	agentCommandSender AgentCommandSender

	// clusterNodeGetter is used to get cluster nodes for plugin installation
	// clusterNodeGetter 用于获取集群节点进行插件安装
	clusterNodeGetter ClusterNodeGetter

	// hostInfoGetter is used to get host information (including AgentID)
	// hostInfoGetter 用于获取主机信息（包括 AgentID）
	hostInfoGetter HostInfoGetter

	// Plugin cache / 插件缓存
	cachedPlugins    map[string][]Plugin // key: version
	pluginsCacheTime map[string]time.Time
	pluginsMu        sync.RWMutex

	// Installation progress tracking / 安装进度跟踪
	installProgress   map[string]*PluginInstallStatus // key: clusterID:pluginName
	installProgressMu sync.RWMutex
}

// NewService creates a new Service instance.
// NewService 创建一个新的 Service 实例。
func NewService(repo *Repository) *Service {
	return &Service{
		repo:             repo,
		downloader:       NewDownloader("./lib/plugins"),
		cachedPlugins:    make(map[string][]Plugin),
		pluginsCacheTime: make(map[string]time.Time),
		installProgress:  make(map[string]*PluginInstallStatus),
	}
}

// NewServiceWithDownloader creates a new Service instance with a custom downloader.
// NewServiceWithDownloader 创建一个带有自定义下载器的新 Service 实例。
func NewServiceWithDownloader(repo *Repository, pluginsDir string) *Service {
	return &Service{
		repo:             repo,
		downloader:       NewDownloader(pluginsDir),
		cachedPlugins:    make(map[string][]Plugin),
		pluginsCacheTime: make(map[string]time.Time),
		installProgress:  make(map[string]*PluginInstallStatus),
	}
}

// SetClusterGetter sets the cluster getter for version validation.
// SetClusterGetter 设置集群获取器用于版本校验。
func (s *Service) SetClusterGetter(getter ClusterGetter) {
	s.clusterGetter = getter
}

// ==================== Available Plugins 可用插件 ====================

// ListAvailablePlugins returns available plugins from Maven repository.
// ListAvailablePlugins 从 Maven 仓库获取可用插件列表。
// Supports multiple mirror sources (apache/aliyun/huaweicloud).
// 支持多仓库源（apache/aliyun/huaweicloud）。
func (s *Service) ListAvailablePlugins(ctx context.Context, version string, mirror MirrorSource) (*AvailablePluginsResponse, error) {
	if version == "" {
		version = "2.3.12" // Default version / 默认版本
	}

	if mirror == "" {
		mirror = MirrorSourceApache // Default mirror / 默认镜像源
	}

	// Validate mirror source / 验证镜像源
	if _, ok := MirrorURLs[mirror]; !ok {
		return nil, ErrInvalidMirror
	}

	// Get plugins (from cache, online, or fallback)
	// 获取插件（从缓存、在线或备用列表）
	plugins := s.getPlugins(ctx, version)

	return &AvailablePluginsResponse{
		Plugins: plugins,
		Total:   len(plugins),
		Version: version,
		Mirror:  string(mirror),
	}, nil
}

// getPlugins returns the plugin list, using cache if valid, otherwise fetching from SeaTunnel docs.
// getPlugins 返回插件列表，如果缓存有效则使用缓存，否则从 SeaTunnel 文档获取。
func (s *Service) getPlugins(ctx context.Context, version string) []Plugin {
	s.pluginsMu.RLock()
	// Check if cache is valid / 检查缓存是否有效
	if plugins, ok := s.cachedPlugins[version]; ok {
		if cacheTime, exists := s.pluginsCacheTime[version]; exists {
			if time.Since(cacheTime) < PluginCacheDuration {
				s.pluginsMu.RUnlock()
				return plugins
			}
		}
	}
	s.pluginsMu.RUnlock()

	// Try to fetch from SeaTunnel docs / 尝试从 SeaTunnel 文档获取
	plugins, err := s.fetchPluginsFromDocs(ctx, version)
	if err != nil {
		// Use fallback plugins on error / 出错时使用备用插件列表
		return getAvailablePluginsForVersion(version)
	}

	// Update cache / 更新缓存
	s.pluginsMu.Lock()
	s.cachedPlugins[version] = plugins
	s.pluginsCacheTime[version] = time.Now()
	s.pluginsMu.Unlock()

	return plugins
}

// fetchPluginsFromDocs fetches plugin list from SeaTunnel documentation.
// fetchPluginsFromDocs 从 SeaTunnel 文档获取插件列表。
func (s *Service) fetchPluginsFromDocs(ctx context.Context, version string) ([]Plugin, error) {
	var allPlugins []Plugin

	// Fetch source connectors / 获取Source连接器
	sourcePlugins, err := s.fetchPluginsByCategory(ctx, version, PluginCategorySource)
	if err == nil {
		allPlugins = append(allPlugins, sourcePlugins...)
	}

	// Fetch sink connectors / 获取Sink连接器
	sinkPlugins, err := s.fetchPluginsByCategory(ctx, version, PluginCategorySink)
	if err == nil {
		allPlugins = append(allPlugins, sinkPlugins...)
	}

	// Fetch transform connectors / 获取数据转换连接器
	transformPlugins, err := s.fetchPluginsByCategory(ctx, version, PluginCategoryTransform)
	if err == nil {
		allPlugins = append(allPlugins, transformPlugins...)
	}

	if len(allPlugins) == 0 {
		return nil, fmt.Errorf("no plugins found from docs")
	}

	return allPlugins, nil
}

// fetchPluginsByCategory fetches plugins of a specific category from SeaTunnel docs.
// fetchPluginsByCategory 从 SeaTunnel 文档获取特定分类的插件。
func (s *Service) fetchPluginsByCategory(ctx context.Context, version string, category PluginCategory) ([]Plugin, error) {
	// Build URL based on category / 根据分类构建 URL
	var url string
	switch category {
	case PluginCategorySource:
		url = fmt.Sprintf("%s/%s/connector-v2/source", SeaTunnelDocsBaseURL, version)
	case PluginCategorySink:
		url = fmt.Sprintf("%s/%s/connector-v2/sink", SeaTunnelDocsBaseURL, version)
	case PluginCategoryTransform:
		url = fmt.Sprintf("%s/%s/transform-v2", SeaTunnelDocsBaseURL, version)
	default:
		return nil, fmt.Errorf("unknown category: %s", category)
	}

	// Create HTTP request with timeout / 创建带超时的 HTTP 请求
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch plugins: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body / 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse HTML to extract plugin names / 解析 HTML 提取插件名称
	return parsePluginsFromHTML(string(body), version, category), nil
}

// parsePluginsFromHTML parses plugin names from SeaTunnel docs HTML.
// parsePluginsFromHTML 从 SeaTunnel 文档 HTML 解析插件名称。
func parsePluginsFromHTML(html string, version string, category PluginCategory) []Plugin {
	var plugins []Plugin

	// Pattern to match connector links in the sidebar
	// 匹配侧边栏中连接器链接的模式
	// Example: <a href="/docs/2.3.12/connector-v2/source/Jdbc">Jdbc</a>
	var pattern string
	switch category {
	case PluginCategorySource:
		pattern = `<a[^>]*href="[^"]*connector-v2/source/([^"]+)"[^>]*>([^<]+)</a>`
	case PluginCategorySink:
		pattern = `<a[^>]*href="[^"]*connector-v2/sink/([^"]+)"[^>]*>([^<]+)</a>`
	case PluginCategoryTransform:
		pattern = `<a[^>]*href="[^"]*transform-v2/([^"]+)"[^>]*>([^<]+)</a>`
	}

	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(html, -1)

	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) >= 3 {
			// urlPath preserves original case for doc URL / urlPath 保留原始大小写用于文档 URL
			urlPath := match[1]
			// name is lowercase for internal identification / name 小写用于内部标识
			name := strings.ToLower(urlPath)
			displayName := match[2]

			// Skip duplicates and common pages / 跳过重复项和通用页面
			if seen[name] || name == "common-options" || name == "about" {
				continue
			}
			seen[name] = true

			plugin := Plugin{
				Name:        name,
				DisplayName: displayName,
				Category:    category,
				Version:     version,
				GroupID:     "org.apache.seatunnel",
				ArtifactID:  getArtifactID(name),
				// Use original urlPath for doc URL to preserve case / 使用原始 urlPath 构建文档 URL 以保留大小写
				DocURL: buildDocURL(version, category, urlPath),
			}

			// Add description based on category / 根据分类添加描述
			plugin.Description = generatePluginDescription(displayName, category)

			plugins = append(plugins, plugin)
		}
	}

	return plugins
}

// getArtifactID returns the correct Maven artifact ID for a plugin name.
// pluginArtifactMappings contains all special plugin name to artifact ID mappings.
// pluginArtifactMappings 包含所有特殊的插件名称到 artifact ID 的映射。
// This mapping is based on SeaTunnel's Maven repository structure.
// 此映射基于 SeaTunnel 的 Maven 仓库结构。
var pluginArtifactMappings = map[string]string{
	// CDC connectors / CDC 连接器
	"mysql-cdc":     "connector-cdc-mysql",
	"postgres-cdc":  "connector-cdc-postgres",
	"sqlserver-cdc": "connector-cdc-sqlserver",
	"oracle-cdc":    "connector-cdc-oracle",
	"mongodb-cdc":   "connector-cdc-mongodb",
	"tidb-cdc":      "connector-cdc-tidb",
	"db2-cdc":       "connector-cdc-db2",
	"opengauss-cdc": "connector-cdc-opengauss",

	// File connectors / 文件连接器
	"localfile": "connector-file-local",
	"hdfsfile":  "connector-file-hadoop",
	"s3file":    "connector-file-s3",
	"ossfile":   "connector-file-oss",
	"ftpfile":   "connector-file-ftp",
	"sftpfile":  "connector-file-sftp",
	"cosfile":   "connector-file-cos",
	"obsfile":   "connector-file-obs",

	// HTTP-based connectors / 基于 HTTP 的连接器
	"http":      "connector-http-base",
	"feishu":    "connector-http-feishu",
	"github":    "connector-http-github",
	"gitlab":    "connector-http-gitlab",
	"jira":      "connector-http-jira",
	"klaviyo":   "connector-http-klaviyo",
	"lemlist":   "connector-http-lemlist",
	"myhours":   "connector-http-myhours",
	"notion":    "connector-http-notion",
	"onesignal": "connector-http-onesignal",
	"persistiq": "connector-http-persistiq",
	"wechat":    "connector-http-wechat",

	// JDBC connector and JDBC-based databases / JDBC 连接器和基于 JDBC 的数据库
	// All these databases use connector-jdbc with their respective drivers
	// 所有这些数据库都使用 connector-jdbc 配合各自的驱动
	"jdbc":       "connector-jdbc",
	"mysql":      "connector-jdbc", // Driver: com.mysql.cj.jdbc.Driver
	"postgresql": "connector-jdbc", // Driver: org.postgresql.Driver
	"dm":         "connector-jdbc", // Driver: dm.jdbc.driver.DmDriver (达梦数据库)
	"phoenix":    "connector-jdbc", // Driver: org.apache.phoenix.queryserver.client.Driver
	"sqlserver":  "connector-jdbc", // Driver: com.microsoft.sqlserver.jdbc.SQLServerDriver
	"oracle":     "connector-jdbc", // Driver: oracle.jdbc.OracleDriver
	"sqlite":     "connector-jdbc", // Driver: org.sqlite.JDBC
	"gbase8a":    "connector-jdbc", // Driver: com.gbase.jdbc.Driver
	"starrocks":  "connector-jdbc", // Driver: com.mysql.cj.jdbc.Driver (MySQL protocol)
	"db2":        "connector-jdbc", // Driver: com.ibm.db2.jcc.DB2Driver
	"tablestore": "connector-jdbc", // Driver: com.alicloud.openservices.tablestore.jdbc.OTSDriver
	"saphana":    "connector-jdbc", // Driver: com.sap.db.jdbc.Driver
	"doris":      "connector-jdbc", // Driver: com.mysql.cj.jdbc.Driver (MySQL protocol)
	"teradata":   "connector-jdbc", // Driver: com.teradata.jdbc.TeraDriver
	"snowflake":  "connector-jdbc", // Driver: net.snowflake.client.jdbc.SnowflakeDriver
	"redshift":   "connector-jdbc", // Driver: com.amazon.redshift.jdbc42.Driver
	"vertica":    "connector-jdbc", // Driver: com.vertica.jdbc.Driver
	"kingbase":   "connector-jdbc", // Driver: com.kingbase8.Driver (人大金仓)
	"oceanbase":  "connector-jdbc", // Driver: com.oceanbase.jdbc.Driver
	"hive":       "connector-jdbc", // Driver: org.apache.hive.jdbc.HiveDriver
	"xugu":       "connector-jdbc", // Driver: com.xugu.cloudjdbc.Driver (虚谷数据库)
	"iris":       "connector-jdbc", // Driver: com.intersystems.jdbc.IRISDriver
	"opengauss":  "connector-jdbc", // Driver: org.opengauss.Driver
	"highgo":     "connector-jdbc", // Driver: com.highgo.jdbc.Driver (瀚高数据库)
	"presto":     "connector-jdbc", // Driver: com.facebook.presto.jdbc.PrestoDriver
	"trino":      "connector-jdbc", // Driver: io.trino.jdbc.TrinoDriver
}

// getArtifactID returns the correct Maven artifact ID for a plugin name.
// getArtifactID 返回插件名称对应的正确 Maven artifact ID。
// Some plugins have special naming conventions that differ from the standard connector-${name} pattern.
// 某些插件有特殊的命名约定，与标准的 connector-${name} 模式不同。
func getArtifactID(name string) string {
	// Check special mappings first / 首先检查特殊映射
	if artifactID, ok := pluginArtifactMappings[name]; ok {
		return artifactID
	}

	// Default: connector-${name} / 默认：connector-${name}
	return fmt.Sprintf("connector-%s", name)
}

// buildDocURL builds the documentation URL for a plugin.
// buildDocURL 构建插件的文档 URL。
func buildDocURL(version string, category PluginCategory, name string) string {
	switch category {
	case PluginCategorySource:
		return fmt.Sprintf("%s/%s/connector-v2/source/%s", SeaTunnelDocsBaseURL, version, name)
	case PluginCategorySink:
		return fmt.Sprintf("%s/%s/connector-v2/sink/%s", SeaTunnelDocsBaseURL, version, name)
	case PluginCategoryTransform:
		return fmt.Sprintf("%s/%s/transform-v2/%s", SeaTunnelDocsBaseURL, version, name)
	}
	return ""
}

// generatePluginDescription generates a description for a plugin.
// generatePluginDescription 为插件生成描述。
func generatePluginDescription(displayName string, category PluginCategory) string {
	switch category {
	case PluginCategorySource:
		return fmt.Sprintf("Read data from %s / 从 %s 读取数据", displayName, displayName)
	case PluginCategorySink:
		return fmt.Sprintf("Write data to %s / 将数据写入 %s", displayName, displayName)
	case PluginCategoryTransform:
		return fmt.Sprintf("Transform data using %s / 使用 %s 转换数据", displayName, displayName)
	}
	return ""
}

// RefreshPlugins forces a refresh of the plugin list from SeaTunnel docs.
// RefreshPlugins 强制从 SeaTunnel 文档刷新插件列表。
func (s *Service) RefreshPlugins(ctx context.Context, version string) ([]Plugin, error) {
	plugins, err := s.fetchPluginsFromDocs(ctx, version)
	if err != nil {
		return getAvailablePluginsForVersion(version), err
	}

	// Update cache / 更新缓存
	s.pluginsMu.Lock()
	s.cachedPlugins[version] = plugins
	s.pluginsCacheTime[version] = time.Now()
	s.pluginsMu.Unlock()

	return plugins, nil
}

// GetPluginInfo returns detailed information about a specific plugin.
// GetPluginInfo 返回特定插件的详细信息。
func (s *Service) GetPluginInfo(ctx context.Context, name string, version string) (*Plugin, error) {
	if version == "" {
		version = "2.3.12"
	}

	// Normalize name to lowercase for comparison / 将名称转换为小写进行比较
	normalizedName := strings.ToLower(name)

	// First, try to find in cached plugins from docs / 首先尝试从文档缓存中查找
	s.pluginsMu.RLock()
	if cachedPlugins, ok := s.cachedPlugins[version]; ok {
		for _, p := range cachedPlugins {
			if strings.ToLower(p.Name) == normalizedName {
				s.pluginsMu.RUnlock()
				return &p, nil
			}
		}
	}
	s.pluginsMu.RUnlock()

	// Then, try to find in fallback plugins / 然后尝试从备用插件列表中查找
	plugins := getAvailablePluginsForVersion(version)
	for _, p := range plugins {
		if strings.ToLower(p.Name) == normalizedName {
			return &p, nil
		}
	}

	// If not found in cache or fallback, try to fetch from docs / 如果缓存和备用列表都没有，尝试从文档获取
	fetchedPlugins := s.getPlugins(ctx, version)
	for _, p := range fetchedPlugins {
		if strings.ToLower(p.Name) == normalizedName {
			return &p, nil
		}
	}

	return nil, ErrPluginNotAvailable
}

// ==================== Installed Plugins 已安装插件 ====================

// ListInstalledPlugins returns installed plugins for a cluster.
// ListInstalledPlugins 返回集群上已安装的插件列表。
func (s *Service) ListInstalledPlugins(ctx context.Context, clusterID uint) ([]InstalledPlugin, error) {
	return s.repo.ListByCluster(ctx, clusterID)
}

// GetInstalledPlugin returns an installed plugin by cluster and name.
// GetInstalledPlugin 通过集群和名称获取已安装插件。
func (s *Service) GetInstalledPlugin(ctx context.Context, clusterID uint, pluginName string) (*InstalledPlugin, error) {
	return s.repo.GetByClusterAndName(ctx, clusterID, pluginName)
}

// ==================== Plugin Installation 插件安装 ====================

// InstallPlugin installs a plugin on a cluster via Agent.
// InstallPlugin 通过 Agent 在集群上安装插件。
// Requirements: Validates that plugin version matches cluster version.
// 需求：校验插件版本与集群版本是否匹配。
func (s *Service) InstallPlugin(ctx context.Context, clusterID uint, req *InstallPluginRequest) (*InstalledPlugin, error) {
	// Delegate to InstallPluginToCluster which handles the full installation flow
	// 委托给 InstallPluginToCluster 处理完整的安装流程
	return s.InstallPluginToCluster(ctx, clusterID, req)
}

// UninstallPlugin uninstalls a plugin from a cluster.
// UninstallPlugin 从集群上卸载插件。
func (s *Service) UninstallPlugin(ctx context.Context, clusterID uint, pluginName string) error {
	// Check if plugin exists / 检查插件是否存在
	_, err := s.repo.GetByClusterAndName(ctx, clusterID, pluginName)
	if err != nil {
		return err
	}

	// TODO: Send uninstall command to Agent via gRPC
	// TODO: 通过 gRPC 向 Agent 发送卸载命令

	// Delete installed plugin record / 删除已安装插件记录
	return s.repo.DeleteByClusterAndName(ctx, clusterID, pluginName)
}

// EnablePlugin enables an installed plugin.
// EnablePlugin 启用已安装的插件。
func (s *Service) EnablePlugin(ctx context.Context, clusterID uint, pluginName string) (*InstalledPlugin, error) {
	plugin, err := s.repo.GetByClusterAndName(ctx, clusterID, pluginName)
	if err != nil {
		return nil, err
	}

	plugin.Status = PluginStatusEnabled
	plugin.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, plugin); err != nil {
		return nil, err
	}

	return plugin, nil
}

// DisablePlugin disables an installed plugin.
// DisablePlugin 禁用已安装的插件。
func (s *Service) DisablePlugin(ctx context.Context, clusterID uint, pluginName string) (*InstalledPlugin, error) {
	plugin, err := s.repo.GetByClusterAndName(ctx, clusterID, pluginName)
	if err != nil {
		return nil, err
	}

	plugin.Status = PluginStatusDisabled
	plugin.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, plugin); err != nil {
		return nil, err
	}

	return plugin, nil
}

// ==================== Helper Functions 辅助函数 ====================

// getAvailablePluginsForVersion returns predefined plugins for a SeaTunnel version.
// getAvailablePluginsForVersion 返回指定 SeaTunnel 版本的预定义插件列表。
// In production, this would fetch from Maven repository metadata.
// 在生产环境中，这将从 Maven 仓库元数据获取。
func getAvailablePluginsForVersion(version string) []Plugin {
	// Common plugins available for all supported versions
	// 所有支持版本的通用插件
	return []Plugin{
		// Source connectors / Source连接器
		{
			Name:        "jdbc",
			DisplayName: "JDBC",
			Category:    PluginCategorySource,
			Version:     version,
			Description: "Read data from JDBC databases (MySQL, PostgreSQL, Oracle, etc.) / 从 JDBC 数据库读取数据",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-jdbc",
			Dependencies: []PluginDependency{
				{GroupID: "mysql", ArtifactID: "mysql-connector-java", Version: "8.0.28", TargetDir: "lib"},
				{GroupID: "org.postgresql", ArtifactID: "postgresql", Version: "42.3.3", TargetDir: "lib"},
			},
			DocURL: "https://seatunnel.apache.org/docs/connector-v2/source/Jdbc",
		},
		{
			Name:        "kafka",
			DisplayName: "Kafka",
			Category:    PluginCategorySource,
			Version:     version,
			Description: "Read data from Apache Kafka / 从 Apache Kafka 读取数据",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-kafka",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/source/Kafka",
		},
		{
			Name:        "mysql-cdc",
			DisplayName: "MySQL CDC",
			Category:    PluginCategorySource,
			Version:     version,
			Description: "Capture MySQL change data in real-time / 实时捕获 MySQL 变更数据",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-cdc-mysql",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/source/MySQL-CDC",
		},
		{
			Name:        "postgres-cdc",
			DisplayName: "PostgreSQL CDC",
			Category:    PluginCategorySource,
			Version:     version,
			Description: "Capture PostgreSQL change data in real-time / 实时捕获 PostgreSQL 变更数据",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-cdc-postgres",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/source/Postgres-CDC",
		},
		{
			Name:        "http",
			DisplayName: "HTTP",
			Category:    PluginCategorySource,
			Version:     version,
			Description: "Read data from HTTP APIs / 从 HTTP API 读取数据",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-http-base",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/source/Http",
		},
		{
			Name:        "file-local",
			DisplayName: "Local File",
			Category:    PluginCategorySource,
			Version:     version,
			Description: "Read data from local files / 从本地文件读取数据",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-file-local",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/source/LocalFile",
		},
		{
			Name:        "file-hdfs",
			DisplayName: "HDFS",
			Category:    PluginCategorySource,
			Version:     version,
			Description: "Read data from HDFS / 从 HDFS 读取数据",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-file-hadoop",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/source/HdfsFile",
		},
		{
			Name:        "file-s3",
			DisplayName: "Amazon S3",
			Category:    PluginCategorySource,
			Version:     version,
			Description: "Read data from Amazon S3 / 从 Amazon S3 读取数据",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-file-s3",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/source/S3File",
		},
		{
			Name:        "elasticsearch",
			DisplayName: "Elasticsearch",
			Category:    PluginCategorySource,
			Version:     version,
			Description: "Read data from Elasticsearch / 从 Elasticsearch 读取数据",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-elasticsearch",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/source/Elasticsearch",
		},
		{
			Name:        "mongodb",
			DisplayName: "MongoDB",
			Category:    PluginCategorySource,
			Version:     version,
			Description: "Read data from MongoDB / 从 MongoDB 读取数据",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-mongodb",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/source/MongoDB",
		},

		// Sink connectors / Sink连接器
		{
			Name:        "jdbc-sink",
			DisplayName: "JDBC Sink",
			Category:    PluginCategorySink,
			Version:     version,
			Description: "Write data to JDBC databases / 将数据写入 JDBC 数据库",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-jdbc",
			Dependencies: []PluginDependency{
				{GroupID: "mysql", ArtifactID: "mysql-connector-java", Version: "8.0.28", TargetDir: "lib"},
				{GroupID: "org.postgresql", ArtifactID: "postgresql", Version: "42.3.3", TargetDir: "lib"},
			},
			DocURL: "https://seatunnel.apache.org/docs/connector-v2/sink/Jdbc",
		},
		{
			Name:        "kafka-sink",
			DisplayName: "Kafka Sink",
			Category:    PluginCategorySink,
			Version:     version,
			Description: "Write data to Apache Kafka / 将数据写入 Apache Kafka",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-kafka",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/sink/Kafka",
		},
		{
			Name:        "clickhouse",
			DisplayName: "ClickHouse",
			Category:    PluginCategorySink,
			Version:     version,
			Description: "Write data to ClickHouse / 将数据写入 ClickHouse",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-clickhouse",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/sink/Clickhouse",
		},
		{
			Name:        "doris",
			DisplayName: "Apache Doris",
			Category:    PluginCategorySink,
			Version:     version,
			Description: "Write data to Apache Doris / 将数据写入 Apache Doris",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-doris",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/sink/Doris",
		},
		{
			Name:        "starrocks",
			DisplayName: "StarRocks",
			Category:    PluginCategorySink,
			Version:     version,
			Description: "Write data to StarRocks / 将数据写入 StarRocks",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-starrocks",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/sink/StarRocks",
		},
		{
			Name:        "elasticsearch-sink",
			DisplayName: "Elasticsearch Sink",
			Category:    PluginCategorySink,
			Version:     version,
			Description: "Write data to Elasticsearch / 将数据写入 Elasticsearch",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-elasticsearch",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/sink/Elasticsearch",
		},
		{
			Name:        "hive",
			DisplayName: "Apache Hive",
			Category:    PluginCategorySink,
			Version:     version,
			Description: "Write data to Apache Hive / 将数据写入 Apache Hive",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-hive",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/sink/Hive",
		},
		{
			Name:        "console",
			DisplayName: "Console",
			Category:    PluginCategorySink,
			Version:     version,
			Description: "Print data to console for debugging / 将数据打印到控制台用于调试",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "connector-console",
			DocURL:      "https://seatunnel.apache.org/docs/connector-v2/sink/Console",
		},

		// Transform connectors / 数据转换连接器
		{
			Name:        "filter",
			DisplayName: "Filter",
			Category:    PluginCategoryTransform,
			Version:     version,
			Description: "Filter rows based on conditions / 根据条件过滤行",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "transform-filter",
			DocURL:      "https://seatunnel.apache.org/docs/transform-v2/filter",
		},
		{
			Name:        "sql",
			DisplayName: "SQL",
			Category:    PluginCategoryTransform,
			Version:     version,
			Description: "Transform data using SQL / 使用 SQL 转换数据",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "transform-sql",
			DocURL:      "https://seatunnel.apache.org/docs/transform-v2/sql",
		},
		{
			Name:        "field-mapper",
			DisplayName: "Field Mapper",
			Category:    PluginCategoryTransform,
			Version:     version,
			Description: "Map and rename fields / 映射和重命名字段",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "transform-field-mapper",
			DocURL:      "https://seatunnel.apache.org/docs/transform-v2/field-mapper",
		},
		{
			Name:        "replace",
			DisplayName: "Replace",
			Category:    PluginCategoryTransform,
			Version:     version,
			Description: "Replace field values / 替换字段值",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "transform-replace",
			DocURL:      "https://seatunnel.apache.org/docs/transform-v2/replace",
		},
		{
			Name:        "split",
			DisplayName: "Split",
			Category:    PluginCategoryTransform,
			Version:     version,
			Description: "Split field values / 拆分字段值",
			GroupID:     "org.apache.seatunnel",
			ArtifactID:  "transform-split",
			DocURL:      "https://seatunnel.apache.org/docs/transform-v2/split",
		},
	}
}

// ==================== Plugin Download Methods 插件下载方法 ====================

// DownloadPlugin downloads a plugin to the Control Plane local storage.
// DownloadPlugin 下载插件到 Control Plane 本地存储。
func (s *Service) DownloadPlugin(ctx context.Context, name, version string, mirror MirrorSource) (*DownloadProgress, error) {
	if version == "" {
		version = "2.3.12" // Default version / 默认版本
	}

	if mirror == "" {
		mirror = MirrorSourceApache
	}

	// Get plugin info / 获取插件信息
	plugin, err := s.GetPluginInfo(ctx, name, version)
	if err != nil {
		return nil, err
	}

	// Ensure artifact_id is set / 确保 artifact_id 已设置
	if plugin.ArtifactID == "" {
		plugin.ArtifactID = getArtifactID(name)
		fmt.Printf("[DownloadPlugin] Warning: plugin.ArtifactID was empty for %s, set to: %s\n", name, plugin.ArtifactID)
	}
	fmt.Printf("[DownloadPlugin] Plugin: name=%s, artifactID=%s, version=%s\n", plugin.Name, plugin.ArtifactID, plugin.Version)

	// Check if already downloaded / 检查是否已下载
	if s.downloader.IsConnectorDownloaded(name, version) {
		return &DownloadProgress{
			PluginName:  name,
			Version:     version,
			Status:      "completed",
			Progress:    100,
			CurrentStep: "Already downloaded / 已下载",
		}, nil
	}

	// Start download in background / 在后台开始下载
	go func() {
		downloadCtx := context.Background()
		if err := s.downloader.DownloadPlugin(downloadCtx, plugin, mirror, nil); err != nil {
			// Log error for debugging / 记录错误用于调试
			fmt.Printf("[Plugin Download Error] plugin=%s, version=%s, error=%v\n", name, version, err)
		}
	}()

	// Return initial progress / 返回初始进度
	return &DownloadProgress{
		PluginName:  name,
		Version:     version,
		Status:      "downloading",
		Progress:    0,
		CurrentStep: "Starting download / 开始下载",
		StartTime:   time.Now(),
	}, nil
}

// GetDownloadStatus returns the current download status for a plugin.
// GetDownloadStatus 返回插件的当前下载状态。
func (s *Service) GetDownloadStatus(name, version string) *DownloadProgress {
	// Check if download is in progress / 检查是否正在下载
	progress := s.downloader.GetDownloadProgress(name, version)
	if progress != nil {
		return progress
	}

	// Check if already downloaded / 检查是否已下载
	if s.downloader.IsConnectorDownloaded(name, version) {
		return &DownloadProgress{
			PluginName:  name,
			Version:     version,
			Status:      "completed",
			Progress:    100,
			CurrentStep: "Downloaded / 已下载",
		}
	}

	// Not downloaded / 未下载
	return &DownloadProgress{
		PluginName:  name,
		Version:     version,
		Status:      "not_started",
		Progress:    0,
		CurrentStep: "Not downloaded / 未下载",
	}
}

// ListLocalPlugins returns a list of locally downloaded plugins.
// ListLocalPlugins 返回本地已下载的插件列表。
func (s *Service) ListLocalPlugins() ([]LocalPlugin, error) {
	return s.downloader.ListLocalPlugins()
}

// DeleteLocalPlugin deletes a locally downloaded plugin.
// DeleteLocalPlugin 删除本地已下载的插件。
func (s *Service) DeleteLocalPlugin(name, version string) error {
	return s.downloader.DeleteLocalPlugin(name, version)
}

// IsPluginDownloaded checks if a plugin is downloaded locally.
// IsPluginDownloaded 检查插件是否已在本地下载。
func (s *Service) IsPluginDownloaded(name, version string) bool {
	return s.downloader.IsConnectorDownloaded(name, version)
}

// ListActiveDownloads returns all active download tasks.
// ListActiveDownloads 返回所有活动的下载任务。
func (s *Service) ListActiveDownloads() []*DownloadProgress {
	return s.downloader.ListActiveDownloads()
}

// ==================== Plugin Installation Progress Methods 插件安装进度方法 ====================

// GetInstallProgress returns the installation progress for a plugin on a cluster.
// GetInstallProgress 返回集群上插件的安装进度。
func (s *Service) GetInstallProgress(clusterID uint, pluginName string) *PluginInstallStatus {
	key := fmt.Sprintf("%d:%s", clusterID, pluginName)

	s.installProgressMu.RLock()
	defer s.installProgressMu.RUnlock()

	if progress, exists := s.installProgress[key]; exists {
		return progress
	}

	return nil
}

// setInstallProgress sets the installation progress for a plugin on a cluster.
// setInstallProgress 设置集群上插件的安装进度。
func (s *Service) setInstallProgress(clusterID uint, pluginName string, status *PluginInstallStatus) {
	key := fmt.Sprintf("%d:%s", clusterID, pluginName)

	s.installProgressMu.Lock()
	defer s.installProgressMu.Unlock()

	s.installProgress[key] = status
}

// clearInstallProgress clears the installation progress for a plugin on a cluster.
// clearInstallProgress 清除集群上插件的安装进度。
func (s *Service) clearInstallProgress(clusterID uint, pluginName string) {
	key := fmt.Sprintf("%d:%s", clusterID, pluginName)

	s.installProgressMu.Lock()
	defer s.installProgressMu.Unlock()

	delete(s.installProgress, key)
}

// ==================== Cluster Plugin Installation Methods 集群插件安装方法 ====================

// AgentCommandSender is an interface for sending commands to agents.
// AgentCommandSender 是向 Agent 发送命令的接口。
type AgentCommandSender interface {
	SendCommand(ctx context.Context, agentID string, commandType string, params map[string]string) (bool, string, error)
}

// SetAgentCommandSender sets the agent command sender for plugin installation.
// SetAgentCommandSender 设置用于插件安装的 Agent 命令发送器。
func (s *Service) SetAgentCommandSender(sender AgentCommandSender) {
	s.agentCommandSender = sender
}

// SetClusterNodeGetter sets the cluster node getter for plugin installation.
// SetClusterNodeGetter 设置用于插件安装的集群节点获取器。
func (s *Service) SetClusterNodeGetter(getter ClusterNodeGetter) {
	s.clusterNodeGetter = getter
}

// SetHostInfoGetter sets the host info getter for plugin installation.
// SetHostInfoGetter 设置用于插件安装的主机信息获取器。
func (s *Service) SetHostInfoGetter(getter HostInfoGetter) {
	s.hostInfoGetter = getter
}

// InstallPluginToCluster installs a plugin to all nodes in a cluster.
// InstallPluginToCluster 将插件安装到集群中的所有节点。
// This method:
// 1. Checks if plugin is downloaded locally (downloads if not)
// 2. Gets all cluster nodes
// 3. Transfers plugin files to each node's Agent
// 4. Sends install command to each Agent
// 5. Updates database record
func (s *Service) InstallPluginToCluster(ctx context.Context, clusterID uint, req *InstallPluginRequest) (*InstalledPlugin, error) {
	// Validate plugin version matches cluster version / 校验插件版本与集群版本是否匹配
	if s.clusterGetter != nil {
		clusterVersion, err := s.clusterGetter.GetClusterVersion(ctx, clusterID)
		if err != nil {
			return nil, err
		}
		if clusterVersion == "" {
			return nil, ErrClusterVersionEmpty
		}
		if req.Version != clusterVersion {
			return nil, fmt.Errorf("%w: plugin version %s, cluster version %s", ErrVersionMismatch, req.Version, clusterVersion)
		}
	}

	// Check if plugin already installed / 检查插件是否已安装
	exists, err := s.repo.ExistsByClusterAndName(ctx, clusterID, req.PluginName)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrPluginAlreadyExists
	}

	// Get plugin info / 获取插件信息
	pluginInfo, err := s.GetPluginInfo(ctx, req.PluginName, req.Version)
	if err != nil {
		return nil, err
	}

	// Initialize progress / 初始化进度
	progress := &PluginInstallStatus{
		PluginName: req.PluginName,
		Status:     "downloading",
		Progress:   0,
		Message:    "Checking local plugin files / 检查本地插件文件",
	}
	s.setInstallProgress(clusterID, req.PluginName, progress)

	// Check if plugin is downloaded locally / 检查插件是否已在本地下载
	if !s.downloader.IsConnectorDownloaded(req.PluginName, req.Version) {
		progress.Message = "Downloading plugin / 下载插件"
		s.setInstallProgress(clusterID, req.PluginName, progress)

		// Download plugin / 下载插件
		mirror := req.Mirror
		if mirror == "" {
			mirror = MirrorSourceApache
		}

		if err := s.downloader.DownloadPlugin(ctx, pluginInfo, mirror, func(p *DownloadProgress) {
			progress.Progress = p.Progress / 2 // First half is download / 前半部分是下载
			progress.Message = p.CurrentStep
			s.setInstallProgress(clusterID, req.PluginName, progress)
		}); err != nil {
			progress.Status = "failed"
			progress.Error = err.Error()
			s.setInstallProgress(clusterID, req.PluginName, progress)
			return nil, fmt.Errorf("failed to download plugin: %w", err)
		}
	}

	// Update progress / 更新进度
	progress.Progress = 50
	progress.Status = "installing"
	progress.Message = "Plugin downloaded, preparing installation / 插件已下载，准备安装"
	s.setInstallProgress(clusterID, req.PluginName, progress)

	// Get cluster nodes / 获取集群节点
	// Log dependency status for debugging / 记录依赖状态用于调试
	fmt.Printf("[Plugin Install] Dependencies: clusterNodeGetter=%v, agentCommandSender=%v, hostInfoGetter=%v\n",
		s.clusterNodeGetter != nil, s.agentCommandSender != nil, s.hostInfoGetter != nil)
	fmt.Printf("[Plugin Install] Installing plugin %s v%s to cluster %d\n", req.PluginName, req.Version, clusterID)

	// Get artifact ID from plugin info, use mapping as fallback
	// 从插件信息获取 artifact ID，使用映射作为备用
	artifactID := pluginInfo.ArtifactID
	if artifactID == "" {
		artifactID = getArtifactID(req.PluginName)
	}
	fmt.Printf("[Plugin Install] Plugin %s -> ArtifactID: %s\n", req.PluginName, artifactID)

	if s.clusterNodeGetter != nil && s.agentCommandSender != nil && s.hostInfoGetter != nil {
		nodes, err := s.clusterNodeGetter.GetClusterNodes(ctx, clusterID)
		if err != nil {
			progress.Status = "failed"
			progress.Error = fmt.Sprintf("Failed to get cluster nodes: %v / 获取集群节点失败: %v", err, err)
			s.setInstallProgress(clusterID, req.PluginName, progress)
			return nil, fmt.Errorf("failed to get cluster nodes: %w", err)
		}

		fmt.Printf("[Plugin Install] Found %d nodes in cluster %d\n", len(nodes), clusterID)

		if len(nodes) == 0 {
			progress.Status = "failed"
			progress.Error = "No nodes found in cluster / 集群中没有节点"
			s.setInstallProgress(clusterID, req.PluginName, progress)
			return nil, fmt.Errorf("no nodes found in cluster")
		}

		// Transfer and install plugin to each node / 将插件传输并安装到每个节点
		totalNodes := len(nodes)
		for i, node := range nodes {
			// Update progress / 更新进度
			nodeProgress := 50 + (i * 50 / totalNodes)
			progress.Progress = nodeProgress
			progress.Message = fmt.Sprintf("Installing to node %d/%d / 正在安装到节点 %d/%d", i+1, totalNodes, i+1, totalNodes)
			s.setInstallProgress(clusterID, req.PluginName, progress)

			// Get agent ID for this host / 获取此主机的 Agent ID
			agentID, err := s.hostInfoGetter.GetHostAgentID(ctx, node.HostID)
			if err != nil {
				progress.Status = "failed"
				progress.Error = fmt.Sprintf("Failed to get agent ID for host %d: %v / 获取主机 %d 的 Agent ID 失败: %v", node.HostID, err, node.HostID, err)
				s.setInstallProgress(clusterID, req.PluginName, progress)
				return nil, fmt.Errorf("failed to get agent ID for host %d: %w", node.HostID, err)
			}

			fmt.Printf("[Plugin Install] Node %d: HostID=%d, AgentID=%s, InstallDir=%s\n", node.NodeID, node.HostID, agentID, node.InstallDir)

			if agentID == "" {
				progress.Status = "failed"
				progress.Error = fmt.Sprintf("Agent not installed on host %d / 主机 %d 未安装 Agent", node.HostID, node.HostID)
				s.setInstallProgress(clusterID, req.PluginName, progress)
				return nil, fmt.Errorf("agent not installed on host %d", node.HostID)
			}

			// Transfer plugin file to agent using artifact ID / 使用 artifact ID 传输插件文件到 Agent
			fmt.Printf("[Plugin Install] Transferring plugin %s (artifact: %s) to agent %s...\n", req.PluginName, artifactID, agentID)
			if err := s.transferPluginToAgent(ctx, agentID, artifactID, req.PluginName, req.Version, node.InstallDir); err != nil {
				progress.Status = "failed"
				progress.Error = fmt.Sprintf("Failed to transfer plugin to node %d: %v / 传输插件到节点 %d 失败: %v", node.NodeID, err, node.NodeID, err)
				s.setInstallProgress(clusterID, req.PluginName, progress)
				return nil, fmt.Errorf("failed to transfer plugin to node %d: %w", node.NodeID, err)
			}
		}
	}

	// Create database record / 创建数据库记录
	installed := &InstalledPlugin{
		ClusterID:   clusterID,
		PluginName:  req.PluginName,
		ArtifactID:  artifactID,
		Category:    pluginInfo.Category,
		Version:     req.Version,
		Status:      PluginStatusInstalled,
		InstallPath: fmt.Sprintf("connectors/%s-%s.jar", artifactID, req.Version),
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, installed); err != nil {
		progress.Status = "failed"
		progress.Error = err.Error()
		s.setInstallProgress(clusterID, req.PluginName, progress)
		return nil, err
	}

	// Mark as completed / 标记为完成
	progress.Status = "completed"
	progress.Progress = 100
	progress.Message = "Plugin installed successfully / 插件安装成功"
	s.setInstallProgress(clusterID, req.PluginName, progress)

	// Clear progress after a delay / 延迟后清除进度
	go func() {
		time.Sleep(30 * time.Second)
		s.clearInstallProgress(clusterID, req.PluginName)
	}()

	return installed, nil
}

// UninstallPluginFromCluster uninstalls a plugin from all nodes in a cluster.
// UninstallPluginFromCluster 从集群中的所有节点卸载插件。
func (s *Service) UninstallPluginFromCluster(ctx context.Context, clusterID uint, pluginName string) error {
	// Check if plugin exists / 检查插件是否存在
	plugin, err := s.repo.GetByClusterAndName(ctx, clusterID, pluginName)
	if err != nil {
		return err
	}

	// Initialize progress / 初始化进度
	progress := &PluginInstallStatus{
		PluginName: pluginName,
		Status:     "uninstalling",
		Progress:   0,
		Message:    "Uninstalling plugin / 正在卸载插件",
	}
	s.setInstallProgress(clusterID, pluginName, progress)

	// TODO: Get cluster nodes and send uninstall commands to each Agent
	// TODO: 获取集群节点并向每个 Agent 发送卸载命令

	// Delete database record / 删除数据库记录
	if err := s.repo.Delete(ctx, plugin.ID); err != nil {
		progress.Status = "failed"
		progress.Error = err.Error()
		s.setInstallProgress(clusterID, pluginName, progress)
		return err
	}

	// Mark as completed / 标记为完成
	progress.Status = "completed"
	progress.Progress = 100
	progress.Message = "Plugin uninstalled successfully / 插件卸载成功"
	s.setInstallProgress(clusterID, pluginName, progress)

	// Clear progress after a delay / 延迟后清除进度
	go func() {
		time.Sleep(30 * time.Second)
		s.clearInstallProgress(clusterID, pluginName)
	}()

	return nil
}

// ==================== Plugin Transfer Methods 插件传输方法 ====================

// transferPluginToAgent transfers a plugin file to an Agent and installs it.
// transferPluginToAgent 将插件文件传输到 Agent 并安装。
// This method:
// 1. Reads the plugin file from local storage
// 2. Sends file chunks to Agent via TRANSFER_PLUGIN command
// 3. Sends INSTALL_PLUGIN command to finalize installation
// Parameters:
// - artifactID: Maven artifact ID (e.g., connector-cdc-mysql, connector-file-cos)
// - pluginName: Plugin display name (e.g., mysql-cdc, cosfile)
func (s *Service) transferPluginToAgent(ctx context.Context, agentID, artifactID, pluginName, version, installDir string) error {
	if s.agentCommandSender == nil {
		return fmt.Errorf("agent command sender not configured / Agent 命令发送器未配置")
	}

	// Use artifact ID directly for file name / 直接使用 artifact ID 作为文件名
	fileName := fmt.Sprintf("%s-%s.jar", artifactID, version)

	// Read plugin file using artifact ID / 使用 artifact ID 读取插件文件
	fileData, err := s.downloader.ReadPluginFileByArtifactID(artifactID, version)
	if err != nil {
		return fmt.Errorf("failed to read plugin file: %w / 读取插件文件失败: %w", err, err)
	}

	// Transfer file in chunks / 分块传输文件
	// Chunk size: 1MB / 块大小: 1MB
	const chunkSize = 1024 * 1024
	totalSize := int64(len(fileData))
	var offset int64 = 0

	for offset < totalSize {
		end := offset + chunkSize
		if end > totalSize {
			end = totalSize
		}

		chunk := fileData[offset:end]
		isLast := end >= totalSize

		// Encode chunk as base64 / 将块编码为 base64
		chunkBase64 := encodeBase64(chunk)

		// Send transfer command / 发送传输命令
		params := map[string]string{
			"plugin_name":  pluginName,
			"version":      version,
			"file_type":    "connector",
			"file_name":    fileName,
			"chunk":        chunkBase64,
			"offset":       fmt.Sprintf("%d", offset),
			"total_size":   fmt.Sprintf("%d", totalSize),
			"is_last":      fmt.Sprintf("%t", isLast),
			"install_path": installDir,
		}

		success, message, err := s.agentCommandSender.SendCommand(ctx, agentID, "transfer_plugin", params)
		if err != nil {
			return fmt.Errorf("failed to transfer chunk at offset %d: %w / 传输偏移 %d 处的块失败: %w", offset, err, offset, err)
		}
		if !success {
			return fmt.Errorf("transfer chunk failed: %s / 传输块失败: %s", message, message)
		}

		offset = end
	}

	// Send install command / 发送安装命令
	// Pass artifact_id so Agent can find the file directly
	// 传递 artifact_id 以便 Agent 可以直接找到文件
	installParams := map[string]string{
		"plugin_name":  pluginName,
		"artifact_id":  artifactID,
		"version":      version,
		"install_path": installDir,
	}

	success, message, err := s.agentCommandSender.SendCommand(ctx, agentID, "install_plugin", installParams)
	if err != nil {
		return fmt.Errorf("failed to send install command: %w / 发送安装命令失败: %w", err, err)
	}
	if !success {
		return fmt.Errorf("plugin installation failed: %s / 插件安装失败: %s", message, message)
	}

	return nil
}

// encodeBase64 encodes data to base64 string.
// encodeBase64 将数据编码为 base64 字符串。
func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
