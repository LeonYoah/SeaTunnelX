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
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
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

// SupportedVersions lists supported SeaTunnel versions
// SupportedVersions 列出支持的 SeaTunnel 版本
var SupportedVersions = []string{
	"2.3.12",
	"2.3.11",
	"2.3.10",
	"2.3.9",
	"2.3.8",
}

// RecommendedVersion is the recommended SeaTunnel version
// RecommendedVersion 是推荐的 SeaTunnel 版本
const RecommendedVersion = "2.3.12"

// Service provides installation management functionality.
// Service 提供安装管理功能。
type Service struct {
	// packageDir is the directory for storing local packages
	// packageDir 是存储本地安装包的目录
	packageDir string

	// installations tracks ongoing installations by host ID
	// installations 按主机 ID 跟踪正在进行的安装
	installations map[string]*InstallationStatus
	installMu     sync.RWMutex

	// agentManager is used to communicate with agents
	// agentManager 用于与 Agent 通信
	// agentManager *agent.Manager // TODO: inject agent manager
}

// NewService creates a new Service instance.
// NewService 创建一个新的 Service 实例。
func NewService(packageDir string) *Service {
	// Create package directory if not exists / 如果不存在则创建安装包目录
	if err := os.MkdirAll(packageDir, 0755); err != nil {
		// Log error but continue / 记录错误但继续
	}

	return &Service{
		packageDir:    packageDir,
		installations: make(map[string]*InstallationStatus),
	}
}

// ==================== Package Management 安装包管理 ====================

// ListAvailableVersions returns available SeaTunnel versions.
// ListAvailableVersions 返回可用的 SeaTunnel 版本。
func (s *Service) ListAvailableVersions(ctx context.Context) (*AvailableVersions, error) {
	result := &AvailableVersions{
		Versions:           SupportedVersions,
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
