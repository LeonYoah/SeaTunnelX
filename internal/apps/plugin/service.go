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

// Service provides plugin management functionality.
// Service 提供插件管理功能。
type Service struct {
	repo          *Repository
	clusterGetter ClusterGetter
	// agentManager is used to communicate with agents for plugin installation
	// agentManager 用于与 Agent 通信进行插件安装
	// agentManager *agent.Manager // TODO: inject agent manager

	// Plugin cache / 插件缓存
	cachedPlugins    map[string][]Plugin // key: version
	pluginsCacheTime map[string]time.Time
	pluginsMu        sync.RWMutex
}

// NewService creates a new Service instance.
// NewService 创建一个新的 Service 实例。
func NewService(repo *Repository) *Service {
	return &Service{
		repo:             repo,
		cachedPlugins:    make(map[string][]Plugin),
		pluginsCacheTime: make(map[string]time.Time),
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

	// Fetch source connectors / 获取数据源连接器
	sourcePlugins, err := s.fetchPluginsByCategory(ctx, version, PluginCategorySource)
	if err == nil {
		allPlugins = append(allPlugins, sourcePlugins...)
	}

	// Fetch sink connectors / 获取数据目标连接器
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
				ArtifactID:  fmt.Sprintf("connector-%s", name),
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

	plugins := getAvailablePluginsForVersion(version)
	for _, p := range plugins {
		if p.Name == name {
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
	// Validate plugin version matches cluster version / 校验插件版本与集群版本是否匹配
	if s.clusterGetter != nil {
		clusterVersion, err := s.clusterGetter.GetClusterVersion(ctx, clusterID)
		if err != nil {
			return nil, err
		}
		if clusterVersion == "" {
			return nil, ErrClusterVersionEmpty
		}
		// Compare versions - plugin version must match cluster version
		// 比较版本 - 插件版本必须与集群版本匹配
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

	// TODO: Send install command to Agent via gRPC
	// TODO: 通过 gRPC 向 Agent 发送安装命令
	// The Agent will:
	// 1. Download connector jar to connectors/ directory
	// 2. Download dependencies to lib/ directory

	// Create installed plugin record / 创建已安装插件记录
	installed := &InstalledPlugin{
		ClusterID:   clusterID,
		PluginName:  req.PluginName,
		Category:    pluginInfo.Category,
		Version:     req.Version,
		Status:      PluginStatusInstalled,
		InstallPath: fmt.Sprintf("connectors/connector-%s-%s.jar", req.PluginName, req.Version),
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, installed); err != nil {
		return nil, err
	}

	return installed, nil
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
		// Source connectors / 数据源连接器
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

		// Sink connectors / 数据目标连接器
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
