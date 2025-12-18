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

// Package plugin provides SeaTunnel plugin marketplace management.
// plugin 包提供 SeaTunnel 插件市场管理功能。
package plugin

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Downloader errors / 下载器错误
var (
	ErrDownloadFailed     = errors.New("download failed / 下载失败")
	ErrChecksumMismatch   = errors.New("checksum mismatch / 校验和不匹配")
	ErrFileNotFound       = errors.New("file not found on server / 服务器上未找到文件")
	ErrDownloadCancelled  = errors.New("download cancelled / 下载已取消")
	ErrDownloadInProgress = errors.New("download already in progress / 下载正在进行中")
)

// DownloadProgress represents the progress of a download task.
// DownloadProgress 表示下载任务的进度。
type DownloadProgress struct {
	PluginName      string     `json:"plugin_name"`        // 插件名称 / Plugin name
	Version         string     `json:"version"`            // 版本号 / Version
	Status          string     `json:"status"`             // 状态: pending/downloading/completed/failed / Status
	Progress        int        `json:"progress"`           // 进度 (0-100) / Progress
	CurrentStep     string     `json:"current_step"`       // 当前步骤 / Current step
	DownloadedBytes int64      `json:"downloaded_bytes"`   // 已下载字节 / Downloaded bytes
	TotalBytes      int64      `json:"total_bytes"`        // 总字节 / Total bytes
	Speed           int64      `json:"speed"`              // 下载速度 (bytes/s) / Download speed
	Message         string     `json:"message,omitempty"`  // 消息 / Message
	Error           string     `json:"error,omitempty"`    // 错误信息 / Error message
	StartTime       time.Time  `json:"start_time"`         // 开始时间 / Start time
	EndTime         *time.Time `json:"end_time,omitempty"` // 结束时间 / End time
}

// ProgressCallback is a callback function for reporting download progress.
// ProgressCallback 是用于报告下载进度的回调函数。
type ProgressCallback func(progress *DownloadProgress)

// Downloader handles plugin file downloads from Maven repositories.
// Downloader 处理从 Maven 仓库下载插件文件。
type Downloader struct {
	// pluginsDir is the base directory for storing downloaded plugins
	// pluginsDir 是存储下载插件的基础目录
	pluginsDir string

	// httpClient is the HTTP client for downloading files
	// httpClient 是用于下载文件的 HTTP 客户端
	httpClient *http.Client

	// activeDownloads tracks currently active downloads
	// activeDownloads 跟踪当前活动的下载
	activeDownloads map[string]*DownloadProgress
	downloadsMu     sync.RWMutex

	// cancelFuncs stores cancel functions for active downloads
	// cancelFuncs 存储活动下载的取消函数
	cancelFuncs map[string]context.CancelFunc
	cancelMu    sync.Mutex
}

// NewDownloader creates a new Downloader instance.
// NewDownloader 创建一个新的 Downloader 实例。
func NewDownloader(pluginsDir string) *Downloader {
	return &Downloader{
		pluginsDir: pluginsDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Minute, // Long timeout for large files / 大文件的长超时
		},
		activeDownloads: make(map[string]*DownloadProgress),
		cancelFuncs:     make(map[string]context.CancelFunc),
	}
}

// GetPluginsDir returns the plugins directory path.
// GetPluginsDir 返回插件目录路径。
func (d *Downloader) GetPluginsDir() string {
	return d.pluginsDir
}

// GetConnectorPath returns the path for a connector jar file.
// GetConnectorPath 返回连接器 jar 文件的路径。
// Path format: plugins_dir/${version}/connectors/${artifactId}-${version}.jar
// The artifactID should be provided directly from plugin.ArtifactID (e.g., connector-cdc-mysql)
// artifactID 应该直接从 plugin.ArtifactID 提供（如 connector-cdc-mysql）
func (d *Downloader) GetConnectorPath(artifactID, version string) string {
	return filepath.Join(d.pluginsDir, version, "connectors", fmt.Sprintf("%s-%s.jar", artifactID, version))
}

// GetConnectorPathByName returns the path for a connector jar file by plugin name.
// GetConnectorPathByName 通过插件名称返回连接器 jar 文件的路径。
// This is a convenience method that looks up the artifact ID from the plugin name.
// 这是一个便捷方法，从插件名称查找 artifact ID。
// Deprecated: Use GetConnectorPath with artifact_id directly when possible.
// 已弃用：尽可能直接使用 GetConnectorPath 和 artifact_id。
func (d *Downloader) GetConnectorPathByName(name, version string) string {
	artifactID := getArtifactIDForPath(name)
	return d.GetConnectorPath(artifactID, version)
}

// pluginArtifactMappingsForPath contains all special plugin name to artifact ID mappings.
// pluginArtifactMappingsForPath 包含所有特殊的插件名称到 artifact ID 的映射。
// This mapping is based on SeaTunnel's Maven repository structure.
// 此映射基于 SeaTunnel 的 Maven 仓库结构。
// NOTE: This must be kept in sync with service.go's pluginArtifactMappings
// 注意：此映射必须与 service.go 的 pluginArtifactMappings 保持同步 TODO 看看后面有没有更方便的页面抓取
var pluginArtifactMappingsForPath = map[string]string{
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
	"db2":        "connector-jdbc", // Driver: com.ibm.db2.jcc.DB2Driver
	"tablestore": "connector-jdbc", // Driver: com.alicloud.openservices.tablestore.jdbc.OTSDriver
	"saphana":    "connector-jdbc", // Driver: com.sap.db.jdbc.Driver
	"teradata":   "connector-jdbc", // Driver: com.teradata.jdbc.TeraDriver
	"snowflake":  "connector-jdbc", // Driver: net.snowflake.client.jdbc.SnowflakeDriver
	"redshift":   "connector-jdbc", // Driver: com.amazon.redshift.jdbc42.Driver
	"vertica":    "connector-jdbc", // Driver: com.vertica.jdbc.Driver
	"kingbase":   "connector-jdbc", // Driver: com.kingbase8.Driver (人大金仓)
	"oceanbase":  "connector-jdbc", // Driver: com.oceanbase.jdbc.Driver
	"xugu":       "connector-jdbc", // Driver: com.xugu.cloudjdbc.Driver (虚谷数据库)
	"iris":       "connector-jdbc", // Driver: com.intersystems.jdbc.IRISDriver
	"opengauss":  "connector-jdbc", // Driver: org.opengauss.Driver
	"highgo":     "connector-jdbc", // Driver: com.highgo.jdbc.Driver (瀚高数据库)
	"presto":     "connector-jdbc", // Driver: com.facebook.presto.jdbc.PrestoDriver
	"trino":      "connector-jdbc", // Driver: io.trino.jdbc.TrinoDriver
}

// getArtifactIDForPath returns the correct Maven artifact ID for a plugin name.
// getArtifactIDForPath 返回插件名称对应的正确 Maven artifact ID。
// This is used as a fallback when artifact_id is not available.
// 当 artifact_id 不可用时作为备用。
func getArtifactIDForPath(name string) string {
	// Check special mappings first / 首先检查特殊映射
	if artifactID, ok := pluginArtifactMappingsForPath[name]; ok {
		return artifactID
	}

	// Default: connector-${name} / 默认：connector-${name}
	return fmt.Sprintf("connector-%s", name)
}

// GetLibPath returns the path for a dependency library file.
// GetLibPath 返回依赖库文件的路径。
// Path format: plugins_dir/${version}/lib/${artifactId}-${version}.jar
func (d *Downloader) GetLibPath(artifactID, version, pluginVersion string) string {
	return filepath.Join(d.pluginsDir, pluginVersion, "lib", fmt.Sprintf("%s-%s.jar", artifactID, version))
}

// IsConnectorDownloaded checks if a connector is already downloaded.
// IsConnectorDownloaded 检查连接器是否已下载。
// Note: Uses GetConnectorPathByName which maps plugin name to artifact ID.
// 注意：使用 GetConnectorPathByName 将插件名称映射到 artifact ID。
func (d *Downloader) IsConnectorDownloaded(name, version string) bool {
	path := d.GetConnectorPathByName(name, version)
	_, err := os.Stat(path)
	return err == nil
}

// IsConnectorDownloadedByArtifactID checks if a connector is already downloaded using artifact ID.
// IsConnectorDownloadedByArtifactID 使用 artifact ID 检查连接器是否已下载。
func (d *Downloader) IsConnectorDownloadedByArtifactID(artifactID, version string) bool {
	path := d.GetConnectorPath(artifactID, version)
	_, err := os.Stat(path)
	return err == nil
}

// ReadPluginFile reads the plugin file content from local storage.
// ReadPluginFile 从本地存储读取插件文件内容。
// Note: Uses GetConnectorPathByName which maps plugin name to artifact ID.
// 注意：使用 GetConnectorPathByName 将插件名称映射到 artifact ID。
func (d *Downloader) ReadPluginFile(name, version string) ([]byte, error) {
	path := d.GetConnectorPathByName(name, version)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin file %s: %w / 读取插件文件 %s 失败: %w", path, err, path, err)
	}
	return data, nil
}

// ReadPluginFileByArtifactID reads the plugin file content using artifact ID.
// ReadPluginFileByArtifactID 使用 artifact ID 读取插件文件内容。
func (d *Downloader) ReadPluginFileByArtifactID(artifactID, version string) ([]byte, error) {
	path := d.GetConnectorPath(artifactID, version)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin file %s: %w / 读取插件文件 %s 失败: %w", path, err, path, err)
	}
	return data, nil
}

// IsDependencyDownloaded checks if a dependency is already downloaded.
// IsDependencyDownloaded 检查依赖是否已下载。
func (d *Downloader) IsDependencyDownloaded(artifactID, depVersion, pluginVersion string) bool {
	path := d.GetLibPath(artifactID, depVersion, pluginVersion)
	_, err := os.Stat(path)
	return err == nil
}

// GetDownloadProgress returns the current download progress for a plugin.
// GetDownloadProgress 返回插件的当前下载进度。
func (d *Downloader) GetDownloadProgress(name, version string) *DownloadProgress {
	key := fmt.Sprintf("%s:%s", name, version)
	d.downloadsMu.RLock()
	defer d.downloadsMu.RUnlock()
	return d.activeDownloads[key]
}

// ListActiveDownloads returns all active download tasks.
// ListActiveDownloads 返回所有活动的下载任务。
func (d *Downloader) ListActiveDownloads() []*DownloadProgress {
	d.downloadsMu.RLock()
	defer d.downloadsMu.RUnlock()

	result := make([]*DownloadProgress, 0, len(d.activeDownloads))
	for _, p := range d.activeDownloads {
		result = append(result, p)
	}
	return result
}

// CancelDownload cancels an active download.
// CancelDownload 取消活动的下载。
func (d *Downloader) CancelDownload(name, version string) error {
	key := fmt.Sprintf("%s:%s", name, version)

	d.cancelMu.Lock()
	cancel, exists := d.cancelFuncs[key]
	d.cancelMu.Unlock()

	if !exists {
		return fmt.Errorf("no active download for %s:%s", name, version)
	}

	cancel()
	return nil
}

// DownloadConnector downloads a connector jar from Maven repository.
// DownloadConnector 从 Maven 仓库下载连接器 jar。
// Downloads to: plugins_dir/${version}/connectors/connector-${name}-${version}.jar
func (d *Downloader) DownloadConnector(ctx context.Context, plugin *Plugin, mirror MirrorSource, callback ProgressCallback) error {
	key := fmt.Sprintf("%s:%s", plugin.Name, plugin.Version)

	// Check if already downloading / 检查是否正在下载
	d.downloadsMu.RLock()
	if _, exists := d.activeDownloads[key]; exists {
		d.downloadsMu.RUnlock()
		return ErrDownloadInProgress
	}
	d.downloadsMu.RUnlock()

	// Create cancellable context / 创建可取消的上下文
	downloadCtx, cancel := context.WithCancel(ctx)

	d.cancelMu.Lock()
	d.cancelFuncs[key] = cancel
	d.cancelMu.Unlock()

	defer func() {
		d.cancelMu.Lock()
		delete(d.cancelFuncs, key)
		d.cancelMu.Unlock()
	}()

	// Initialize progress / 初始化进度
	progress := &DownloadProgress{
		PluginName:  plugin.Name,
		Version:     plugin.Version,
		Status:      "downloading",
		Progress:    0,
		CurrentStep: "Downloading connector / 下载连接器",
		StartTime:   time.Now(),
	}

	d.downloadsMu.Lock()
	d.activeDownloads[key] = progress
	d.downloadsMu.Unlock()

	defer func() {
		d.downloadsMu.Lock()
		delete(d.activeDownloads, key)
		d.downloadsMu.Unlock()
	}()

	// Build Maven URL / 构建 Maven URL
	baseURL := MirrorURLs[mirror]
	if baseURL == "" {
		baseURL = MirrorURLs[MirrorSourceApache]
	}

	// Maven path: groupId/artifactId/version/artifactId-version.jar
	// Example: org/apache/seatunnel/connector-jdbc/2.3.12/connector-jdbc-2.3.12.jar
	groupPath := strings.ReplaceAll(plugin.GroupID, ".", "/")

	// Ensure artifact_id is set / 确保 artifact_id 已设置
	artifactID := plugin.ArtifactID
	if artifactID == "" {
		// Fallback to mapping if artifact_id is not set / 如果 artifact_id 未设置则使用映射
		artifactID = getArtifactIDForPath(plugin.Name)
		fmt.Printf("[Download] Warning: plugin.ArtifactID is empty for %s, using fallback: %s\n", plugin.Name, artifactID)
	}

	jarName := fmt.Sprintf("%s-%s.jar", artifactID, plugin.Version)
	jarURL := fmt.Sprintf("%s/%s/%s/%s/%s", baseURL, groupPath, artifactID, plugin.Version, jarName)
	sha1URL := jarURL + ".sha1"

	fmt.Printf("[Download] URL: %s\n", jarURL)

	// Create target directory / 创建目标目录
	// Use artifact_id directly for file path / 直接使用 artifact_id 作为文件路径
	targetPath := d.GetConnectorPath(artifactID, plugin.Version)
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		progress.Status = "failed"
		progress.Error = err.Error()
		if callback != nil {
			callback(progress)
		}
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Download the jar file / 下载 jar 文件
	if err := d.downloadFile(downloadCtx, jarURL, targetPath, progress, callback); err != nil {
		progress.Status = "failed"
		progress.Error = err.Error()
		now := time.Now()
		progress.EndTime = &now
		if callback != nil {
			callback(progress)
		}
		return err
	}

	// Verify checksum / 验证校验和
	progress.CurrentStep = "Verifying checksum / 验证校验和"
	if callback != nil {
		callback(progress)
	}

	if err := d.verifyChecksum(downloadCtx, targetPath, sha1URL); err != nil {
		// Remove the downloaded file if checksum fails / 如果校验失败则删除下载的文件
		os.Remove(targetPath)
		progress.Status = "failed"
		progress.Error = err.Error()
		now := time.Now()
		progress.EndTime = &now
		if callback != nil {
			callback(progress)
		}
		return err
	}

	// Mark as completed / 标记为完成
	progress.Status = "completed"
	progress.Progress = 100
	progress.CurrentStep = "Completed / 完成"
	now := time.Now()
	progress.EndTime = &now
	if callback != nil {
		callback(progress)
	}

	return nil
}

// DownloadDependencies downloads all dependencies for a plugin.
// DownloadDependencies 下载插件的所有依赖。
// Downloads to: plugins_dir/${version}/lib/
func (d *Downloader) DownloadDependencies(ctx context.Context, plugin *Plugin, mirror MirrorSource, callback ProgressCallback) error {
	if len(plugin.Dependencies) == 0 {
		return nil
	}

	baseURL := MirrorURLs[mirror]
	if baseURL == "" {
		baseURL = MirrorURLs[MirrorSourceApache]
	}

	// Create lib directory / 创建 lib 目录
	libDir := filepath.Join(d.pluginsDir, plugin.Version, "lib")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		return fmt.Errorf("failed to create lib directory: %w", err)
	}

	for i, dep := range plugin.Dependencies {
		select {
		case <-ctx.Done():
			return ErrDownloadCancelled
		default:
		}

		// Check if already downloaded / 检查是否已下载
		if d.IsDependencyDownloaded(dep.ArtifactID, dep.Version, plugin.Version) {
			continue
		}

		// Build Maven URL / 构建 Maven URL
		groupPath := strings.ReplaceAll(dep.GroupID, ".", "/")
		jarName := fmt.Sprintf("%s-%s.jar", dep.ArtifactID, dep.Version)
		jarURL := fmt.Sprintf("%s/%s/%s/%s/%s", baseURL, groupPath, dep.ArtifactID, dep.Version, jarName)
		sha1URL := jarURL + ".sha1"

		targetPath := d.GetLibPath(dep.ArtifactID, dep.Version, plugin.Version)

		// Create progress for this dependency / 为此依赖创建进度
		progress := &DownloadProgress{
			PluginName: plugin.Name,
			Version:    plugin.Version,
			Status:     "downloading",
			Progress:   (i * 100) / len(plugin.Dependencies),
			CurrentStep: fmt.Sprintf("Downloading dependency %d/%d: %s / 下载依赖 %d/%d: %s",
				i+1, len(plugin.Dependencies), dep.ArtifactID,
				i+1, len(plugin.Dependencies), dep.ArtifactID),
			StartTime: time.Now(),
		}

		if callback != nil {
			callback(progress)
		}

		// Download the dependency / 下载依赖
		if err := d.downloadFile(ctx, jarURL, targetPath, progress, callback); err != nil {
			// Log warning but continue with other dependencies / 记录警告但继续下载其他依赖
			progress.Message = fmt.Sprintf("Warning: failed to download %s: %v", dep.ArtifactID, err)
			if callback != nil {
				callback(progress)
			}
			continue
		}

		// Verify checksum (optional for dependencies) / 验证校验和（依赖可选）
		if err := d.verifyChecksum(ctx, targetPath, sha1URL); err != nil {
			// Log warning but don't fail / 记录警告但不失败
			progress.Message = fmt.Sprintf("Warning: checksum verification failed for %s", dep.ArtifactID)
			if callback != nil {
				callback(progress)
			}
		}
	}

	return nil
}

// DownloadPlugin downloads a plugin and all its dependencies.
// DownloadPlugin 下载插件及其所有依赖。
func (d *Downloader) DownloadPlugin(ctx context.Context, plugin *Plugin, mirror MirrorSource, callback ProgressCallback) error {
	// Download connector first / 首先下载连接器
	if err := d.DownloadConnector(ctx, plugin, mirror, callback); err != nil {
		return fmt.Errorf("failed to download connector: %w", err)
	}

	// Download dependencies / 下载依赖
	if err := d.DownloadDependencies(ctx, plugin, mirror, callback); err != nil {
		return fmt.Errorf("failed to download dependencies: %w", err)
	}

	return nil
}

// downloadFile downloads a file from URL to the target path with progress reporting.
// downloadFile 从 URL 下载文件到目标路径，并报告进度。
func (d *Downloader) downloadFile(ctx context.Context, url, targetPath string, progress *DownloadProgress, callback ProgressCallback) error {
	// Create HTTP request / 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request / 执行请求
	resp, err := d.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return ErrDownloadCancelled
		}
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrFileNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: HTTP %d", ErrDownloadFailed, resp.StatusCode)
	}

	// Get content length / 获取内容长度
	progress.TotalBytes = resp.ContentLength

	// Create target file / 创建目标文件
	tmpPath := targetPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		file.Close()
		// Clean up temp file on error / 出错时清理临时文件
		if progress.Status != "completed" {
			os.Remove(tmpPath)
		}
	}()

	// Download with progress tracking / 带进度跟踪的下载
	buf := make([]byte, 32*1024) // 32KB buffer
	startTime := time.Now()
	lastReportTime := startTime

	for {
		select {
		case <-ctx.Done():
			return ErrDownloadCancelled
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("failed to write file: %w", writeErr)
			}

			progress.DownloadedBytes += int64(n)

			// Calculate progress and speed / 计算进度和速度
			if progress.TotalBytes > 0 {
				progress.Progress = int((progress.DownloadedBytes * 100) / progress.TotalBytes)
			}

			elapsed := time.Since(startTime).Seconds()
			if elapsed > 0 {
				progress.Speed = int64(float64(progress.DownloadedBytes) / elapsed)
			}

			// Report progress every 500ms / 每 500ms 报告一次进度
			if time.Since(lastReportTime) > 500*time.Millisecond && callback != nil {
				callback(progress)
				lastReportTime = time.Now()
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
	}

	// Close file before rename / 重命名前关闭文件
	file.Close()

	// Rename temp file to target / 将临时文件重命名为目标文件
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// verifyChecksum verifies the SHA1 checksum of a downloaded file.
// verifyChecksum 验证下载文件的 SHA1 校验和。
func (d *Downloader) verifyChecksum(ctx context.Context, filePath, sha1URL string) error {
	// Download SHA1 checksum / 下载 SHA1 校验和
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sha1URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create checksum request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		// If checksum file not available, skip verification / 如果校验和文件不可用，跳过验证
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// If checksum file not available, skip verification / 如果校验和文件不可用，跳过验证
		return nil
	}

	// Read expected checksum / 读取预期的校验和
	checksumBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil // Skip verification on read error / 读取错误时跳过验证
	}

	expectedChecksum := strings.TrimSpace(string(checksumBytes))
	// Some checksum files contain additional info, extract just the hash / 某些校验和文件包含额外信息，只提取哈希值
	if idx := strings.Index(expectedChecksum, " "); idx > 0 {
		expectedChecksum = expectedChecksum[:idx]
	}

	// Calculate actual checksum / 计算实际校验和
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for checksum: %w", err)
	}
	defer file.Close()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))

	if !strings.EqualFold(actualChecksum, expectedChecksum) {
		return fmt.Errorf("%w: expected %s, got %s", ErrChecksumMismatch, expectedChecksum, actualChecksum)
	}

	return nil
}

// ListLocalPlugins returns a list of locally downloaded plugins.
// ListLocalPlugins 返回本地已下载的插件列表。
func (d *Downloader) ListLocalPlugins() ([]LocalPlugin, error) {
	var plugins []LocalPlugin

	// Walk through plugins directory / 遍历插件目录
	entries, err := os.ReadDir(d.pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return plugins, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		version := entry.Name()
		connectorsDir := filepath.Join(d.pluginsDir, version, "connectors")

		// List connector jars / 列出连接器 jar
		connectorEntries, err := os.ReadDir(connectorsDir)
		if err != nil {
			continue
		}

		for _, connEntry := range connectorEntries {
			if connEntry.IsDir() || !strings.HasSuffix(connEntry.Name(), ".jar") {
				continue
			}

			// Parse plugin name from filename / 从文件名解析插件名称
			// Handles multiple formats:
			// - connector-{name}-{version}.jar (standard)
			// - connector-cdc-{database}-{version}.jar (CDC connectors)
			// - connector-file-{type}-{version}.jar (file connectors)
			name := parsePluginNameFromFilename(connEntry.Name(), version)

			info, _ := connEntry.Info()
			var size int64
			var modTime time.Time
			if info != nil {
				size = info.Size()
				modTime = info.ModTime()
			}

			// Determine category from plugin name / 从插件名称判断分类
			category := determinePluginCategory(name)

			plugins = append(plugins, LocalPlugin{
				Name:          name,
				Version:       version,
				Category:      category,
				ConnectorPath: filepath.Join(connectorsDir, connEntry.Name()),
				Size:          size,
				DownloadedAt:  modTime,
			})
		}
	}

	return plugins, nil
}

// parsePluginNameFromFilename extracts the plugin name from a jar filename.
// parsePluginNameFromFilename 从 jar 文件名中提取插件名称。
func parsePluginNameFromFilename(filename, version string) string {
	// Remove .jar suffix and version / 移除 .jar 后缀和版本号
	name := strings.TrimSuffix(filename, fmt.Sprintf("-%s.jar", version))

	// Handle CDC connectors: connector-cdc-{database} -> {database}-cdc
	// 处理 CDC 连接器：connector-cdc-{database} -> {database}-cdc
	if strings.HasPrefix(name, "connector-cdc-") {
		database := strings.TrimPrefix(name, "connector-cdc-")
		return database + "-cdc"
	}

	// Handle file connectors: connector-file-{type} -> {type}file
	// 处理文件连接器：connector-file-{type} -> {type}file
	if strings.HasPrefix(name, "connector-file-") {
		fileType := strings.TrimPrefix(name, "connector-file-")
		// Map back to original names / 映射回原始名称
		fileTypeMap := map[string]string{
			"local":  "localfile",
			"hadoop": "hdfsfile",
			"s3":     "s3file",
			"oss":    "ossfile",
			"ftp":    "ftpfile",
			"sftp":   "sftpfile",
			"cos":    "cosfile",
			"obs":    "obsfile",
		}
		if originalName, ok := fileTypeMap[fileType]; ok {
			return originalName
		}
		return fileType + "file"
	}

	// Standard format: connector-{name} -> {name}
	// 标准格式：connector-{name} -> {name}
	if strings.HasPrefix(name, "connector-") {
		return strings.TrimPrefix(name, "connector-")
	}

	return name
}

// determinePluginCategory determines the category of a plugin from its name.
// determinePluginCategory 从插件名称判断插件分类。
func determinePluginCategory(name string) PluginCategory {
	lowerName := strings.ToLower(name)

	// Sink plugins typically have "-sink" suffix or are known sink connectors
	// 目标插件通常有 "-sink" 后缀或是已知的目标连接器
	if strings.HasSuffix(lowerName, "-sink") || strings.HasSuffix(lowerName, "sink") {
		return PluginCategorySink
	}

	// Transform plugins / 转换插件
	if strings.HasPrefix(lowerName, "transform-") ||
		lowerName == "filter" || lowerName == "sql" ||
		lowerName == "field-mapper" || lowerName == "replace" || lowerName == "split" {
		return PluginCategoryTransform
	}

	// Default to source / 默认为Source
	return PluginCategorySource
}

// DeleteLocalPlugin deletes a locally downloaded plugin.
// DeleteLocalPlugin 删除本地已下载的插件。
// Note: Uses GetConnectorPathByName which maps plugin name to artifact ID.
// 注意：使用 GetConnectorPathByName 将插件名称映射到 artifact ID。
func (d *Downloader) DeleteLocalPlugin(name, version string) error {
	connectorPath := d.GetConnectorPathByName(name, version)

	// Check if exists / 检查是否存在
	if _, err := os.Stat(connectorPath); os.IsNotExist(err) {
		return ErrFileNotFound
	}

	// Remove connector jar / 删除连接器 jar
	if err := os.Remove(connectorPath); err != nil {
		return fmt.Errorf("failed to delete connector: %w", err)
	}

	return nil
}

// LocalPlugin represents a locally downloaded plugin.
// LocalPlugin 表示本地已下载的插件。
type LocalPlugin struct {
	Name          string         `json:"name"`           // 插件名称 / Plugin name
	Version       string         `json:"version"`        // 版本号 / Version
	Category      PluginCategory `json:"category"`       // 插件分类 / Plugin category
	ConnectorPath string         `json:"connector_path"` // 连接器路径 / Connector path
	Size          int64          `json:"size"`           // 文件大小 / File size
	DownloadedAt  time.Time      `json:"downloaded_at"`  // 下载时间 / Downloaded at
}
