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

package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/seatunnel/seatunnelX/internal/config"
)

// Common errors / 常见错误
var (
	ErrPackageNotFound        = errors.New("package not found / 安装包未找到")
	ErrInstallationNotFound   = errors.New("installation not found / 安装任务未找到")
	ErrInstallationInProgress = errors.New("installation already in progress / 安装任务正在进行中")
	ErrHostNotConnected       = errors.New("host agent not connected / 主机 Agent 未连接")
)

// MirrorURLs maps mirror sources to their base URLs
// MirrorURLs 将镜像源映射到其基础 URL
var MirrorURLs = map[MirrorSource]string{
	MirrorAliyun:      "https://mirrors.aliyun.com/apache/seatunnel",
	MirrorApache:      "https://archive.apache.org/dist/seatunnel",
	MirrorHuaweiCloud: "https://mirrors.huaweicloud.com/apache/seatunnel",
}

// FallbackVersions is the fallback version list when online fetch fails
// FallbackVersions 是在线获取失败时的备用版本列表
var FallbackVersions = []string{
	"2.3.12",
	"2.3.11",
	"2.3.10",
	"2.3.9",
	"2.3.8",
	"2.3.7",
	"2.3.6",
	"2.3.5",
	"2.3.4",
	"2.3.3",
	"2.3.2",
	"2.3.1",
	"2.3.0",
	"2.2.0-beta",
	"2.1.3",
	"2.1.2",
	"2.1.1",
	"2.1.0",
}

// RecommendedVersion is the recommended SeaTunnel version
// RecommendedVersion 是推荐的 SeaTunnel 版本
const RecommendedVersion = "2.3.12"

// ApacheArchiveURL is the URL to fetch version list from Apache Archive
// ApacheArchiveURL 是从 Apache Archive 获取版本列表的 URL
const ApacheArchiveURL = "https://archive.apache.org/dist/seatunnel/"

// VersionCacheDuration is how long to cache the version list
// VersionCacheDuration 是版本列表的缓存时间
const VersionCacheDuration = 1 * time.Hour

// Service provides installation management functionality.
// Service 提供安装管理功能。
type Service struct {
	// packageDir is the directory for storing local packages
	// packageDir 是存储本地安装包的目录
	packageDir string

	// tempDir is the directory for temporary files (downloads in progress)
	// tempDir 是临时文件目录（下载中的文件）
	tempDir string

	// installations tracks ongoing installations by host ID
	// installations 按主机 ID 跟踪正在进行的安装
	installations map[string]*InstallationStatus
	installMu     sync.RWMutex

	// downloads tracks ongoing download tasks by version
	// downloads 按版本跟踪正在进行的下载任务
	downloads   map[string]*DownloadTask
	downloadsMu sync.RWMutex

	// cachedVersions stores the cached version list from Apache Archive
	// cachedVersions 存储从 Apache Archive 获取的缓存版本列表
	cachedVersions []string
	// versionsCacheTime is when the version cache was last updated
	// versionsCacheTime 是版本缓存最后更新的时间
	versionsCacheTime time.Time
	// versionsMu protects version cache access
	// versionsMu 保护版本缓存访问
	versionsMu sync.RWMutex

	// agentManager is used to communicate with agents
	// agentManager 用于与 Agent 通信
	// agentManager *agent.Manager // TODO: inject agent manager
}

// NewService creates a new Service instance.
// NewService 创建一个新的 Service 实例。
// If packageDir is empty, it uses the configured packages directory.
// 如果 packageDir 为空，则使用配置的安装包目录。
func NewService(packageDir string) *Service {
	// Use configured directory if not specified / 如果未指定则使用配置的目录
	if packageDir == "" {
		packageDir = config.GetPackagesDir()
	}

	// Create package directory if not exists / 如果不存在则创建安装包目录
	if err := os.MkdirAll(packageDir, 0755); err != nil {
		// Log error but continue / 记录错误但继续
	}

	// Also create temp directory / 同时创建临时目录
	if err := os.MkdirAll(config.GetTempDir(), 0755); err != nil {
		// Log error but continue / 记录错误但继续
	}

	return &Service{
		packageDir:    packageDir,
		tempDir:       config.GetTempDir(),
		installations: make(map[string]*InstallationStatus),
		downloads:     make(map[string]*DownloadTask),
	}
}

// NewServiceWithDefaults creates a new Service instance with default configuration.
// NewServiceWithDefaults 使用默认配置创建新的 Service 实例。
func NewServiceWithDefaults() *Service {
	return NewService("")
}

// ==================== Version Management 版本管理 ====================

// getVersions returns the version list, using cache if valid, otherwise fetching from Apache Archive.
// getVersions 返回版本列表，如果缓存有效则使用缓存，否则从 Apache Archive 获取。
func (s *Service) getVersions(ctx context.Context) []string {
	s.versionsMu.RLock()
	// Check if cache is valid / 检查缓存是否有效
	if len(s.cachedVersions) > 0 && time.Since(s.versionsCacheTime) < VersionCacheDuration {
		versions := s.cachedVersions
		s.versionsMu.RUnlock()
		return versions
	}
	s.versionsMu.RUnlock()

	// Try to fetch from Apache Archive / 尝试从 Apache Archive 获取
	versions, err := s.fetchVersionsFromApache(ctx)
	if err != nil {
		// Use fallback versions on error / 出错时使用备用版本
		return FallbackVersions
	}

	// Update cache / 更新缓存
	s.versionsMu.Lock()
	s.cachedVersions = versions
	s.versionsCacheTime = time.Now()
	s.versionsMu.Unlock()

	return versions
}

// fetchVersionsFromApache fetches the version list from Apache Archive.
// fetchVersionsFromApache 从 Apache Archive 获取版本列表。
func (s *Service) fetchVersionsFromApache(ctx context.Context) ([]string, error) {
	// Create HTTP request with timeout / 创建带超时的 HTTP 请求
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ApacheArchiveURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch versions: %w", err)
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

	// Parse HTML to extract version directories / 解析 HTML 提取版本目录
	// Apache Archive HTML format: <a href="2.3.12/">2.3.12/</a>
	// Apache Archive HTML 格式: <a href="2.3.12/">2.3.12/</a>
	versionRegex := regexp.MustCompile(`<a href="(\d+\.\d+\.\d+(?:-[a-zA-Z0-9]+)?)/?">\d+\.\d+\.\d+(?:-[a-zA-Z0-9]+)?/?</a>`)
	matches := versionRegex.FindAllStringSubmatch(string(body), -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf("no versions found in response")
	}

	// Extract versions and sort in descending order / 提取版本并按降序排序
	versions := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 2 {
			version := strings.TrimSuffix(match[1], "/")
			versions = append(versions, version)
		}
	}

	// Sort versions in descending order (newest first) / 按降序排序（最新版本在前）
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) > 0
	})

	return versions, nil
}

// RefreshVersions forces a refresh of the version list from Apache Archive.
// RefreshVersions 强制从 Apache Archive 刷新版本列表。
func (s *Service) RefreshVersions(ctx context.Context) ([]string, error) {
	versions, err := s.fetchVersionsFromApache(ctx)
	if err != nil {
		return FallbackVersions, err
	}

	// Update cache / 更新缓存
	s.versionsMu.Lock()
	s.cachedVersions = versions
	s.versionsCacheTime = time.Now()
	s.versionsMu.Unlock()

	return versions, nil
}

// compareVersions compares two version strings.
// compareVersions 比较两个版本字符串。
// Returns: >0 if v1 > v2, <0 if v1 < v2, 0 if equal
// 返回: >0 如果 v1 > v2, <0 如果 v1 < v2, 0 如果相等
func compareVersions(v1, v2 string) int {
	// Split by dots and compare each part / 按点分割并比较每个部分
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 string
		if i < len(parts1) {
			p1 = parts1[i]
		}
		if i < len(parts2) {
			p2 = parts2[i]
		}

		// Handle suffix like "-beta" / 处理后缀如 "-beta"
		n1, s1 := parseVersionPart(p1)
		n2, s2 := parseVersionPart(p2)

		if n1 != n2 {
			return n1 - n2
		}
		// If numbers are equal, compare suffixes (no suffix > with suffix)
		// 如果数字相等，比较后缀（无后缀 > 有后缀）
		if s1 != s2 {
			if s1 == "" {
				return 1
			}
			if s2 == "" {
				return -1
			}
			return strings.Compare(s1, s2)
		}
	}
	return 0
}

// parseVersionPart parses a version part like "12" or "0-beta".
// parseVersionPart 解析版本部分如 "12" 或 "0-beta"。
func parseVersionPart(part string) (int, string) {
	if part == "" {
		return 0, ""
	}

	// Split by hyphen for suffix / 按连字符分割后缀
	idx := strings.Index(part, "-")
	if idx == -1 {
		var num int
		fmt.Sscanf(part, "%d", &num)
		return num, ""
	}

	var num int
	fmt.Sscanf(part[:idx], "%d", &num)
	return num, part[idx:]
}

// ==================== Package Management 安装包管理 ====================

// ListAvailableVersions returns available SeaTunnel versions.
// ListAvailableVersions 返回可用的 SeaTunnel 版本。
func (s *Service) ListAvailableVersions(ctx context.Context) (*AvailableVersions, error) {
	// Get versions (from cache, online, or fallback)
	// 获取版本（从缓存、在线或备用列表）
	versions := s.getVersions(ctx)

	result := &AvailableVersions{
		Versions:           versions,
		RecommendedVersion: RecommendedVersion,
		LocalPackages:      make([]PackageInfo, 0),
	}

	// Scan local packages / 扫描本地安装包
	entries, err := os.ReadDir(s.packageDir)
	if err != nil {
		// Directory might not exist, return empty list / 目录可能不存在，返回空列表
		return result, nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Check if it's a SeaTunnel package / 检查是否是 SeaTunnel 安装包
		name := entry.Name()
		if !isSeaTunnelPackage(name) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		version := extractVersionFromFileName(name)
		uploadedAt := info.ModTime()

		result.LocalPackages = append(result.LocalPackages, PackageInfo{
			Version:      version,
			FileName:     name,
			FileSize:     info.Size(),
			IsLocal:      true,
			LocalPath:    filepath.Join(s.packageDir, name),
			UploadedAt:   &uploadedAt,
			DownloadURLs: getDownloadURLs(version),
		})
	}

	return result, nil
}

// GetPackageInfo returns information about a specific package version.
// GetPackageInfo 返回特定版本安装包的信息。
func (s *Service) GetPackageInfo(ctx context.Context, version string) (*PackageInfo, error) {
	// Check if local package exists / 检查本地安装包是否存在
	fileName := fmt.Sprintf("apache-seatunnel-%s-bin.tar.gz", version)
	localPath := filepath.Join(s.packageDir, fileName)

	info := &PackageInfo{
		Version:      version,
		FileName:     fileName,
		DownloadURLs: getDownloadURLs(version),
	}

	if fileInfo, err := os.Stat(localPath); err == nil {
		info.IsLocal = true
		info.LocalPath = localPath
		info.FileSize = fileInfo.Size()
		uploadedAt := fileInfo.ModTime()
		info.UploadedAt = &uploadedAt

		// Calculate checksum / 计算校验和
		checksum, err := calculateChecksum(localPath)
		if err == nil {
			info.Checksum = checksum
		}
	}

	return info, nil
}

// UploadPackage handles package file upload.
// UploadPackage 处理安装包文件上传。
func (s *Service) UploadPackage(ctx context.Context, version string, file *multipart.FileHeader) (*PackageInfo, error) {
	// Open uploaded file / 打开上传的文件
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Create destination file / 创建目标文件
	fileName := fmt.Sprintf("apache-seatunnel-%s-bin.tar.gz", version)
	destPath := filepath.Join(s.packageDir, fileName)

	dst, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Copy file content / 复制文件内容
	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(destPath) // Clean up on error / 出错时清理
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Get file info / 获取文件信息
	fileInfo, err := os.Stat(destPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Calculate checksum / 计算校验和
	checksum, _ := calculateChecksum(destPath)

	uploadedAt := fileInfo.ModTime()
	return &PackageInfo{
		Version:      version,
		FileName:     fileName,
		FileSize:     fileInfo.Size(),
		Checksum:     checksum,
		IsLocal:      true,
		LocalPath:    destPath,
		UploadedAt:   &uploadedAt,
		DownloadURLs: getDownloadURLs(version),
	}, nil
}

// DeletePackage deletes a local package.
// DeletePackage 删除本地安装包。
func (s *Service) DeletePackage(ctx context.Context, version string) error {
	fileName := fmt.Sprintf("apache-seatunnel-%s-bin.tar.gz", version)
	localPath := filepath.Join(s.packageDir, fileName)

	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		return ErrPackageNotFound
	}

	return os.Remove(localPath)
}

// ==================== Package Download 安装包下载 ====================

// ErrDownloadInProgress indicates a download is already in progress for this version
// ErrDownloadInProgress 表示该版本的下载已在进行中
var ErrDownloadInProgress = errors.New("download already in progress / 下载已在进行中")

// ErrDownloadNotFound indicates the download task was not found
// ErrDownloadNotFound 表示下载任务未找到
var ErrDownloadNotFound = errors.New("download task not found / 下载任务未找到")

// StartDownload starts downloading a package from mirror to local storage.
// StartDownload 开始从镜像源下载安装包到本地存储。
func (s *Service) StartDownload(ctx context.Context, req *DownloadRequest) (*DownloadTask, error) {
	s.downloadsMu.Lock()
	defer s.downloadsMu.Unlock()

	// Check if download is already in progress / 检查是否已有下载正在进行
	if existing, ok := s.downloads[req.Version]; ok {
		if existing.Status == DownloadStatusDownloading || existing.Status == DownloadStatusPending {
			return existing, ErrDownloadInProgress
		}
	}

	// Use default mirror if not specified / 如果未指定则使用默认镜像源
	mirror := req.Mirror
	if mirror == "" {
		mirror = MirrorAliyun
	}

	// Get download URL / 获取下载 URL
	downloadURL := fmt.Sprintf("%s/%s/apache-seatunnel-%s-bin.tar.gz",
		MirrorURLs[mirror], req.Version, req.Version)

	// Create download task / 创建下载任务
	task := &DownloadTask{
		ID:          uuid.New().String(),
		Version:     req.Version,
		Mirror:      mirror,
		DownloadURL: downloadURL,
		Status:      DownloadStatusPending,
		Progress:    0,
		Message:     "准备下载 / Preparing download",
		StartTime:   time.Now(),
	}

	s.downloads[req.Version] = task

	// Start download in background / 在后台开始下载
	go s.runDownload(context.Background(), task)

	return task, nil
}

// GetDownloadStatus returns the current download status for a version.
// GetDownloadStatus 返回某版本的当前下载状态。
func (s *Service) GetDownloadStatus(ctx context.Context, version string) (*DownloadTask, error) {
	s.downloadsMu.RLock()
	defer s.downloadsMu.RUnlock()

	task, ok := s.downloads[version]
	if !ok {
		return nil, ErrDownloadNotFound
	}

	return task, nil
}

// CancelDownload cancels an ongoing download.
// CancelDownload 取消正在进行的下载。
func (s *Service) CancelDownload(ctx context.Context, version string) (*DownloadTask, error) {
	s.downloadsMu.Lock()
	defer s.downloadsMu.Unlock()

	task, ok := s.downloads[version]
	if !ok {
		return nil, ErrDownloadNotFound
	}

	if task.Status != DownloadStatusDownloading && task.Status != DownloadStatusPending {
		return task, nil // Already completed or failed / 已完成或失败
	}

	now := time.Now()
	task.Status = DownloadStatusCancelled
	task.Message = "下载已取消 / Download cancelled"
	task.EndTime = &now

	// Clean up temp file / 清理临时文件
	tempPath := filepath.Join(s.tempDir, fmt.Sprintf("apache-seatunnel-%s-bin.tar.gz.tmp", version))
	os.Remove(tempPath)

	return task, nil
}

// ListDownloads returns all download tasks.
// ListDownloads 返回所有下载任务。
func (s *Service) ListDownloads(ctx context.Context) []*DownloadTask {
	s.downloadsMu.RLock()
	defer s.downloadsMu.RUnlock()

	tasks := make([]*DownloadTask, 0, len(s.downloads))
	for _, task := range s.downloads {
		tasks = append(tasks, task)
	}
	return tasks
}

// runDownload executes the download process.
// runDownload 执行下载过程。
func (s *Service) runDownload(ctx context.Context, task *DownloadTask) {
	s.downloadsMu.Lock()
	task.Status = DownloadStatusDownloading
	task.Message = "正在下载 / Downloading"
	s.downloadsMu.Unlock()

	fileName := fmt.Sprintf("apache-seatunnel-%s-bin.tar.gz", task.Version)
	tempPath := filepath.Join(s.tempDir, fileName+".tmp")
	finalPath := filepath.Join(s.packageDir, fileName)

	// Create HTTP request / 创建 HTTP 请求
	resp, err := http.Get(task.DownloadURL)
	if err != nil {
		s.downloadsMu.Lock()
		now := time.Now()
		task.Status = DownloadStatusFailed
		task.Error = fmt.Sprintf("请求失败 / Request failed: %v", err)
		task.EndTime = &now
		s.downloadsMu.Unlock()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.downloadsMu.Lock()
		now := time.Now()
		task.Status = DownloadStatusFailed
		task.Error = fmt.Sprintf("HTTP 错误 / HTTP error: %d", resp.StatusCode)
		task.EndTime = &now
		s.downloadsMu.Unlock()
		return
	}

	// Get total size / 获取总大小
	s.downloadsMu.Lock()
	task.TotalBytes = resp.ContentLength
	s.downloadsMu.Unlock()

	// Create temp file / 创建临时文件
	out, err := os.Create(tempPath)
	if err != nil {
		s.downloadsMu.Lock()
		now := time.Now()
		task.Status = DownloadStatusFailed
		task.Error = fmt.Sprintf("创建文件失败 / Failed to create file: %v", err)
		task.EndTime = &now
		s.downloadsMu.Unlock()
		return
	}
	defer out.Close()

	// Download with progress tracking / 带进度跟踪的下载
	buf := make([]byte, 32*1024) // 32KB buffer
	var downloaded int64
	lastUpdate := time.Now()
	var lastDownloaded int64

	for {
		// Check if cancelled / 检查是否已取消
		s.downloadsMu.RLock()
		if task.Status == DownloadStatusCancelled {
			s.downloadsMu.RUnlock()
			out.Close()
			os.Remove(tempPath)
			return
		}
		s.downloadsMu.RUnlock()

		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				s.downloadsMu.Lock()
				now := time.Now()
				task.Status = DownloadStatusFailed
				task.Error = fmt.Sprintf("写入文件失败 / Failed to write file: %v", writeErr)
				task.EndTime = &now
				s.downloadsMu.Unlock()
				os.Remove(tempPath)
				return
			}
			downloaded += int64(n)

			// Update progress every 500ms / 每 500ms 更新一次进度
			if time.Since(lastUpdate) > 500*time.Millisecond {
				s.downloadsMu.Lock()
				task.DownloadedBytes = downloaded
				if task.TotalBytes > 0 {
					task.Progress = int(downloaded * 100 / task.TotalBytes)
				}
				// Calculate speed / 计算速度
				elapsed := time.Since(lastUpdate).Seconds()
				if elapsed > 0 {
					task.Speed = int64(float64(downloaded-lastDownloaded) / elapsed)
				}
				task.Message = fmt.Sprintf("正在下载 / Downloading: %d%%", task.Progress)
				s.downloadsMu.Unlock()

				lastUpdate = time.Now()
				lastDownloaded = downloaded
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			s.downloadsMu.Lock()
			now := time.Now()
			task.Status = DownloadStatusFailed
			task.Error = fmt.Sprintf("下载失败 / Download failed: %v", err)
			task.EndTime = &now
			s.downloadsMu.Unlock()
			os.Remove(tempPath)
			return
		}
	}

	// Close file before moving / 移动前关闭文件
	out.Close()

	// Move temp file to final location / 将临时文件移动到最终位置
	if err := os.Rename(tempPath, finalPath); err != nil {
		s.downloadsMu.Lock()
		now := time.Now()
		task.Status = DownloadStatusFailed
		task.Error = fmt.Sprintf("移动文件失败 / Failed to move file: %v", err)
		task.EndTime = &now
		s.downloadsMu.Unlock()
		os.Remove(tempPath)
		return
	}

	// Mark as completed / 标记为完成
	s.downloadsMu.Lock()
	now := time.Now()
	task.Status = DownloadStatusCompleted
	task.Progress = 100
	task.DownloadedBytes = downloaded
	task.Message = "下载完成 / Download completed"
	task.EndTime = &now
	s.downloadsMu.Unlock()
}

// ==================== Precheck 预检查 ====================

// RunPrecheck runs precheck on a host via Agent.
// RunPrecheck 通过 Agent 在主机上运行预检查。
func (s *Service) RunPrecheck(ctx context.Context, hostID uint, req *PrecheckRequest) (*PrecheckResult, error) {
	// TODO: Send precheck command to Agent via gRPC
	// TODO: 通过 gRPC 向 Agent 发送预检查命令

	// For now, return a mock result / 目前返回模拟结果
	return &PrecheckResult{
		Items: []PrecheckItem{
			{Name: "memory", Status: CheckStatusPassed, Message: "Memory check passed / 内存检查通过"},
			{Name: "cpu", Status: CheckStatusPassed, Message: "CPU check passed / CPU 检查通过"},
			{Name: "disk", Status: CheckStatusPassed, Message: "Disk check passed / 磁盘检查通过"},
			{Name: "ports", Status: CheckStatusPassed, Message: "Ports check passed / 端口检查通过"},
			{Name: "java", Status: CheckStatusPassed, Message: "Java check passed / Java 检查通过"},
		},
		OverallStatus: CheckStatusPassed,
		Summary:       "All checks passed / 所有检查通过",
	}, nil
}

// ==================== Installation 安装 ====================

// StartInstallation starts a new installation.
// StartInstallation 开始新的安装。
func (s *Service) StartInstallation(ctx context.Context, req *InstallationRequest) (*InstallationStatus, error) {
	s.installMu.Lock()
	defer s.installMu.Unlock()

	// Check if installation is already in progress / 检查是否已有安装正在进行
	if existing, ok := s.installations[req.HostID]; ok {
		if existing.Status == StepStatusRunning {
			return nil, ErrInstallationInProgress
		}
	}

	// Create new installation status / 创建新的安装状态
	status := &InstallationStatus{
		ID:          uuid.New().String(),
		HostID:      req.HostID,
		Status:      StepStatusRunning,
		CurrentStep: InstallStepDownload,
		Steps:       createInitialSteps(),
		Progress:    0,
		Message:     "Installation started / 安装已开始",
		StartTime:   time.Now(),
	}

	s.installations[req.HostID] = status

	// Start installation in background / 在后台开始安装
	go s.runInstallation(context.Background(), req, status)

	return status, nil
}

// GetInstallationStatus returns the current installation status.
// GetInstallationStatus 返回当前安装状态。
func (s *Service) GetInstallationStatus(ctx context.Context, hostID uint) (*InstallationStatus, error) {
	s.installMu.RLock()
	defer s.installMu.RUnlock()

	hostIDStr := fmt.Sprintf("%d", hostID)
	status, ok := s.installations[hostIDStr]
	if !ok {
		return nil, ErrInstallationNotFound
	}

	return status, nil
}

// RetryStep retries a failed installation step.
// RetryStep 重试失败的安装步骤。
func (s *Service) RetryStep(ctx context.Context, hostID uint, step string) (*InstallationStatus, error) {
	s.installMu.Lock()
	defer s.installMu.Unlock()

	hostIDStr := fmt.Sprintf("%d", hostID)
	status, ok := s.installations[hostIDStr]
	if !ok {
		return nil, ErrInstallationNotFound
	}

	// Find and reset the step / 找到并重置步骤
	for i := range status.Steps {
		if status.Steps[i].Name == step {
			status.Steps[i].Status = StepStatusPending
			status.Steps[i].Error = ""
			break
		}
	}

	status.Status = StepStatusRunning
	status.Error = ""

	// TODO: Resume installation from the failed step
	// TODO: 从失败的步骤恢复安装

	return status, nil
}

// CancelInstallation cancels an ongoing installation.
// CancelInstallation 取消正在进行的安装。
func (s *Service) CancelInstallation(ctx context.Context, hostID uint) (*InstallationStatus, error) {
	s.installMu.Lock()
	defer s.installMu.Unlock()

	hostIDStr := fmt.Sprintf("%d", hostID)
	status, ok := s.installations[hostIDStr]
	if !ok {
		return nil, ErrInstallationNotFound
	}

	// TODO: Send cancel command to Agent
	// TODO: 向 Agent 发送取消命令

	now := time.Now()
	status.Status = StepStatusFailed
	status.Message = "Installation cancelled / 安装已取消"
	status.EndTime = &now

	return status, nil
}

// runInstallation runs the installation process.
// runInstallation 运行安装过程。
func (s *Service) runInstallation(ctx context.Context, req *InstallationRequest, status *InstallationStatus) {
	// TODO: Implement actual installation via Agent gRPC
	// TODO: 通过 Agent gRPC 实现实际安装

	// Simulate installation steps / 模拟安装步骤
	steps := []InstallStep{
		InstallStepDownload,
		InstallStepVerify,
		InstallStepExtract,
		InstallStepConfigureCluster,
		InstallStepConfigureCheckpoint,
		InstallStepConfigureJVM,
		InstallStepInstallPlugins,
		InstallStepRegisterCluster,
		InstallStepComplete,
	}

	for i, step := range steps {
		s.installMu.Lock()
		status.CurrentStep = step
		status.Progress = (i * 100) / len(steps)

		// Update step status / 更新步骤状态
		for j := range status.Steps {
			if status.Steps[j].Step == step {
				now := time.Now()
				status.Steps[j].Status = StepStatusRunning
				status.Steps[j].StartTime = &now
				break
			}
		}
		s.installMu.Unlock()

		// Simulate step execution / 模拟步骤执行
		time.Sleep(500 * time.Millisecond)

		s.installMu.Lock()
		for j := range status.Steps {
			if status.Steps[j].Step == step {
				now := time.Now()
				status.Steps[j].Status = StepStatusSuccess
				status.Steps[j].Progress = 100
				status.Steps[j].EndTime = &now
				break
			}
		}
		s.installMu.Unlock()
	}

	// Mark installation as complete / 标记安装完成
	s.installMu.Lock()
	now := time.Now()
	status.Status = StepStatusSuccess
	status.Progress = 100
	status.Message = "Installation completed successfully / 安装成功完成"
	status.EndTime = &now
	s.installMu.Unlock()
}

// ==================== Helper Functions 辅助函数 ====================

// createInitialSteps creates the initial step list.
// createInitialSteps 创建初始步骤列表。
func createInitialSteps() []StepInfo {
	return []StepInfo{
		{Step: InstallStepDownload, Name: "download", Description: "Download package / 下载安装包", Status: StepStatusPending, Retryable: true},
		{Step: InstallStepVerify, Name: "verify", Description: "Verify checksum / 验证校验和", Status: StepStatusPending, Retryable: true},
		{Step: InstallStepExtract, Name: "extract", Description: "Extract package / 解压安装包", Status: StepStatusPending, Retryable: true},
		{Step: InstallStepConfigureCluster, Name: "configure_cluster", Description: "Configure cluster / 配置集群", Status: StepStatusPending, Retryable: true},
		{Step: InstallStepConfigureCheckpoint, Name: "configure_checkpoint", Description: "Configure checkpoint / 配置检查点", Status: StepStatusPending, Retryable: true},
		{Step: InstallStepConfigureJVM, Name: "configure_jvm", Description: "Configure JVM / 配置 JVM", Status: StepStatusPending, Retryable: true},
		{Step: InstallStepInstallPlugins, Name: "install_plugins", Description: "Install plugins / 安装插件", Status: StepStatusPending, Retryable: true},
		{Step: InstallStepRegisterCluster, Name: "register_cluster", Description: "Register to cluster / 注册到集群", Status: StepStatusPending, Retryable: true},
		{Step: InstallStepComplete, Name: "complete", Description: "Complete / 完成", Status: StepStatusPending, Retryable: false},
	}
}

// getDownloadURLs returns download URLs for a version.
// getDownloadURLs 返回某版本的下载 URL。
func getDownloadURLs(version string) map[MirrorSource]string {
	urls := make(map[MirrorSource]string)
	for mirror, baseURL := range MirrorURLs {
		urls[mirror] = fmt.Sprintf("%s/%s/apache-seatunnel-%s-bin.tar.gz", baseURL, version, version)
	}
	return urls
}

// isSeaTunnelPackage checks if a file name is a SeaTunnel package.
// isSeaTunnelPackage 检查文件名是否是 SeaTunnel 安装包。
func isSeaTunnelPackage(name string) bool {
	return len(name) > 20 && name[:17] == "apache-seatunnel-" && name[len(name)-11:] == "-bin.tar.gz"
}

// extractVersionFromFileName extracts version from package file name.
// extractVersionFromFileName 从安装包文件名中提取版本。
func extractVersionFromFileName(name string) string {
	// Format: apache-seatunnel-{version}-bin.tar.gz
	if len(name) < 29 {
		return ""
	}
	return name[17 : len(name)-11]
}

// calculateChecksum calculates SHA256 checksum of a file.
// calculateChecksum 计算文件的 SHA256 校验和。
func calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
