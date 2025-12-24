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

// SeaTunnel Maven repository and documentation URLs
// SeaTunnel Maven 仓库和文档 URL
const (
	// Maven repository URL for fetching connector list / Maven 仓库 URL，用于获取连接器列表
	MavenRepoBaseURL     = "https://repo.maven.apache.org/maven2/org/apache/seatunnel"
	// SeaTunnel documentation URL for connector docs / SeaTunnel 文档 URL，用于连接器文档
	SeaTunnelDocsBaseURL = "https://seatunnel.apache.org/zh-CN/docs"
	// Cache duration / 缓存时间
	PluginCacheDuration  = 24 * time.Hour
	// HTTP request timeout for fetching plugin list / 获取插件列表的 HTTP 请求超时
	PluginFetchTimeout   = 60 * time.Second
)

// skipModuleList contains modules to skip when fetching connectors from Maven.
// skipModuleList 包含从 Maven 获取连接器时需要跳过的模块。
var skipModuleList = []string{
	"connector-common",   // Common utilities / 通用工具
	"connector-cdc-base", // CDC base module / CDC 基础模块
	"connector-cdc",      // CDC parent module / CDC 父模块
	"connector-file",     // File parent module / File 父模块
	"connector-http",     // HTTP parent module / HTTP 父模块
}

// isSkippedModule checks if the artifact ID should be skipped.
// isSkippedModule 检查 artifact ID 是否应该被跳过。
func isSkippedModule(artifactID string) bool {
	for _, skip := range skipModuleList {
		if artifactID == skip {
			return true
		}
	}
	return false
}

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

// getPlugins returns the plugin list, using cache if valid, otherwise fetching from Maven.
// getPlugins 返回插件列表，如果缓存有效则使用缓存，否则从 Maven 获取。
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

	// Fetch from Maven repository / 从 Maven 仓库获取
	plugins, err := s.fetchPluginsFromDocs(ctx, version)
	if err != nil {
		// Return empty list on error / 出错时返回空列表
		fmt.Printf("[Plugin] Failed to fetch plugins from Maven: %v\n", err)
		return []Plugin{}
	}

	// Update cache / 更新缓存
	s.pluginsMu.Lock()
	s.cachedPlugins[version] = plugins
	s.pluginsCacheTime[version] = time.Now()
	s.pluginsMu.Unlock()

	return plugins
}

// fetchPluginsFromDocs fetches plugin list from Maven repository.
// fetchPluginsFromDocs 从 Maven 仓库获取插件列表。
// Strategy: Fetch connector list from Maven repo and filter by version
// 策略：从 Maven 仓库获取连接器列表并按版本过滤
func (s *Service) fetchPluginsFromDocs(ctx context.Context, version string) ([]Plugin, error) {
	// Fetch connectors from Maven repo / 从 Maven 仓库获取连接器
	connectors, err := s.fetchConnectorsFromMaven(ctx, version)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch connectors from Maven: %w / 从 Maven 获取连接器失败: %w", err, err)
	}

	if len(connectors) == 0 {
		return nil, fmt.Errorf("no connectors found for version %s / 未找到版本 %s 的连接器", version, version)
	}

	return connectors, nil
}

// fetchConnectorsFromMaven fetches connector list from Maven repository.
// fetchConnectorsFromMaven 从 Maven 仓库获取连接器列表。
func (s *Service) fetchConnectorsFromMaven(ctx context.Context, version string) ([]Plugin, error) {
	// Fetch the main directory listing / 获取主目录列表
	client := &http.Client{Timeout: PluginFetchTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, MavenRepoBaseURL+"/", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Maven repo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse HTML to extract connector names / 解析 HTML 提取连接器名称
	// Pattern: <a href="connector-xxx/">connector-xxx/</a>
	pattern := `<a[^>]*href="(connector-[^/"]+)/"[^>]*>`
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(string(body), -1)

	var plugins []Plugin
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) >= 2 {
			artifactID := match[1]

			// Skip e2e test modules / 跳过 e2e 测试模块
			if strings.Contains(artifactID, "-e2e") {
				continue
			}
			// Skip common/base modules / 跳过通用/基础模块
			if isSkippedModule(artifactID) {
				continue
			}
			// Skip if already seen / 跳过已处理的
			if seen[artifactID] {
				continue
			}
			seen[artifactID] = true

			// Check if this connector has the specified version / 检查此连接器是否有指定版本
			hasVersion, err := s.checkConnectorVersion(ctx, artifactID, version)
			if err != nil || !hasVersion {
				continue
			}

			// Create plugin entry / 创建插件条目
			plugin := s.createPluginFromArtifactID(artifactID, version)
			plugins = append(plugins, plugin)
		}
	}

	return plugins, nil
}

// checkConnectorVersion checks if a connector has the specified version in Maven.
// checkConnectorVersion 检查连接器在 Maven 中是否有指定版本。
func (s *Service) checkConnectorVersion(ctx context.Context, artifactID, version string) (bool, error) {
	url := fmt.Sprintf("%s/%s/", MavenRepoBaseURL, artifactID)
	
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	// Check if version directory exists / 检查版本目录是否存在
	// Pattern: <a href="2.3.12/">2.3.12/</a>
	pattern := fmt.Sprintf(`<a[^>]*href="%s/"`, regexp.QuoteMeta(version))
	matched, _ := regexp.MatchString(pattern, string(body))
	return matched, nil
}

// createPluginFromArtifactID creates a Plugin from Maven artifact ID.
// createPluginFromArtifactID 从 Maven artifact ID 创建 Plugin。
// Plugin name = artifact ID without "connector-" prefix (e.g., connector-jdbc -> jdbc)
// Display name = Title case with dashes replaced by spaces
// 插件名称 = artifact ID 去掉 "connector-" 前缀
// 显示名称 = 首字母大写，横杠替换为空格
func (s *Service) createPluginFromArtifactID(artifactID, version string) Plugin {
	// Extract plugin name from artifact ID / 从 artifact ID 提取插件名称
	// connector-jdbc -> jdbc, connector-cdc-mysql -> cdc-mysql
	name := strings.TrimPrefix(artifactID, "connector-")
	
	// Generate display name: title case with dashes replaced by spaces
	// 生成显示名称：首字母大写，横杠替换为空格
	displayName := strings.Title(strings.ReplaceAll(name, "-", " "))

	return Plugin{
		Name:        name,
		DisplayName: displayName,
		Category:    PluginCategoryConnector,
		Version:     version,
		Description: fmt.Sprintf("SeaTunnel %s connector / SeaTunnel %s 连接器", displayName, displayName),
		GroupID:     "org.apache.seatunnel",
		ArtifactID:  artifactID,
		DocURL:      fmt.Sprintf("%s/%s/connector-v2", SeaTunnelDocsBaseURL, version),
	}
}



// getArtifactID returns the Maven artifact ID for a plugin name.
// getArtifactID 返回插件名称对应的 Maven artifact ID。
// Since we fetch from Maven directly, artifact ID = "connector-" + name
// 由于我们直接从 Maven 获取，artifact ID = "connector-" + name
func getArtifactID(name string) string {
	return fmt.Sprintf("connector-%s", name)
}

// RefreshPlugins forces a refresh of the plugin list from Maven repository.
// RefreshPlugins 强制从 Maven 仓库刷新插件列表。
func (s *Service) RefreshPlugins(ctx context.Context, version string) ([]Plugin, error) {
	plugins, err := s.fetchPluginsFromDocs(ctx, version)
	if err != nil {
		return nil, err
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

	// First, try to find in cached plugins / 首先尝试从缓存中查找
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

	// If not found in cache, try to fetch from Maven / 如果缓存中没有，尝试从 Maven 获取
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

	// Load configured dependencies from database / 从数据库加载配置的依赖
	deps, err := s.GetPluginDependencies(ctx, name)
	if err == nil && len(deps) > 0 {
		plugin.Dependencies = deps
		fmt.Printf("[DownloadPlugin] Loaded %d dependencies for %s\n", len(deps), name)
	}

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

// DownloadAllPluginsProgress represents the progress of downloading all plugins.
// DownloadAllPluginsProgress 表示下载所有插件的进度。
type DownloadAllPluginsProgress struct {
	Total      int    `json:"total"`       // 总插件数 / Total plugins
	Downloaded int    `json:"downloaded"`  // 已下载数 / Downloaded count
	Failed     int    `json:"failed"`      // 失败数 / Failed count
	Skipped    int    `json:"skipped"`     // 跳过数（已存在）/ Skipped count (already exists)
	Status     string `json:"status"`      // 状态 / Status
	Message    string `json:"message"`     // 消息 / Message
}

// DownloadAllPlugins downloads all available plugins for a version.
// DownloadAllPlugins 下载指定版本的所有可用插件。
func (s *Service) DownloadAllPlugins(ctx context.Context, version string, mirror MirrorSource) (*DownloadAllPluginsProgress, error) {
	if version == "" {
		version = "2.3.12"
	}
	if mirror == "" {
		mirror = MirrorSourceApache
	}

	// Get all available plugins / 获取所有可用插件
	plugins := s.getPlugins(ctx, version)
	if len(plugins) == 0 {
		return nil, fmt.Errorf("no plugins found for version %s / 未找到版本 %s 的插件", version, version)
	}

	progress := &DownloadAllPluginsProgress{
		Total:   len(plugins),
		Status:  "downloading",
		Message: fmt.Sprintf("Starting download of %d plugins / 开始下载 %d 个插件", len(plugins), len(plugins)),
	}

	// Start downloading all plugins in background / 在后台开始下载所有插件
	go func() {
		downloadCtx := context.Background()
		for i := range plugins {
			plugin := &plugins[i]
			
			// Check if already downloaded / 检查是否已下载
			if s.downloader.IsConnectorDownloaded(plugin.Name, version) {
				progress.Skipped++
				continue
			}

			// Load configured dependencies from database / 从数据库加载配置的依赖
			deps, err := s.GetPluginDependencies(downloadCtx, plugin.Name)
			if err == nil && len(deps) > 0 {
				plugin.Dependencies = deps
			}

			// Download plugin / 下载插件
			err = s.downloader.DownloadPlugin(downloadCtx, plugin, mirror, nil)
			if err != nil {
				progress.Failed++
				fmt.Printf("[DownloadAllPlugins] Failed to download %s: %v\n", plugin.Name, err)
			} else {
				progress.Downloaded++
				fmt.Printf("[DownloadAllPlugins] Downloaded %s successfully\n", plugin.Name)
			}
		}
		progress.Status = "completed"
		progress.Message = fmt.Sprintf("Download completed: %d downloaded, %d skipped, %d failed / 下载完成: %d 已下载, %d 已跳过, %d 失败",
			progress.Downloaded, progress.Skipped, progress.Failed,
			progress.Downloaded, progress.Skipped, progress.Failed)
	}()

	return progress, nil
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

// ==================== Plugin Dependency Config Methods 插件依赖配置方法 ====================

// ListDependencies returns all configured dependencies for a plugin.
// ListDependencies 返回插件的所有配置依赖。
func (s *Service) ListDependencies(ctx context.Context, pluginName string) ([]PluginDependencyConfig, error) {
	return s.repo.ListDependencies(ctx, pluginName)
}

// AddDependency adds a dependency configuration for a plugin.
// AddDependency 为插件添加依赖配置。
func (s *Service) AddDependency(ctx context.Context, req *AddDependencyRequest) (*PluginDependencyConfig, error) {
	dep := &PluginDependencyConfig{
		PluginName: req.PluginName,
		GroupID:    req.GroupID,
		ArtifactID: req.ArtifactID,
		Version:    req.Version,
		TargetDir:  "lib", // Dependencies go to lib directory / 依赖放到 lib 目录
	}

	if err := s.repo.CreateDependency(ctx, dep); err != nil {
		return nil, err
	}

	return dep, nil
}

// DeleteDependency deletes a dependency configuration.
// DeleteDependency 删除依赖配置。
func (s *Service) DeleteDependency(ctx context.Context, depID uint) error {
	return s.repo.DeleteDependency(ctx, depID)
}

// GetPluginDependencies returns the configured dependencies for a plugin.
// GetPluginDependencies 返回插件的配置依赖（用于下载时）。
func (s *Service) GetPluginDependencies(ctx context.Context, pluginName string) ([]PluginDependency, error) {
	configs, err := s.repo.ListDependencies(ctx, pluginName)
	if err != nil {
		return nil, err
	}

	deps := make([]PluginDependency, len(configs))
	for i, cfg := range configs {
		deps[i] = PluginDependency{
			GroupID:    cfg.GroupID,
			ArtifactID: cfg.ArtifactID,
			Version:    cfg.Version,
			TargetDir:  cfg.TargetDir,
		}
	}

	return deps, nil
}
