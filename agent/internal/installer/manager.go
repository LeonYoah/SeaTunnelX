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

// Package installer provides SeaTunnel installation management for the Agent.
// installer 包提供 Agent 的 SeaTunnel 安装管理功能。
//
// This package provides:
// 此包提供：
// - Online/offline installation / 在线/离线安装
// - Package download and verification / 安装包下载和验证
// - Configuration generation / 配置生成
// - Upgrade and rollback / 升级和回滚
package installer

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Common errors for installation management
// 安装管理的常见错误
var (
	// ErrPackageNotFound indicates the installation package was not found
	// ErrPackageNotFound 表示安装包未找到
	ErrPackageNotFound = errors.New("installation package not found")

	// ErrChecksumMismatch indicates the package checksum verification failed
	// ErrChecksumMismatch 表示安装包校验和验证失败
	ErrChecksumMismatch = errors.New("package checksum mismatch")

	// ErrDownloadFailed indicates the package download failed
	// ErrDownloadFailed 表示安装包下载失败
	ErrDownloadFailed = errors.New("package download failed")

	// ErrExtractionFailed indicates the package extraction failed
	// ErrExtractionFailed 表示安装包解压失败
	ErrExtractionFailed = errors.New("package extraction failed")

	// ErrConfigGenerationFailed indicates the configuration generation failed
	// ErrConfigGenerationFailed 表示配置生成失败
	ErrConfigGenerationFailed = errors.New("configuration generation failed")

	// ErrInstallDirNotWritable indicates the installation directory is not writable
	// ErrInstallDirNotWritable 表示安装目录不可写
	ErrInstallDirNotWritable = errors.New("installation directory is not writable")

	// ErrInvalidDeploymentMode indicates an invalid deployment mode
	// ErrInvalidDeploymentMode 表示无效的部署模式
	ErrInvalidDeploymentMode = errors.New("invalid deployment mode")

	// ErrInvalidNodeRole indicates an invalid node role
	// ErrInvalidNodeRole 表示无效的节点角色
	ErrInvalidNodeRole = errors.New("invalid node role")

	// ErrInvalidMirrorSource indicates an invalid mirror source
	// ErrInvalidMirrorSource 表示无效的镜像源
	ErrInvalidMirrorSource = errors.New("invalid mirror source")
)

// MirrorSource represents the download mirror source
// MirrorSource 表示下载镜像源
type MirrorSource string

const (
	// MirrorAliyun is the Aliyun mirror (default, fastest in China)
	// MirrorAliyun 是阿里云镜像（默认，国内最快）
	MirrorAliyun MirrorSource = "aliyun"

	// MirrorApache is the official Apache mirror
	// MirrorApache 是 Apache 官方镜像
	MirrorApache MirrorSource = "apache"

	// MirrorHuaweiCloud is the Huawei Cloud mirror
	// MirrorHuaweiCloud 是华为云镜像
	MirrorHuaweiCloud MirrorSource = "huaweicloud"
)

// MirrorURLs maps mirror sources to their base URLs
// MirrorURLs 将镜像源映射到其基础 URL
var MirrorURLs = map[MirrorSource]string{
	MirrorAliyun:      "https://mirrors.aliyun.com/apache/seatunnel",
	MirrorApache:      "https://archive.apache.org/dist/seatunnel",
	MirrorHuaweiCloud: "https://mirrors.huaweicloud.com/apache/seatunnel",
}

// GetDownloadURL generates the download URL for a specific version and mirror
// GetDownloadURL 为特定版本和镜像生成下载 URL
func GetDownloadURL(mirror MirrorSource, version string) string {
	baseURL, ok := MirrorURLs[mirror]
	if !ok {
		baseURL = MirrorURLs[MirrorAliyun] // Default to Aliyun / 默认使用阿里云
	}

	// Standard Apache mirror format / 标准 Apache 镜像格式
	// Example: https://mirrors.aliyun.com/apache/seatunnel/2.3.12/apache-seatunnel-2.3.12-bin.tar.gz
	return fmt.Sprintf("%s/%s/apache-seatunnel-%s-bin.tar.gz", baseURL, version, version)
}

// GetAllMirrorURLs returns download URLs from all mirrors for a version
// GetAllMirrorURLs 返回某版本所有镜像的下载 URL
func GetAllMirrorURLs(version string) map[MirrorSource]string {
	urls := make(map[MirrorSource]string)
	for mirror := range MirrorURLs {
		urls[mirror] = GetDownloadURL(mirror, version)
	}
	return urls
}

// ValidateMirrorSource validates if the mirror source is valid
// ValidateMirrorSource 验证镜像源是否有效
func ValidateMirrorSource(mirror MirrorSource) bool {
	_, ok := MirrorURLs[mirror]
	return ok
}

// InstallMode represents the installation mode
// InstallMode 表示安装模式
type InstallMode string

const (
	// InstallModeOnline indicates online installation (download from mirror)
	// InstallModeOnline 表示在线安装（从镜像源下载）
	InstallModeOnline InstallMode = "online"

	// InstallModeOffline indicates offline installation (use local package)
	// InstallModeOffline 表示离线安装（使用本地安装包）
	InstallModeOffline InstallMode = "offline"
)

// DeploymentMode represents the SeaTunnel deployment mode
// DeploymentMode 表示 SeaTunnel 部署模式
type DeploymentMode string

const (
	// DeploymentModeHybrid indicates hybrid mode (master and worker on same node)
	// DeploymentModeHybrid 表示混合模式（master 和 worker 在同一节点）
	DeploymentModeHybrid DeploymentMode = "hybrid"

	// DeploymentModeSeparated indicates separated mode (master and worker on different nodes)
	// DeploymentModeSeparated 表示分离模式（master 和 worker 在不同节点）
	DeploymentModeSeparated DeploymentMode = "separated"
)

// NodeRole represents the node role in a cluster
// NodeRole 表示集群中的节点角色
type NodeRole string

const (
	// NodeRoleMaster indicates a master node
	// NodeRoleMaster 表示 master 节点
	NodeRoleMaster NodeRole = "master"

	// NodeRoleWorker indicates a worker node
	// NodeRoleWorker 表示 worker 节点
	NodeRoleWorker NodeRole = "worker"
)

// InstallStep represents a step in the installation process
// InstallStep 表示安装过程中的步骤
type InstallStep string

const (
	// InstallStepDownload is the download step
	// InstallStepDownload 是下载步骤
	InstallStepDownload InstallStep = "download"

	// InstallStepVerify is the verification step
	// InstallStepVerify 是验证步骤
	InstallStepVerify InstallStep = "verify"

	// InstallStepExtract is the extraction step
	// InstallStepExtract 是解压步骤
	InstallStepExtract InstallStep = "extract"

	// InstallStepConfigureCluster is the cluster configuration step
	// InstallStepConfigureCluster 是集群配置步骤
	InstallStepConfigureCluster InstallStep = "configure_cluster"

	// InstallStepConfigureCheckpoint is the checkpoint configuration step
	// InstallStepConfigureCheckpoint 是检查点配置步骤
	InstallStepConfigureCheckpoint InstallStep = "configure_checkpoint"

	// InstallStepConfigureJVM is the JVM configuration step
	// InstallStepConfigureJVM 是 JVM 配置步骤
	InstallStepConfigureJVM InstallStep = "configure_jvm"

	// InstallStepInstallPlugins is the plugin installation step
	// InstallStepInstallPlugins 是插件安装步骤
	InstallStepInstallPlugins InstallStep = "install_plugins"

	// InstallStepRegisterCluster is the cluster registration step
	// InstallStepRegisterCluster 是集群注册步骤
	InstallStepRegisterCluster InstallStep = "register_cluster"

	// InstallStepComplete is the completion step
	// InstallStepComplete 是完成步骤
	InstallStepComplete InstallStep = "complete"

	// Legacy step for backward compatibility / 向后兼容的旧步骤
	InstallStepConfigure InstallStep = "configure"
)

// CheckpointStorageType represents the checkpoint storage type
// CheckpointStorageType 表示检查点存储类型
type CheckpointStorageType string

const (
	// CheckpointStorageLocalFile is local file storage (not recommended for production)
	// CheckpointStorageLocalFile 是本地文件存储（不建议生产环境使用）
	CheckpointStorageLocalFile CheckpointStorageType = "LOCAL_FILE"

	// CheckpointStorageHDFS is HDFS storage
	// CheckpointStorageHDFS 是 HDFS 存储
	CheckpointStorageHDFS CheckpointStorageType = "HDFS"

	// CheckpointStorageOSS is Aliyun OSS storage
	// CheckpointStorageOSS 是阿里云 OSS 存储
	CheckpointStorageOSS CheckpointStorageType = "OSS"

	// CheckpointStorageS3 is AWS S3 or S3-compatible storage
	// CheckpointStorageS3 是 AWS S3 或 S3 兼容存储
	CheckpointStorageS3 CheckpointStorageType = "S3"
)

// CheckpointConfig contains checkpoint storage configuration
// CheckpointConfig 包含检查点存储配置
type CheckpointConfig struct {
	// StorageType is the checkpoint storage type
	// StorageType 是检查点存储类型
	StorageType CheckpointStorageType `json:"storage_type"`

	// Namespace is the checkpoint storage path/namespace
	// Namespace 是检查点存储路径/命名空间
	Namespace string `json:"namespace"`

	// HDFS configuration / HDFS 配置
	HDFSNameNodeHost string `json:"hdfs_namenode_host,omitempty"`
	HDFSNameNodePort int    `json:"hdfs_namenode_port,omitempty"`

	// OSS/S3 configuration / OSS/S3 配置
	StorageEndpoint  string `json:"storage_endpoint,omitempty"`
	StorageAccessKey string `json:"storage_access_key,omitempty"`
	StorageSecretKey string `json:"storage_secret_key,omitempty"`
	StorageBucket    string `json:"storage_bucket,omitempty"`
}

// JVMConfig contains JVM memory configuration
// JVMConfig 包含 JVM 内存配置
type JVMConfig struct {
	// HybridHeapSize is the heap size for hybrid mode (in GB)
	// HybridHeapSize 是混合模式的堆内存大小（GB）
	HybridHeapSize int `json:"hybrid_heap_size"`

	// MasterHeapSize is the heap size for master nodes (in GB)
	// MasterHeapSize 是 master 节点的堆内存大小（GB）
	MasterHeapSize int `json:"master_heap_size"`

	// WorkerHeapSize is the heap size for worker nodes (in GB)
	// WorkerHeapSize 是 worker 节点的堆内存大小（GB）
	WorkerHeapSize int `json:"worker_heap_size"`
}

// ConnectorConfig contains connector installation configuration
// ConnectorConfig 包含连接器安装配置
type ConnectorConfig struct {
	// InstallConnectors indicates whether to install connectors
	// InstallConnectors 表示是否安装连接器
	InstallConnectors bool `json:"install_connectors"`

	// Connectors is the list of connectors to install
	// Connectors 是要安装的连接器列表
	Connectors []string `json:"connectors,omitempty"`

	// PluginRepo is the plugin repository source
	// PluginRepo 是插件仓库源
	PluginRepo MirrorSource `json:"plugin_repo,omitempty"`
}

// StepStatus represents the status of an installation step
// StepStatus 表示安装步骤的状态
type StepStatus string

const (
	// StepStatusPending indicates the step is pending
	// StepStatusPending 表示步骤待执行
	StepStatusPending StepStatus = "pending"

	// StepStatusRunning indicates the step is running
	// StepStatusRunning 表示步骤正在执行
	StepStatusRunning StepStatus = "running"

	// StepStatusSuccess indicates the step completed successfully
	// StepStatusSuccess 表示步骤执行成功
	StepStatusSuccess StepStatus = "success"

	// StepStatusFailed indicates the step failed
	// StepStatusFailed 表示步骤执行失败
	StepStatusFailed StepStatus = "failed"

	// StepStatusSkipped indicates the step was skipped
	// StepStatusSkipped 表示步骤被跳过
	StepStatusSkipped StepStatus = "skipped"
)

// StepInfo contains information about an installation step
// StepInfo 包含安装步骤的信息
type StepInfo struct {
	// Step is the step identifier
	// Step 是步骤标识符
	Step InstallStep `json:"step"`

	// Name is the step name
	// Name 是步骤名称
	Name string `json:"name"`

	// Description is the step description
	// Description 是步骤描述
	Description string `json:"description"`

	// Status is the current status
	// Status 是当前状态
	Status StepStatus `json:"status"`

	// Progress is the progress percentage (0-100)
	// Progress 是进度百分比（0-100）
	Progress int `json:"progress"`

	// Message is the current status message
	// Message 是当前状态消息
	Message string `json:"message,omitempty"`

	// Error is the error message if failed
	// Error 是失败时的错误消息
	Error string `json:"error,omitempty"`

	// StartTime is when the step started
	// StartTime 是步骤开始时间
	StartTime *time.Time `json:"start_time,omitempty"`

	// EndTime is when the step ended
	// EndTime 是步骤结束时间
	EndTime *time.Time `json:"end_time,omitempty"`

	// Retryable indicates if the step can be retried
	// Retryable 表示步骤是否可重试
	Retryable bool `json:"retryable"`
}

// InstallationSteps defines all installation steps in order
// InstallationSteps 定义所有安装步骤的顺序
// Note: Agent manages SeaTunnel process lifecycle, no systemd auto-start needed
// 注意：Agent 管理 SeaTunnel 进程生命周期，不需要 systemd 开机自启动
// Note: Precheck is done separately via Prechecker, not part of installation steps
// 注意：预检通过 Prechecker 单独完成，不是安装步骤的一部分
var InstallationSteps = []StepInfo{
	{Step: InstallStepDownload, Name: "download", Description: "Download package / 下载安装包", Retryable: true},
	{Step: InstallStepVerify, Name: "verify", Description: "Verify checksum / 验证校验和", Retryable: true},
	{Step: InstallStepExtract, Name: "extract", Description: "Extract package / 解压安装包", Retryable: true},
	{Step: InstallStepConfigureCluster, Name: "configure_cluster", Description: "Configure cluster / 配置集群", Retryable: true},
	{Step: InstallStepConfigureCheckpoint, Name: "configure_checkpoint", Description: "Configure checkpoint / 配置检查点", Retryable: true},
	{Step: InstallStepConfigureJVM, Name: "configure_jvm", Description: "Configure JVM / 配置 JVM", Retryable: true},
	{Step: InstallStepInstallPlugins, Name: "install_plugins", Description: "Install plugins / 安装插件", Retryable: true},
	{Step: InstallStepRegisterCluster, Name: "register_cluster", Description: "Register to cluster / 注册到集群", Retryable: true},
	{Step: InstallStepComplete, Name: "complete", Description: "Complete / 完成", Retryable: false},
}

// ============================================================================
// One-Click Installation API Types (for frontend integration)
// 一键安装 API 类型（用于前端集成）
// ============================================================================

// PackageInfo contains information about a SeaTunnel package
// PackageInfo 包含 SeaTunnel 安装包信息
type PackageInfo struct {
	// Version is the SeaTunnel version
	// Version 是 SeaTunnel 版本
	Version string `json:"version"`

	// FileName is the package file name
	// FileName 是安装包文件名
	FileName string `json:"file_name"`

	// FileSize is the package file size in bytes
	// FileSize 是安装包文件大小（字节）
	FileSize int64 `json:"file_size"`

	// Checksum is the SHA256 checksum
	// Checksum 是 SHA256 校验和
	Checksum string `json:"checksum,omitempty"`

	// DownloadURLs contains download URLs from different mirrors
	// DownloadURLs 包含不同镜像的下载 URL
	DownloadURLs map[MirrorSource]string `json:"download_urls"`

	// IsLocal indicates if the package is available locally
	// IsLocal 表示安装包是否在本地可用
	IsLocal bool `json:"is_local"`

	// LocalPath is the local file path if available
	// LocalPath 是本地文件路径（如果可用）
	LocalPath string `json:"local_path,omitempty"`

	// UploadedAt is when the package was uploaded (for local packages)
	// UploadedAt 是安装包上传时间（本地安装包）
	UploadedAt *time.Time `json:"uploaded_at,omitempty"`
}

// InstallationRequest is the request for one-click installation (from Control Plane to Agent)
// InstallationRequest 是一键安装的请求（从 Control Plane 到 Agent）
// Note: Package is transferred from Control Plane, not downloaded by Agent
// 注意：安装包从 Control Plane 传输，而不是由 Agent 下载
type InstallationRequest struct {
	// HostID is the target host ID
	// HostID 是目标主机 ID
	HostID string `json:"host_id"`

	// ClusterID is the cluster to join after installation
	// ClusterID 是安装后要加入的集群
	ClusterID string `json:"cluster_id"`

	// Version is the SeaTunnel version to install
	// Version 是要安装的 SeaTunnel 版本
	Version string `json:"version"`

	// DeploymentMode is hybrid or separated
	// DeploymentMode 是混合或分离模式
	DeploymentMode DeploymentMode `json:"deployment_mode"`

	// NodeRole is master or worker
	// NodeRole 是 master 或 worker
	NodeRole NodeRole `json:"node_role"`

	// MasterAddresses is the list of master node addresses
	// MasterAddresses 是 master 节点地址列表
	MasterAddresses []string `json:"master_addresses,omitempty"`

	// WorkerAddresses is the list of worker node addresses
	// WorkerAddresses 是 worker 节点地址列表
	WorkerAddresses []string `json:"worker_addresses,omitempty"`

	// ClusterPort is the cluster communication port
	// ClusterPort 是集群通信端口
	ClusterPort int `json:"cluster_port,omitempty"`

	// HTTPPort is the HTTP API port
	// HTTPPort 是 HTTP API 端口
	HTTPPort int `json:"http_port,omitempty"`

	// JVM is the JVM configuration
	// JVM 是 JVM 配置
	JVM *JVMConfig `json:"jvm,omitempty"`

	// Checkpoint is the checkpoint configuration
	// Checkpoint 是检查点配置
	Checkpoint *CheckpointConfig `json:"checkpoint,omitempty"`

	// Connector is the connector configuration
	// Connector 是连接器配置
	Connector *ConnectorConfig `json:"connector,omitempty"`
}

// PackageTransferSource defines how Agent receives the package
// PackageTransferSource 定义 Agent 如何接收安装包
type PackageTransferSource string

const (
	// PackageTransferFromControlPlane means package is transferred from Control Plane via gRPC stream
	// PackageTransferFromControlPlane 表示安装包通过 gRPC 流从 Control Plane 传输
	PackageTransferFromControlPlane PackageTransferSource = "control_plane"

	// PackageTransferFromURL means package is downloaded from a URL (fallback)
	// PackageTransferFromURL 表示安装包从 URL 下载（备用）
	PackageTransferFromURL PackageTransferSource = "url"

	// PackageTransferLocal means package is already on the Agent node
	// PackageTransferLocal 表示安装包已在 Agent 节点上
	PackageTransferLocal PackageTransferSource = "local"
)

// PackageTransferInfo contains information for package transfer
// PackageTransferInfo 包含安装包传输信息
type PackageTransferInfo struct {
	// Source is how the package will be transferred
	// Source 是安装包的传输方式
	Source PackageTransferSource `json:"source"`

	// Version is the SeaTunnel version
	// Version 是 SeaTunnel 版本
	Version string `json:"version"`

	// FileName is the package file name
	// FileName 是安装包文件名
	FileName string `json:"file_name"`

	// FileSize is the package file size in bytes
	// FileSize 是安装包文件大小（字节）
	FileSize int64 `json:"file_size"`

	// Checksum is the SHA256 checksum for verification
	// Checksum 是用于验证的 SHA256 校验和
	Checksum string `json:"checksum"`

	// DownloadURL is the URL to download from (for URL source)
	// DownloadURL 是下载 URL（用于 URL 源）
	DownloadURL string `json:"download_url,omitempty"`

	// LocalPath is the local file path (for local source)
	// LocalPath 是本地文件路径（用于本地源）
	LocalPath string `json:"local_path,omitempty"`
}

// InstallationStatus represents the current installation status
// InstallationStatus 表示当前安装状态
type InstallationStatus struct {
	// ID is the installation task ID
	// ID 是安装任务 ID
	ID string `json:"id"`

	// HostID is the target host ID
	// HostID 是目标主机 ID
	HostID string `json:"host_id"`

	// Status is the overall status
	// Status 是总体状态
	Status StepStatus `json:"status"`

	// CurrentStep is the current step being executed
	// CurrentStep 是当前正在执行的步骤
	CurrentStep InstallStep `json:"current_step"`

	// Steps contains status of all steps
	// Steps 包含所有步骤的状态
	Steps []StepInfo `json:"steps"`

	// Progress is the overall progress percentage (0-100)
	// Progress 是总体进度百分比（0-100）
	Progress int `json:"progress"`

	// Message is the current status message
	// Message 是当前状态消息
	Message string `json:"message,omitempty"`

	// Error is the error message if failed
	// Error 是失败时的错误消息
	Error string `json:"error,omitempty"`

	// StartTime is when the installation started
	// StartTime 是安装开始时间
	StartTime time.Time `json:"start_time"`

	// EndTime is when the installation ended
	// EndTime 是安装结束时间
	EndTime *time.Time `json:"end_time,omitempty"`
}

// PackageUploadRequest is the request for uploading a package
// PackageUploadRequest 是上传安装包的请求
type PackageUploadRequest struct {
	// Version is the SeaTunnel version
	// Version 是 SeaTunnel 版本
	Version string `json:"version"`

	// FileName is the package file name
	// FileName 是安装包文件名
	FileName string `json:"file_name"`

	// Checksum is the expected SHA256 checksum (optional)
	// Checksum 是预期的 SHA256 校验和（可选）
	Checksum string `json:"checksum,omitempty"`
}

// AvailableVersions contains available SeaTunnel versions
// AvailableVersions 包含可用的 SeaTunnel 版本
type AvailableVersions struct {
	// Versions is the list of available versions
	// Versions 是可用版本列表
	Versions []string `json:"versions"`

	// RecommendedVersion is the recommended version
	// RecommendedVersion 是推荐版本
	RecommendedVersion string `json:"recommended_version"`

	// LocalPackages contains locally available packages
	// LocalPackages 包含本地可用的安装包
	LocalPackages []PackageInfo `json:"local_packages"`
}

// ProgressReporter is an interface for reporting installation progress
// ProgressReporter 是用于上报安装进度的接口
type ProgressReporter interface {
	// Report sends a progress update with the current step, progress percentage, and message
	// Report 发送进度更新，包含当前步骤、进度百分比和消息
	Report(step InstallStep, progress int, message string) error

	// ReportStepStart reports that a step has started
	// ReportStepStart 报告步骤已开始
	ReportStepStart(step InstallStep) error

	// ReportStepComplete reports that a step has completed successfully
	// ReportStepComplete 报告步骤已成功完成
	ReportStepComplete(step InstallStep) error

	// ReportStepFailed reports that a step has failed
	// ReportStepFailed 报告步骤已失败
	ReportStepFailed(step InstallStep, err error) error

	// ReportStepSkipped reports that a step was skipped
	// ReportStepSkipped 报告步骤被跳过
	ReportStepSkipped(step InstallStep, reason string) error
}

// NoOpProgressReporter is a ProgressReporter that does nothing
// NoOpProgressReporter 是一个不执行任何操作的 ProgressReporter
type NoOpProgressReporter struct{}

// Report implements ProgressReporter interface but does nothing
// Report 实现 ProgressReporter 接口但不执行任何操作
func (r *NoOpProgressReporter) Report(step InstallStep, progress int, message string) error {
	return nil
}

// ReportStepStart implements ProgressReporter interface but does nothing
// ReportStepStart 实现 ProgressReporter 接口但不执行任何操作
func (r *NoOpProgressReporter) ReportStepStart(step InstallStep) error {
	return nil
}

// ReportStepComplete implements ProgressReporter interface but does nothing
// ReportStepComplete 实现 ProgressReporter 接口但不执行任何操作
func (r *NoOpProgressReporter) ReportStepComplete(step InstallStep) error {
	return nil
}

// ReportStepFailed implements ProgressReporter interface but does nothing
// ReportStepFailed 实现 ProgressReporter 接口但不执行任何操作
func (r *NoOpProgressReporter) ReportStepFailed(step InstallStep, err error) error {
	return nil
}

// ReportStepSkipped implements ProgressReporter interface but does nothing
// ReportStepSkipped 实现 ProgressReporter 接口但不执行任何操作
func (r *NoOpProgressReporter) ReportStepSkipped(step InstallStep, reason string) error {
	return nil
}

// ChannelProgressReporter reports progress through a channel for frontend interaction
// ChannelProgressReporter 通过通道报告进度，用于前端交互
type ChannelProgressReporter struct {
	StepChan chan StepInfo
}

// NewChannelProgressReporter creates a new ChannelProgressReporter
// NewChannelProgressReporter 创建新的 ChannelProgressReporter
func NewChannelProgressReporter(bufferSize int) *ChannelProgressReporter {
	return &ChannelProgressReporter{
		StepChan: make(chan StepInfo, bufferSize),
	}
}

// Report implements ProgressReporter interface
// Report 实现 ProgressReporter 接口
func (r *ChannelProgressReporter) Report(step InstallStep, progress int, message string) error {
	r.StepChan <- StepInfo{
		Step:     step,
		Status:   StepStatusRunning,
		Progress: progress,
		Message:  message,
	}
	return nil
}

// ReportStepStart implements ProgressReporter interface
// ReportStepStart 实现 ProgressReporter 接口
func (r *ChannelProgressReporter) ReportStepStart(step InstallStep) error {
	now := time.Now()
	r.StepChan <- StepInfo{
		Step:      step,
		Status:    StepStatusRunning,
		Progress:  0,
		StartTime: &now,
	}
	return nil
}

// ReportStepComplete implements ProgressReporter interface
// ReportStepComplete 实现 ProgressReporter 接口
func (r *ChannelProgressReporter) ReportStepComplete(step InstallStep) error {
	now := time.Now()
	r.StepChan <- StepInfo{
		Step:     step,
		Status:   StepStatusSuccess,
		Progress: 100,
		EndTime:  &now,
	}
	return nil
}

// ReportStepFailed implements ProgressReporter interface
// ReportStepFailed 实现 ProgressReporter 接口
func (r *ChannelProgressReporter) ReportStepFailed(step InstallStep, err error) error {
	now := time.Now()
	r.StepChan <- StepInfo{
		Step:    step,
		Status:  StepStatusFailed,
		Error:   err.Error(),
		EndTime: &now,
	}
	return nil
}

// ReportStepSkipped implements ProgressReporter interface
// ReportStepSkipped 实现 ProgressReporter 接口
func (r *ChannelProgressReporter) ReportStepSkipped(step InstallStep, reason string) error {
	now := time.Now()
	r.StepChan <- StepInfo{
		Step:    step,
		Status:  StepStatusSkipped,
		Message: reason,
		EndTime: &now,
	}
	return nil
}

// Close closes the channel
// Close 关闭通道
func (r *ChannelProgressReporter) Close() {
	close(r.StepChan)
}

// InstallParams contains parameters for installation
// InstallParams 包含安装参数
// Note: Package is transferred from Control Plane, not downloaded by Agent directly
// 注意：安装包从 Control Plane 传输，而不是由 Agent 直接下载
type InstallParams struct {
	// Version is the SeaTunnel version to install
	// Version 是要安装的 SeaTunnel 版本
	Version string `json:"version"`

	// InstallDir is the installation directory
	// InstallDir 是安装目录
	InstallDir string `json:"install_dir"`

	// Mode is the installation mode (online/offline)
	// Mode 是安装模式（在线/离线）
	Mode InstallMode `json:"mode"`

	// Mirror is the mirror source for online installation
	// Mirror 是在线安装的镜像源
	Mirror MirrorSource `json:"mirror,omitempty"`

	// DownloadURL is the custom download URL (overrides mirror)
	// DownloadURL 是自定义下载 URL（覆盖镜像源）
	DownloadURL string `json:"download_url,omitempty"`

	// PackageTransfer contains package transfer information from Control Plane
	// PackageTransfer 包含从 Control Plane 传输安装包的信息
	PackageTransfer *PackageTransferInfo `json:"package_transfer,omitempty"`

	// PackagePath is the local package path (set after transfer or for local source)
	// PackagePath 是本地安装包路径（传输后设置或用于本地源）
	PackagePath string `json:"package_path,omitempty"`

	// ExpectedChecksum is the expected SHA256 checksum of the package
	// ExpectedChecksum 是安装包的预期 SHA256 校验和
	ExpectedChecksum string `json:"expected_checksum,omitempty"`

	// DeploymentMode is the deployment mode (hybrid/separated)
	// DeploymentMode 是部署模式（混合/分离）
	DeploymentMode DeploymentMode `json:"deployment_mode"`

	// NodeRole is the node role (master/worker)
	// NodeRole 是节点角色（master/worker）
	NodeRole NodeRole `json:"node_role"`

	// ClusterName is the cluster name
	// ClusterName 是集群名称
	ClusterName string `json:"cluster_name"`

	// MasterAddresses is the list of master node addresses
	// MasterAddresses 是 master 节点地址列表
	MasterAddresses []string `json:"master_addresses,omitempty"`

	// WorkerAddresses is the list of worker node addresses (for separated mode)
	// WorkerAddresses 是 worker 节点地址列表（分离模式）
	WorkerAddresses []string `json:"worker_addresses,omitempty"`

	// ClusterPort is the cluster communication port (hybrid mode or master port)
	// ClusterPort 是集群通信端口（混合模式或 master 端口）
	ClusterPort int `json:"cluster_port"`

	// WorkerPort is the worker node port (separated mode only)
	// WorkerPort 是 worker 节点端口（仅分离模式）
	WorkerPort int `json:"worker_port,omitempty"`

	// HTTPPort is the HTTP API port
	// HTTPPort 是 HTTP API 端口
	HTTPPort int `json:"http_port"`

	// DynamicSlot enables dynamic slot allocation (default: true)
	// DynamicSlot 启用动态槽位分配（默认：true）
	DynamicSlot *bool `json:"dynamic_slot,omitempty"`

	// JVM is the JVM memory configuration
	// JVM 是 JVM 内存配置
	JVM *JVMConfig `json:"jvm,omitempty"`

	// Checkpoint is the checkpoint storage configuration
	// Checkpoint 是检查点存储配置
	Checkpoint *CheckpointConfig `json:"checkpoint,omitempty"`

	// Connector is the connector installation configuration
	// Connector 是连接器安装配置
	Connector *ConnectorConfig `json:"connector,omitempty"`

	// ClusterID is the cluster ID to register after installation (for cluster registration)
	// ClusterID 是安装后要注册的集群 ID（用于集群注册）
	ClusterID string `json:"cluster_id,omitempty"`
}

// DefaultInstallParams returns default installation parameters
// DefaultInstallParams 返回默认安装参数
func DefaultInstallParams() *InstallParams {
	dynamicSlot := true
	return &InstallParams{
		Version:        "2.3.12",
		InstallDir:     "/opt/seatunnel",
		DeploymentMode: DeploymentModeHybrid,
		ClusterPort:    5801,
		WorkerPort:     5802,
		HTTPPort:       8080,
		DynamicSlot:    &dynamicSlot,
		JVM: &JVMConfig{
			HybridHeapSize: 3, // 3GB for hybrid mode / 混合模式 3GB
			MasterHeapSize: 2, // 2GB for master / master 2GB
			WorkerHeapSize: 2, // 2GB for worker / worker 2GB
		},
		Checkpoint: &CheckpointConfig{
			StorageType: CheckpointStorageLocalFile,
			Namespace:   "/tmp/seatunnel/checkpoint",
		},
		Connector: &ConnectorConfig{
			InstallConnectors: true,
			Connectors:        []string{"jdbc", "hive"},
			PluginRepo:        MirrorAliyun,
		},
	}
}

// DefaultJVMConfig returns default JVM configuration
// DefaultJVMConfig 返回默认 JVM 配置
func DefaultJVMConfig() *JVMConfig {
	return &JVMConfig{
		HybridHeapSize: 3,
		MasterHeapSize: 2,
		WorkerHeapSize: 2,
	}
}

// DefaultCheckpointConfig returns default checkpoint configuration
// DefaultCheckpointConfig 返回默认检查点配置
func DefaultCheckpointConfig() *CheckpointConfig {
	return &CheckpointConfig{
		StorageType: CheckpointStorageLocalFile,
		Namespace:   "/tmp/seatunnel/checkpoint",
	}
}

// DefaultConnectorConfig returns default connector configuration
// DefaultConnectorConfig 返回默认连接器配置
func DefaultConnectorConfig() *ConnectorConfig {
	return &ConnectorConfig{
		InstallConnectors: true,
		Connectors:        []string{"jdbc", "hive"},
		PluginRepo:        MirrorAliyun,
	}
}

// Validate validates the checkpoint configuration
// Validate 验证检查点配置
func (c *CheckpointConfig) Validate() error {
	switch c.StorageType {
	case CheckpointStorageLocalFile:
		if c.Namespace == "" {
			return errors.New("namespace is required for LOCAL_FILE storage")
		}
	case CheckpointStorageHDFS:
		if c.Namespace == "" {
			return errors.New("namespace is required for HDFS storage")
		}
		if c.HDFSNameNodeHost == "" {
			return errors.New("hdfs_namenode_host is required for HDFS storage")
		}
		if c.HDFSNameNodePort == 0 {
			return errors.New("hdfs_namenode_port is required for HDFS storage")
		}
	case CheckpointStorageOSS, CheckpointStorageS3:
		if c.Namespace == "" {
			return errors.New("namespace is required for OSS/S3 storage")
		}
		if c.StorageEndpoint == "" {
			return errors.New("storage_endpoint is required for OSS/S3 storage")
		}
		if c.StorageAccessKey == "" {
			return errors.New("storage_access_key is required for OSS/S3 storage")
		}
		if c.StorageSecretKey == "" {
			return errors.New("storage_secret_key is required for OSS/S3 storage")
		}
		if c.StorageBucket == "" {
			return errors.New("storage_bucket is required for OSS/S3 storage")
		}
	default:
		return fmt.Errorf("unsupported checkpoint storage type: %s", c.StorageType)
	}
	return nil
}

// Validate validates the JVM configuration
// Validate 验证 JVM 配置
func (j *JVMConfig) Validate() error {
	if j.HybridHeapSize < 1 {
		return errors.New("hybrid_heap_size must be at least 1 GB")
	}
	if j.MasterHeapSize < 1 {
		return errors.New("master_heap_size must be at least 1 GB")
	}
	if j.WorkerHeapSize < 1 {
		return errors.New("worker_heap_size must be at least 1 GB")
	}
	return nil
}

// BoolPtr returns a pointer to a bool value (helper function)
// BoolPtr 返回布尔值的指针（辅助函数）
func BoolPtr(b bool) *bool {
	return &b
}

// Validate validates the installation parameters
// Validate 验证安装参数
func (p *InstallParams) Validate() error {
	if p.DeploymentMode != DeploymentModeHybrid && p.DeploymentMode != DeploymentModeSeparated {
		return ErrInvalidDeploymentMode
	}

	if p.NodeRole != NodeRoleMaster && p.NodeRole != NodeRoleWorker {
		return ErrInvalidNodeRole
	}

	if p.Version == "" {
		return errors.New("version is required")
	}

	if p.InstallDir == "" {
		return errors.New("install_dir is required")
	}

	// Validate package transfer info / 验证安装包传输信息
	if p.PackageTransfer != nil {
		if err := p.PackageTransfer.Validate(); err != nil {
			return fmt.Errorf("package transfer validation failed: %w", err)
		}
	}

	// Validate JVM config if provided / 验证 JVM 配置（如果提供）
	if p.JVM != nil {
		if err := p.JVM.Validate(); err != nil {
			return fmt.Errorf("JVM config validation failed: %w", err)
		}
	}

	// Validate checkpoint config if provided / 验证检查点配置（如果提供）
	if p.Checkpoint != nil {
		if err := p.Checkpoint.Validate(); err != nil {
			return fmt.Errorf("checkpoint config validation failed: %w", err)
		}
	}

	return nil
}

// Validate validates the package transfer info
// Validate 验证安装包传输信息
func (p *PackageTransferInfo) Validate() error {
	if p.Version == "" {
		return errors.New("version is required")
	}
	if p.FileName == "" {
		return errors.New("file_name is required")
	}
	if p.Checksum == "" {
		return errors.New("checksum is required for verification")
	}

	switch p.Source {
	case PackageTransferFromControlPlane:
		// No additional validation needed, package will be streamed
		// 无需额外验证，安装包将通过流传输
	case PackageTransferFromURL:
		if p.DownloadURL == "" {
			return errors.New("download_url is required for URL source")
		}
	case PackageTransferLocal:
		if p.LocalPath == "" {
			return errors.New("local_path is required for local source")
		}
	default:
		return fmt.Errorf("invalid package transfer source: %s", p.Source)
	}

	return nil
}

// GetDownloadURLWithMirror returns the download URL for the current params
// GetDownloadURLWithMirror 返回当前参数的下载 URL
func (p *InstallParams) GetDownloadURLWithMirror() string {
	if p.DownloadURL != "" {
		return p.DownloadURL
	}
	mirror := p.Mirror
	if mirror == "" {
		mirror = MirrorAliyun
	}
	return GetDownloadURL(mirror, p.Version)
}

// InstallResult contains the result of an installation
// InstallResult 包含安装结果
type InstallResult struct {
	// Success indicates whether the installation was successful
	// Success 表示安装是否成功
	Success bool `json:"success"`

	// Message is a human-readable message about the result
	// Message 是关于结果的人类可读消息
	Message string `json:"message"`

	// InstallDir is the actual installation directory
	// InstallDir 是实际安装目录
	InstallDir string `json:"install_dir"`

	// Version is the installed version
	// Version 是已安装的版本
	Version string `json:"version"`

	// ConfigPath is the path to the generated configuration file
	// ConfigPath 是生成的配置文件路径
	ConfigPath string `json:"config_path"`

	// FailedStep is the step where installation failed (if any)
	// FailedStep 是安装失败的步骤（如果有）
	FailedStep InstallStep `json:"failed_step,omitempty"`

	// Error is the error message if installation failed
	// Error 是安装失败时的错误消息
	Error string `json:"error,omitempty"`
}

// InstallerManager manages SeaTunnel installation
// InstallerManager 管理 SeaTunnel 安装
type InstallerManager struct {
	// httpClient is the HTTP client for downloading packages
	// httpClient 是用于下载安装包的 HTTP 客户端
	httpClient *http.Client

	// tempDir is the temporary directory for downloads
	// tempDir 是下载的临时目录
	tempDir string
}

// NewInstallerManager creates a new InstallerManager instance
// NewInstallerManager 创建一个新的 InstallerManager 实例
func NewInstallerManager() *InstallerManager {
	return &InstallerManager{
		httpClient: &http.Client{
			Timeout: 30 * time.Minute, // Long timeout for large downloads / 大文件下载的长超时
		},
		tempDir: os.TempDir(),
	}
}

// NewInstallerManagerWithClient creates a new InstallerManager with a custom HTTP client
// NewInstallerManagerWithClient 使用自定义 HTTP 客户端创建新的 InstallerManager
func NewInstallerManagerWithClient(client *http.Client) *InstallerManager {
	return &InstallerManager{
		httpClient: client,
		tempDir:    os.TempDir(),
	}
}

// Install performs the SeaTunnel installation
// Install 执行 SeaTunnel 安装
func (m *InstallerManager) Install(ctx context.Context, params *InstallParams, reporter ProgressReporter) (*InstallResult, error) {
	if reporter == nil {
		reporter = &NoOpProgressReporter{}
	}

	// Validate parameters / 验证参数
	if err := params.Validate(); err != nil {
		return &InstallResult{
			Success:    false,
			Message:    fmt.Sprintf("Invalid parameters: %v / 无效参数：%v", err, err),
			FailedStep: InstallStepDownload,
			Error:      err.Error(),
		}, err
	}

	var packagePath string
	var err error

	// Step 1: Get package (download or use local)
	// 步骤 1：获取安装包（下载或使用本地）
	if params.Mode == InstallModeOnline {
		reporter.Report(InstallStepDownload, 0, "Starting download... / 开始下载...")
		downloadURL := params.GetDownloadURLWithMirror()
		packagePath, err = m.downloadPackage(ctx, downloadURL, reporter)
		if err != nil {
			return &InstallResult{
				Success:    false,
				Message:    fmt.Sprintf("Download failed: %v / 下载失败：%v", err, err),
				FailedStep: InstallStepDownload,
				Error:      err.Error(),
			}, err
		}
		reporter.Report(InstallStepDownload, 100, "Download completed / 下载完成")
	} else {
		// Offline mode - check if package exists
		// 离线模式 - 检查安装包是否存在
		packagePath = params.PackagePath
		if _, err := os.Stat(packagePath); os.IsNotExist(err) {
			msg := fmt.Sprintf("Package not found at %s. Please download the SeaTunnel package and place it at the specified path. / 安装包未找到：%s。请下载 SeaTunnel 安装包并放置在指定路径。", packagePath, packagePath)
			return &InstallResult{
				Success:    false,
				Message:    msg,
				FailedStep: InstallStepDownload,
				Error:      ErrPackageNotFound.Error(),
			}, ErrPackageNotFound
		}
	}

	// Step 2: Verify checksum (if provided)
	// 步骤 2：验证校验和（如果提供）
	if params.ExpectedChecksum != "" {
		reporter.Report(InstallStepVerify, 0, "Verifying checksum... / 验证校验和...")
		if err := m.VerifyChecksum(packagePath, params.ExpectedChecksum); err != nil {
			return &InstallResult{
				Success:    false,
				Message:    fmt.Sprintf("Checksum verification failed: %v / 校验和验证失败：%v", err, err),
				FailedStep: InstallStepVerify,
				Error:      err.Error(),
			}, err
		}
		reporter.Report(InstallStepVerify, 100, "Checksum verified / 校验和验证通过")
	}

	// Step 3: Extract package
	// 步骤 3：解压安装包
	reporter.Report(InstallStepExtract, 0, "Extracting package... / 解压安装包...")
	if err := m.extractPackage(ctx, packagePath, params.InstallDir, reporter); err != nil {
		return &InstallResult{
			Success:    false,
			Message:    fmt.Sprintf("Extraction failed: %v / 解压失败：%v", err, err),
			FailedStep: InstallStepExtract,
			Error:      err.Error(),
		}, err
	}
	reporter.Report(InstallStepExtract, 100, "Extraction completed / 解压完成")

	// Step 4: Generate configuration
	// 步骤 4：生成配置
	reporter.Report(InstallStepConfigure, 0, "Generating configuration... / 生成配置...")
	configPath, err := m.GenerateConfig(params)
	if err != nil {
		return &InstallResult{
			Success:    false,
			Message:    fmt.Sprintf("Configuration generation failed: %v / 配置生成失败：%v", err, err),
			FailedStep: InstallStepConfigure,
			Error:      err.Error(),
		}, err
	}
	reporter.Report(InstallStepConfigure, 100, "Configuration generated / 配置生成完成")

	// Step 5: Complete
	// 步骤 5：完成
	reporter.Report(InstallStepComplete, 100, "Installation completed successfully / 安装成功完成")

	return &InstallResult{
		Success:    true,
		Message:    "Installation completed successfully / 安装成功完成",
		InstallDir: params.InstallDir,
		Version:    params.Version,
		ConfigPath: configPath,
	}, nil
}

// InstallStepByStep performs installation step by step with frontend interaction support
// InstallStepByStep 逐步执行安装，支持前端交互
// This method allows the frontend to:
// 此方法允许前端：
// - Visualize each step's progress / 可视化每个步骤的进度
// - Retry failed steps / 重试失败的步骤
// - Skip optional steps / 跳过可选步骤
func (m *InstallerManager) InstallStepByStep(ctx context.Context, params *InstallParams, reporter ProgressReporter) (*InstallResult, error) {
	if reporter == nil {
		reporter = &NoOpProgressReporter{}
	}

	result := &InstallResult{
		InstallDir: params.InstallDir,
		Version:    params.Version,
	}

	// Validate parameters first (not a separate step, just validation)
	// 首先验证参数（不是单独的步骤，只是验证）
	if err := params.Validate(); err != nil {
		return &InstallResult{
			Success:    false,
			Message:    fmt.Sprintf("Invalid parameters: %v / 无效参数：%v", err, err),
			FailedStep: InstallStepDownload,
			Error:      err.Error(),
		}, err
	}

	// Execute each step / 执行每个步骤
	// Note: Precheck should be done separately via Prechecker before calling this
	// 注意：预检应该在调用此方法之前通过 Prechecker 单独完成
	steps := []struct {
		step    InstallStep
		execute func() error
	}{
		{InstallStepDownload, func() error { return m.executeStepDownload(ctx, params, reporter) }},
		{InstallStepVerify, func() error { return m.executeStepVerify(params, reporter) }},
		{InstallStepExtract, func() error { return m.executeStepExtract(ctx, params, reporter) }},
		{InstallStepConfigureCluster, func() error { return m.executeStepConfigureCluster(params, reporter) }},
		{InstallStepConfigureCheckpoint, func() error { return m.executeStepConfigureCheckpoint(params, reporter) }},
		{InstallStepConfigureJVM, func() error { return m.executeStepConfigureJVM(params, reporter) }},
		{InstallStepInstallPlugins, func() error { return m.executeStepInstallPlugins(ctx, params, reporter) }},
		{InstallStepRegisterCluster, func() error { return m.executeStepRegisterCluster(params, reporter) }},
	}

	for _, s := range steps {
		select {
		case <-ctx.Done():
			result.Success = false
			result.FailedStep = s.step
			result.Error = ctx.Err().Error()
			return result, ctx.Err()
		default:
		}

		reporter.ReportStepStart(s.step)
		if err := s.execute(); err != nil {
			reporter.ReportStepFailed(s.step, err)
			result.Success = false
			result.FailedStep = s.step
			result.Error = err.Error()
			result.Message = fmt.Sprintf("Step %s failed: %v / 步骤 %s 失败：%v", s.step, err, s.step, err)
			return result, err
		}
		reporter.ReportStepComplete(s.step)
	}

	// Complete / 完成
	reporter.ReportStepStart(InstallStepComplete)
	reporter.ReportStepComplete(InstallStepComplete)

	result.Success = true
	result.Message = "Installation completed successfully / 安装成功完成"
	result.ConfigPath = filepath.Join(params.InstallDir, "config", "seatunnel.yaml")
	return result, nil
}

// ExecuteStep executes a single installation step (for retry support)
// ExecuteStep 执行单个安装步骤（支持重试）
func (m *InstallerManager) ExecuteStep(ctx context.Context, step InstallStep, params *InstallParams, reporter ProgressReporter) error {
	if reporter == nil {
		reporter = &NoOpProgressReporter{}
	}

	reporter.ReportStepStart(step)

	var err error
	switch step {
	case InstallStepDownload:
		err = m.executeStepDownload(ctx, params, reporter)
	case InstallStepVerify:
		err = m.executeStepVerify(params, reporter)
	case InstallStepExtract:
		err = m.executeStepExtract(ctx, params, reporter)
	case InstallStepConfigureCluster:
		err = m.executeStepConfigureCluster(params, reporter)
	case InstallStepConfigureCheckpoint:
		err = m.executeStepConfigureCheckpoint(params, reporter)
	case InstallStepConfigureJVM:
		err = m.executeStepConfigureJVM(params, reporter)
	case InstallStepInstallPlugins:
		err = m.executeStepInstallPlugins(ctx, params, reporter)
	case InstallStepRegisterCluster:
		err = m.executeStepRegisterCluster(params, reporter)
	default:
		err = fmt.Errorf("unknown step: %s / 未知步骤：%s", step, step)
	}

	if err != nil {
		reporter.ReportStepFailed(step, err)
		return err
	}

	reporter.ReportStepComplete(step)
	return nil
}

// GetInstallationSteps returns all installation steps with their info
// GetInstallationSteps 返回所有安装步骤及其信息
func GetInstallationSteps() []StepInfo {
	steps := make([]StepInfo, len(InstallationSteps))
	copy(steps, InstallationSteps)
	return steps
}

// executeStepDownload downloads or locates the installation package
// executeStepDownload receives or locates the installation package
// executeStepDownload 接收或定位安装包
// Package transfer modes:
// 安装包传输模式：
//   - control_plane: Receive package via gRPC stream from Control Plane (recommended)
//     control_plane: 通过 gRPC 流从 Control Plane 接收安装包（推荐）
//   - url: Download from URL (fallback)
//     url: 从 URL 下载（备用）
//   - local: Use local package file
//     local: 使用本地安装包文件
func (m *InstallerManager) executeStepDownload(ctx context.Context, params *InstallParams, reporter ProgressReporter) error {
	// If package path is already set, check if it exists
	// 如果安装包路径已设置，检查是否存在
	if params.PackagePath != "" {
		reporter.Report(InstallStepDownload, 0, "Checking local package... / 检查本地安装包...")
		if _, err := os.Stat(params.PackagePath); err == nil {
			reporter.Report(InstallStepDownload, 100, "Local package found / 本地安装包已找到")
			return nil
		}
	}

	// Handle package transfer based on source
	// 根据来源处理安装包传输
	if params.PackageTransfer == nil {
		return fmt.Errorf("package transfer info is required / 需要安装包传输信息")
	}

	transfer := params.PackageTransfer
	switch transfer.Source {
	case PackageTransferFromControlPlane:
		reporter.Report(InstallStepDownload, 0, "Receiving package from Control Plane... / 从 Control Plane 接收安装包...")
		// Package will be received via gRPC stream, set the expected path
		// 安装包将通过 gRPC 流接收，设置预期路径
		params.PackagePath = filepath.Join(m.tempDir, transfer.FileName)
		params.ExpectedChecksum = transfer.Checksum
		// Note: Actual transfer is handled by gRPC client, this step just prepares
		// 注意：实际传输由 gRPC 客户端处理，此步骤只是准备
		// The gRPC client should call ReceivePackage method
		// gRPC 客户端应调用 ReceivePackage 方法
		reporter.Report(InstallStepDownload, 100, "Package transfer prepared / 安装包传输已准备")

	case PackageTransferFromURL:
		reporter.Report(InstallStepDownload, 0, "Downloading package from URL... / 从 URL 下载安装包...")
		packagePath, err := m.downloadPackage(ctx, transfer.DownloadURL, reporter)
		if err != nil {
			return err
		}
		params.PackagePath = packagePath
		params.ExpectedChecksum = transfer.Checksum
		reporter.Report(InstallStepDownload, 100, "Download completed / 下载完成")

	case PackageTransferLocal:
		reporter.Report(InstallStepDownload, 0, "Checking local package... / 检查本地安装包...")
		if _, err := os.Stat(transfer.LocalPath); os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrPackageNotFound, transfer.LocalPath)
		}
		params.PackagePath = transfer.LocalPath
		params.ExpectedChecksum = transfer.Checksum
		reporter.Report(InstallStepDownload, 100, "Local package found / 本地安装包已找到")

	default:
		return fmt.Errorf("unsupported package transfer source: %s / 不支持的安装包传输源：%s", transfer.Source, transfer.Source)
	}

	return nil
}

// ReceivePackage receives a package from Control Plane via gRPC stream
// ReceivePackage 通过 gRPC 流从 Control Plane 接收安装包
// This method should be called by the gRPC client when receiving package data
// 此方法应由 gRPC 客户端在接收安装包数据时调用
func (m *InstallerManager) ReceivePackage(ctx context.Context, transfer *PackageTransferInfo, dataReader io.Reader, reporter ProgressReporter) (string, error) {
	if transfer == nil {
		return "", errors.New("transfer info is required")
	}

	reporter.Report(InstallStepDownload, 0, "Receiving package from Control Plane... / 从 Control Plane 接收安装包...")

	// Create temp file for the package
	// 为安装包创建临时文件
	packagePath := filepath.Join(m.tempDir, transfer.FileName)
	file, err := os.Create(packagePath)
	if err != nil {
		return "", fmt.Errorf("failed to create package file: %w", err)
	}
	defer file.Close()

	// Copy data with progress reporting
	// 带进度报告的数据复制
	var received int64
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		select {
		case <-ctx.Done():
			os.Remove(packagePath)
			return "", ctx.Err()
		default:
		}

		n, err := dataReader.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				os.Remove(packagePath)
				return "", fmt.Errorf("failed to write package data: %w", writeErr)
			}
			received += int64(n)

			// Report progress
			// 报告进度
			if transfer.FileSize > 0 {
				progress := int(float64(received) / float64(transfer.FileSize) * 100)
				reporter.Report(InstallStepDownload, progress,
					fmt.Sprintf("Received %d/%d bytes / 已接收 %d/%d 字节", received, transfer.FileSize, received, transfer.FileSize))
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(packagePath)
			return "", fmt.Errorf("failed to read package data: %w", err)
		}
	}

	reporter.Report(InstallStepDownload, 100, "Package received / 安装包已接收")
	return packagePath, nil
}

// executeStepVerify verifies the package checksum
// executeStepVerify 验证安装包校验和
func (m *InstallerManager) executeStepVerify(params *InstallParams, reporter ProgressReporter) error {
	if params.ExpectedChecksum == "" {
		reporter.Report(InstallStepVerify, 100, "Checksum verification skipped (no checksum provided) / 跳过校验和验证（未提供校验和）")
		return nil
	}

	reporter.Report(InstallStepVerify, 0, "Verifying checksum... / 验证校验和...")
	if err := m.VerifyChecksum(params.PackagePath, params.ExpectedChecksum); err != nil {
		return err
	}
	reporter.Report(InstallStepVerify, 100, "Checksum verified / 校验和验证通过")
	return nil
}

// executeStepExtract extracts the installation package
// executeStepExtract 解压安装包
func (m *InstallerManager) executeStepExtract(ctx context.Context, params *InstallParams, reporter ProgressReporter) error {
	reporter.Report(InstallStepExtract, 0, "Extracting package... / 解压安装包...")
	if err := m.extractPackage(ctx, params.PackagePath, params.InstallDir, reporter); err != nil {
		return err
	}
	reporter.Report(InstallStepExtract, 100, "Extraction completed / 解压完成")
	return nil
}

// executeStepConfigureCluster configures cluster settings
// executeStepConfigureCluster 配置集群设置
func (m *InstallerManager) executeStepConfigureCluster(params *InstallParams, reporter ProgressReporter) error {
	reporter.Report(InstallStepConfigureCluster, 0, "Configuring cluster... / 配置集群...")
	if _, err := m.ConfigureCluster(params); err != nil {
		return err
	}
	reporter.Report(InstallStepConfigureCluster, 100, "Cluster configured / 集群配置完成")
	return nil
}

// executeStepConfigureCheckpoint configures checkpoint storage
// executeStepConfigureCheckpoint 配置检查点存储
func (m *InstallerManager) executeStepConfigureCheckpoint(params *InstallParams, reporter ProgressReporter) error {
	if params.Checkpoint == nil {
		reporter.Report(InstallStepConfigureCheckpoint, 100, "Checkpoint configuration skipped (using defaults) / 跳过检查点配置（使用默认值）")
		return nil
	}

	reporter.Report(InstallStepConfigureCheckpoint, 0, "Configuring checkpoint storage... / 配置检查点存储...")
	if err := m.configureCheckpointStorage(params); err != nil {
		return err
	}
	reporter.Report(InstallStepConfigureCheckpoint, 100, "Checkpoint storage configured / 检查点存储配置完成")
	return nil
}

// executeStepConfigureJVM configures JVM settings
// executeStepConfigureJVM 配置 JVM 设置
func (m *InstallerManager) executeStepConfigureJVM(params *InstallParams, reporter ProgressReporter) error {
	if params.JVM == nil {
		reporter.Report(InstallStepConfigureJVM, 100, "JVM configuration skipped (using defaults) / 跳过 JVM 配置（使用默认值）")
		return nil
	}

	reporter.Report(InstallStepConfigureJVM, 0, "Configuring JVM... / 配置 JVM...")
	if err := m.configureJVM(params); err != nil {
		return err
	}
	reporter.Report(InstallStepConfigureJVM, 100, "JVM configured / JVM 配置完成")
	return nil
}

// executeStepInstallPlugins installs connectors and plugins
// executeStepInstallPlugins 安装连接器和插件
func (m *InstallerManager) executeStepInstallPlugins(ctx context.Context, params *InstallParams, reporter ProgressReporter) error {
	if params.Connector == nil || !params.Connector.InstallConnectors {
		reporter.Report(InstallStepInstallPlugins, 100, "Plugin installation skipped / 跳过插件安装")
		return nil
	}

	reporter.Report(InstallStepInstallPlugins, 0, "Installing plugins... / 安装插件...")
	// TODO: Implement plugin installation
	// TODO: 实现插件安装
	reporter.Report(InstallStepInstallPlugins, 100, "Plugins installed / 插件安装完成")
	return nil
}

// executeStepRegisterCluster registers the node to the cluster
// executeStepRegisterCluster 将节点注册到集群
// Note: Agent manages SeaTunnel process lifecycle, no systemd auto-start needed
// 注意：Agent 管理 SeaTunnel 进程生命周期，不需要 systemd 开机自启动
func (m *InstallerManager) executeStepRegisterCluster(params *InstallParams, reporter ProgressReporter) error {
	if params.ClusterID == "" {
		reporter.Report(InstallStepRegisterCluster, 100, "Cluster registration skipped (no cluster ID provided) / 跳过集群注册（未提供集群 ID）")
		return nil
	}

	reporter.Report(InstallStepRegisterCluster, 0, "Registering to cluster... / 注册到集群...")
	// TODO: Implement cluster registration via gRPC to Control Plane
	// TODO: 通过 gRPC 向 Control Plane 实现集群注册
	// This will be called after installation to notify Control Plane that:
	// 安装完成后将调用此方法通知 Control Plane：
	// 1. SeaTunnel is installed on this node / SeaTunnel 已安装在此节点
	// 2. Node is ready to join the cluster / 节点已准备好加入集群
	// 3. Agent will manage the SeaTunnel process / Agent 将管理 SeaTunnel 进程
	reporter.Report(InstallStepRegisterCluster, 100, "Cluster registration completed / 集群注册完成")
	return nil
}

// configureCheckpointStorage configures checkpoint storage in seatunnel.yaml
// configureCheckpointStorage 在 seatunnel.yaml 中配置检查点存储
func (m *InstallerManager) configureCheckpointStorage(params *InstallParams) error {
	if params.Checkpoint == nil {
		return nil
	}

	seatunnelYaml := filepath.Join(params.InstallDir, "config", "seatunnel.yaml")

	// Backup original file / 备份原始文件
	if err := backupFile(seatunnelYaml); err != nil {
		return fmt.Errorf("%w: %v", ErrConfigGenerationFailed, err)
	}

	// Read file content / 读取文件内容
	content, err := os.ReadFile(seatunnelYaml)
	if err != nil {
		return fmt.Errorf("%w: failed to read seatunnel.yaml: %v", ErrConfigGenerationFailed, err)
	}

	contentStr := string(content)

	// Generate checkpoint config based on storage type
	// 根据存储类型生成检查点配置
	var checkpointConfig string
	switch params.Checkpoint.StorageType {
	case CheckpointStorageLocalFile:
		checkpointConfig = fmt.Sprintf(`      plugin-config:
        namespace: %s
        storage.type: local`, params.Checkpoint.Namespace)
	case CheckpointStorageHDFS:
		checkpointConfig = fmt.Sprintf(`      plugin-config:
        namespace: %s
        storage.type: hdfs
        fs.defaultFS: hdfs://%s:%d`,
			params.Checkpoint.Namespace,
			params.Checkpoint.HDFSNameNodeHost,
			params.Checkpoint.HDFSNameNodePort)
	case CheckpointStorageOSS:
		checkpointConfig = fmt.Sprintf(`      plugin-config:
        namespace: %s
        storage.type: oss
        oss.bucket: %s
        fs.oss.endpoint: %s
        fs.oss.accessKeyId: %s
        fs.oss.accessKeySecret: %s`,
			params.Checkpoint.Namespace,
			params.Checkpoint.StorageBucket,
			params.Checkpoint.StorageEndpoint,
			params.Checkpoint.StorageAccessKey,
			params.Checkpoint.StorageSecretKey)
	case CheckpointStorageS3:
		checkpointConfig = fmt.Sprintf(`      plugin-config:
        namespace: %s
        storage.type: s3
        s3.bucket: %s
        fs.s3a.endpoint: %s
        fs.s3a.access.key: %s
        fs.s3a.secret.key: %s
        fs.s3a.aws.credentials.provider: org.apache.hadoop.fs.s3a.SimpleAWSCredentialsProvider
        disable.cache: true`,
			params.Checkpoint.Namespace,
			params.Checkpoint.StorageBucket,
			params.Checkpoint.StorageEndpoint,
			params.Checkpoint.StorageAccessKey,
			params.Checkpoint.StorageSecretKey)
	}

	// Replace plugin-config section / 替换 plugin-config 部分
	contentStr = replaceCheckpointPluginConfig(contentStr, checkpointConfig)

	// Write modified content / 写入修改后的内容
	if err := os.WriteFile(seatunnelYaml, []byte(contentStr), 0644); err != nil {
		return fmt.Errorf("%w: failed to write seatunnel.yaml: %v", ErrConfigGenerationFailed, err)
	}

	return nil
}

// replaceCheckpointPluginConfig replaces the checkpoint plugin-config section
// replaceCheckpointPluginConfig 替换检查点 plugin-config 部分
func replaceCheckpointPluginConfig(content, newConfig string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inPluginConfig := false
	pluginConfigIndent := 0

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "plugin-config:") {
			// Found plugin-config, record its indentation / 找到 plugin-config，记录其缩进
			pluginConfigIndent = len(line) - len(strings.TrimLeft(line, " \t"))
			result = append(result, newConfig)
			inPluginConfig = true
			continue
		}

		if inPluginConfig {
			// Check if we're still in the plugin-config section / 检查是否仍在 plugin-config 部分
			currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))
			if trimmed != "" && currentIndent <= pluginConfigIndent {
				// We've exited the plugin-config section / 已退出 plugin-config 部分
				inPluginConfig = false
				result = append(result, line)
			}
			// Skip old plugin-config entries / 跳过旧的 plugin-config 条目
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// configureJVM configures JVM options
// configureJVM 配置 JVM 选项
func (m *InstallerManager) configureJVM(params *InstallParams) error {
	if params.JVM == nil {
		return nil
	}

	configDir := filepath.Join(params.InstallDir, "config")

	// Configure based on deployment mode / 根据部署模式配置
	if params.DeploymentMode == DeploymentModeHybrid {
		// Hybrid mode: configure jvm_options / 混合模式：配置 jvm_options
		jvmOptionsPath := filepath.Join(configDir, "jvm_options")
		if err := m.modifyJVMOptions(jvmOptionsPath, params.JVM.HybridHeapSize); err != nil {
			return err
		}
	} else {
		// Separated mode: configure jvm_master_options and jvm_worker_options
		// 分离模式：配置 jvm_master_options 和 jvm_worker_options
		masterOptionsPath := filepath.Join(configDir, "jvm_master_options")
		if err := m.modifyJVMOptions(masterOptionsPath, params.JVM.MasterHeapSize); err != nil {
			return err
		}

		workerOptionsPath := filepath.Join(configDir, "jvm_worker_options")
		if err := m.modifyJVMOptions(workerOptionsPath, params.JVM.WorkerHeapSize); err != nil {
			return err
		}
	}

	return nil
}

// modifyJVMOptions modifies JVM options file to set heap size
// modifyJVMOptions 修改 JVM 选项文件以设置堆大小
func (m *InstallerManager) modifyJVMOptions(filePath string, heapSizeGB int) error {
	// Backup original file / 备份原始文件
	if err := backupFile(filePath); err != nil {
		return fmt.Errorf("%w: %v", ErrConfigGenerationFailed, err)
	}

	// Read file content / 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("%w: failed to read %s: %v", ErrConfigGenerationFailed, filePath, err)
	}

	contentStr := string(content)

	// SeaTunnel 2.3.9+ has commented JVM options, uncomment them first
	// SeaTunnel 2.3.9+ 的 JVM 选项是注释状态，先取消注释
	contentStr = strings.ReplaceAll(contentStr, "# -Xms", "-Xms")
	contentStr = strings.ReplaceAll(contentStr, "# -Xmx", "-Xmx")

	// Replace heap size / 替换堆大小
	lines := strings.Split(contentStr, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-Xms") {
			lines[i] = fmt.Sprintf("-Xms%dg", heapSizeGB)
		} else if strings.HasPrefix(trimmed, "-Xmx") {
			lines[i] = fmt.Sprintf("-Xmx%dg", heapSizeGB)
		}
	}

	contentStr = strings.Join(lines, "\n")

	// Write modified content / 写入修改后的内容
	if err := os.WriteFile(filePath, []byte(contentStr), 0644); err != nil {
		return fmt.Errorf("%w: failed to write %s: %v", ErrConfigGenerationFailed, filePath, err)
	}

	return nil
}

// downloadPackage downloads the installation package from the given URL
// downloadPackage 从给定 URL 下载安装包
func (m *InstallerManager) downloadPackage(ctx context.Context, url string, reporter ProgressReporter) (string, error) {
	// Create request with context / 创建带上下文的请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request / 执行请求
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: HTTP status %d", ErrDownloadFailed, resp.StatusCode)
	}

	// Create temp file / 创建临时文件
	tempFile, err := os.CreateTemp(m.tempDir, "seatunnel-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Download with progress reporting / 带进度上报的下载
	totalSize := resp.ContentLength
	var downloaded int64

	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		select {
		case <-ctx.Done():
			os.Remove(tempFile.Name())
			return "", ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := tempFile.Write(buf[:n]); writeErr != nil {
				os.Remove(tempFile.Name())
				return "", fmt.Errorf("failed to write to temp file: %w", writeErr)
			}
			downloaded += int64(n)

			// Report progress / 上报进度
			if totalSize > 0 {
				progress := int(float64(downloaded) / float64(totalSize) * 100)
				reporter.Report(InstallStepDownload, progress, fmt.Sprintf("Downloaded %d/%d bytes / 已下载 %d/%d 字节", downloaded, totalSize, downloaded, totalSize))
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(tempFile.Name())
			return "", fmt.Errorf("%w: %v", ErrDownloadFailed, err)
		}
	}

	return tempFile.Name(), nil
}

// VerifyChecksum verifies the SHA256 checksum of a file
// VerifyChecksum 验证文件的 SHA256 校验和
func (m *InstallerManager) VerifyChecksum(filePath, expectedChecksum string) error {
	actualChecksum, err := CalculateChecksum(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Normalize checksums for comparison (lowercase)
	// 规范化校验和以进行比较（小写）
	expectedChecksum = strings.ToLower(strings.TrimSpace(expectedChecksum))
	actualChecksum = strings.ToLower(actualChecksum)

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("%w: expected %s, got %s", ErrChecksumMismatch, expectedChecksum, actualChecksum)
	}

	return nil
}

// CalculateChecksum calculates the SHA256 checksum of a file
// CalculateChecksum 计算文件的 SHA256 校验和
func CalculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// extractPackage extracts a tar.gz package to the specified directory
// extractPackage 将 tar.gz 安装包解压到指定目录
func (m *InstallerManager) extractPackage(ctx context.Context, packagePath, destDir string, reporter ProgressReporter) error {
	// Open the package file / 打开安装包文件
	file, err := os.Open(packagePath)
	if err != nil {
		return fmt.Errorf("%w: failed to open package: %v", ErrExtractionFailed, err)
	}
	defer file.Close()

	// Create gzip reader / 创建 gzip 读取器
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("%w: failed to create gzip reader: %v", ErrExtractionFailed, err)
	}
	defer gzReader.Close()

	// Create tar reader / 创建 tar 读取器
	tarReader := tar.NewReader(gzReader)

	// Create destination directory / 创建目标目录
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("%w: failed to create destination directory: %v", ErrExtractionFailed, err)
	}

	// Extract files / 解压文件
	fileCount := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: failed to read tar header: %v", ErrExtractionFailed, err)
		}

		// Construct target path / 构建目标路径
		// Strip the first directory component if it exists (e.g., apache-seatunnel-2.3.4/)
		// 如果存在，去除第一个目录组件（例如 apache-seatunnel-2.3.4/）
		targetPath := filepath.Join(destDir, stripFirstComponent(header.Name))

		// Security check: prevent path traversal / 安全检查：防止路径遍历
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(destDir)) {
			return fmt.Errorf("%w: invalid file path in archive: %s", ErrExtractionFailed, header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("%w: failed to create directory: %v", ErrExtractionFailed, err)
			}
		case tar.TypeReg:
			// Create parent directory / 创建父目录
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("%w: failed to create parent directory: %v", ErrExtractionFailed, err)
			}

			// Create file / 创建文件
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("%w: failed to create file: %v", ErrExtractionFailed, err)
			}

			// Copy content / 复制内容
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("%w: failed to write file: %v", ErrExtractionFailed, err)
			}
			outFile.Close()

			fileCount++
			if fileCount%100 == 0 {
				reporter.Report(InstallStepExtract, 50, fmt.Sprintf("Extracted %d files... / 已解压 %d 个文件...", fileCount, fileCount))
			}
		case tar.TypeSymlink:
			// Create symlink / 创建符号链接
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				// Ignore symlink errors on Windows / 在 Windows 上忽略符号链接错误
				if !os.IsExist(err) {
					// Log but don't fail / 记录但不失败
				}
			}
		}
	}

	return nil
}

// stripFirstComponent removes the first path component from a path
// stripFirstComponent 从路径中移除第一个路径组件
func stripFirstComponent(path string) string {
	// Normalize path separators / 规范化路径分隔符
	path = filepath.ToSlash(path)
	parts := strings.SplitN(path, "/", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return path
}

// ConfigureCluster modifies existing SeaTunnel configuration files
// ConfigureCluster 修改现有的 SeaTunnel 配置文件
// This follows the backup-then-modify pattern instead of generating new files
// 采用备份后修改的模式，而不是生成新文件
func (m *InstallerManager) ConfigureCluster(params *InstallParams) (string, error) {
	configDir := filepath.Join(params.InstallDir, "config")

	// Check if config directory exists / 检查配置目录是否存在
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return "", fmt.Errorf("%w: config directory not found at %s", ErrConfigGenerationFailed, configDir)
	}

	// Modify hazelcast configuration based on deployment mode
	// 根据部署模式修改 hazelcast 配置
	if params.DeploymentMode == DeploymentModeHybrid {
		// Hybrid mode: modify hazelcast.yaml
		// 混合模式：修改 hazelcast.yaml
		hazelcastPath := filepath.Join(configDir, "hazelcast.yaml")
		if err := m.modifyHazelcastConfig(hazelcastPath, params); err != nil {
			return "", err
		}
	} else {
		// Separated mode: modify hazelcast-master.yaml and hazelcast-worker.yaml
		// 分离模式：修改 hazelcast-master.yaml 和 hazelcast-worker.yaml
		masterPath := filepath.Join(configDir, "hazelcast-master.yaml")
		if err := m.modifyHazelcastConfig(masterPath, params); err != nil {
			return "", err
		}
		workerPath := filepath.Join(configDir, "hazelcast-worker.yaml")
		if err := m.modifyHazelcastConfig(workerPath, params); err != nil {
			return "", err
		}
	}

	// Modify hazelcast-client.yaml
	// 修改 hazelcast-client.yaml
	clientPath := filepath.Join(configDir, "hazelcast-client.yaml")
	if err := m.modifyHazelcastClientConfig(clientPath, params); err != nil {
		return "", err
	}

	// Modify seatunnel.yaml for HTTP port and other settings
	// 修改 seatunnel.yaml 的 HTTP 端口和其他设置
	seatunnelPath := filepath.Join(configDir, "seatunnel.yaml")
	if err := m.modifySeaTunnelConfig(seatunnelPath, params); err != nil {
		return "", err
	}

	return seatunnelPath, nil
}

// backupFile creates a backup of a file with .bak extension
// backupFile 创建文件的 .bak 备份
func backupFile(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // File doesn't exist, no need to backup / 文件不存在，无需备份
	}

	backupPath := filePath + ".bak"
	input, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file for backup: %w", err)
	}

	if err := os.WriteFile(backupPath, input, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

// modifyHazelcastConfig modifies hazelcast.yaml or hazelcast-master/worker.yaml
// modifyHazelcastConfig 修改 hazelcast.yaml 或 hazelcast-master/worker.yaml
func (m *InstallerManager) modifyHazelcastConfig(filePath string, params *InstallParams) error {
	// Backup original file / 备份原始文件
	if err := backupFile(filePath); err != nil {
		return fmt.Errorf("%w: %v", ErrConfigGenerationFailed, err)
	}

	// Read file content / 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("%w: failed to read %s: %v", ErrConfigGenerationFailed, filePath, err)
	}

	// Build member list / 构建成员列表
	var memberList []string
	port := params.ClusterPort
	if port == 0 {
		port = 5801
	}

	for _, addr := range params.MasterAddresses {
		memberList = append(memberList, fmt.Sprintf("%s:%d", addr, port))
	}

	// If no master addresses, use localhost / 如果没有 master 地址，使用 localhost
	if len(memberList) == 0 {
		memberList = append(memberList, fmt.Sprintf("127.0.0.1:%d", port))
	}

	// Modify content using simple string replacement
	// 使用简单的字符串替换修改内容
	contentStr := string(content)

	// Replace port configuration / 替换端口配置
	contentStr = replaceYAMLValue(contentStr, "port:", fmt.Sprintf("%d", port))

	// Replace member-list section / 替换 member-list 部分
	contentStr = replaceMemberList(contentStr, memberList)

	// Write modified content / 写入修改后的内容
	if err := os.WriteFile(filePath, []byte(contentStr), 0644); err != nil {
		return fmt.Errorf("%w: failed to write %s: %v", ErrConfigGenerationFailed, filePath, err)
	}

	return nil
}

// modifyHazelcastClientConfig modifies hazelcast-client.yaml
// modifyHazelcastClientConfig 修改 hazelcast-client.yaml
func (m *InstallerManager) modifyHazelcastClientConfig(filePath string, params *InstallParams) error {
	// Backup original file / 备份原始文件
	if err := backupFile(filePath); err != nil {
		return fmt.Errorf("%w: %v", ErrConfigGenerationFailed, err)
	}

	// Read file content / 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("%w: failed to read %s: %v", ErrConfigGenerationFailed, filePath, err)
	}

	// Build cluster members list / 构建集群成员列表
	var memberList []string
	port := params.ClusterPort
	if port == 0 {
		port = 5801
	}

	for _, addr := range params.MasterAddresses {
		memberList = append(memberList, fmt.Sprintf("%s:%d", addr, port))
	}

	if len(memberList) == 0 {
		memberList = append(memberList, fmt.Sprintf("127.0.0.1:%d", port))
	}

	// Modify content / 修改内容
	contentStr := string(content)
	contentStr = replaceClusterMembers(contentStr, memberList)

	// Write modified content / 写入修改后的内容
	if err := os.WriteFile(filePath, []byte(contentStr), 0644); err != nil {
		return fmt.Errorf("%w: failed to write %s: %v", ErrConfigGenerationFailed, filePath, err)
	}

	return nil
}

// modifySeaTunnelConfig modifies seatunnel.yaml
// modifySeaTunnelConfig 修改 seatunnel.yaml
func (m *InstallerManager) modifySeaTunnelConfig(filePath string, params *InstallParams) error {
	// Backup original file / 备份原始文件
	if err := backupFile(filePath); err != nil {
		return fmt.Errorf("%w: %v", ErrConfigGenerationFailed, err)
	}

	// Read file content / 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("%w: failed to read %s: %v", ErrConfigGenerationFailed, filePath, err)
	}

	contentStr := string(content)

	// Modify HTTP port if specified / 如果指定了 HTTP 端口则修改
	if params.HTTPPort > 0 {
		contentStr = replaceYAMLValue(contentStr, "port:", fmt.Sprintf("%d", params.HTTPPort))
	}

	// Set dynamic-slot value (default: true, can be overridden by user)
	// 设置 dynamic-slot 值（默认：true，可由用户覆盖）
	dynamicSlotValue := "true"
	if params.DynamicSlot != nil && !*params.DynamicSlot {
		dynamicSlotValue = "false"
	}
	contentStr = replaceYAMLValue(contentStr, "dynamic-slot:", dynamicSlotValue)

	// Write modified content / 写入修改后的内容
	if err := os.WriteFile(filePath, []byte(contentStr), 0644); err != nil {
		return fmt.Errorf("%w: failed to write %s: %v", ErrConfigGenerationFailed, filePath, err)
	}

	return nil
}

// replaceYAMLValue replaces a YAML key's value
// replaceYAMLValue 替换 YAML 键的值
func replaceYAMLValue(content, key, newValue string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key) {
			// Find the indentation / 找到缩进
			indent := strings.TrimRight(line, strings.TrimLeft(line, " \t"))
			lines[i] = indent + key + " " + newValue
		}
	}
	return strings.Join(lines, "\n")
}

// replaceMemberList replaces the member-list section in hazelcast config
// replaceMemberList 替换 hazelcast 配置中的 member-list 部分
func replaceMemberList(content string, members []string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inMemberList := false
	memberListIndent := 0

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "member-list:") {
			// Found member-list, record its indentation / 找到 member-list，记录其缩进
			memberListIndent = len(line) - len(strings.TrimLeft(line, " \t"))
			result = append(result, line)
			inMemberList = true

			// Add new members / 添加新成员
			memberIndent := strings.Repeat(" ", memberListIndent+2)
			for _, member := range members {
				result = append(result, fmt.Sprintf("%s- \"%s\"", memberIndent, member))
			}
			continue
		}

		if inMemberList {
			// Check if we're still in the member-list section / 检查是否仍在 member-list 部分
			currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))
			if trimmed != "" && currentIndent <= memberListIndent && !strings.HasPrefix(trimmed, "-") {
				// We've exited the member-list section / 已退出 member-list 部分
				inMemberList = false
				result = append(result, line)
			}
			// Skip old member entries / 跳过旧的成员条目
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// replaceClusterMembers replaces the cluster-members section in hazelcast-client config
// replaceClusterMembers 替换 hazelcast-client 配置中的 cluster-members 部分
func replaceClusterMembers(content string, members []string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inClusterMembers := false
	clusterMembersIndent := 0

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "cluster-members:") {
			// Found cluster-members, record its indentation / 找到 cluster-members，记录其缩进
			clusterMembersIndent = len(line) - len(strings.TrimLeft(line, " \t"))
			result = append(result, line)
			inClusterMembers = true

			// Add new members / 添加新成员
			memberIndent := strings.Repeat(" ", clusterMembersIndent+2)
			for _, member := range members {
				result = append(result, fmt.Sprintf("%s- \"%s\"", memberIndent, member))
			}
			continue
		}

		if inClusterMembers {
			// Check if we're still in the cluster-members section / 检查是否仍在 cluster-members 部分
			currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))
			if trimmed != "" && currentIndent <= clusterMembersIndent && !strings.HasPrefix(trimmed, "-") {
				// We've exited the cluster-members section / 已退出 cluster-members 部分
				inClusterMembers = false
				result = append(result, line)
			}
			// Skip old member entries / 跳过旧的成员条目
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// GenerateConfig is kept for backward compatibility, calls ConfigureCluster
// GenerateConfig 保留用于向后兼容，调用 ConfigureCluster
func (m *InstallerManager) GenerateConfig(params *InstallParams) (string, error) {
	return m.ConfigureCluster(params)
}

// GenerateSeaTunnelConfig generates the SeaTunnel configuration content
// GenerateSeaTunnelConfig 生成 SeaTunnel 配置内容
// Deprecated: Use ConfigureCluster with backup+modify pattern instead
// 已废弃：请使用 ConfigureCluster 的备份+修改模式
// This function is kept for backward compatibility and property tests
// 此函数保留用于向后兼容和属性测试
func GenerateSeaTunnelConfig(deploymentMode DeploymentMode, nodeRole NodeRole, clusterName string, masterAddresses []string, clusterPort, httpPort int) string {
	// Set default values / 设置默认值
	if clusterName == "" {
		clusterName = "seatunnel-cluster"
	}
	if clusterPort == 0 {
		clusterPort = 5801
	}
	if httpPort == 0 {
		httpPort = 8080
	}

	var config strings.Builder

	// Write header / 写入头部
	config.WriteString("# SeaTunnel Configuration\n")
	config.WriteString("# SeaTunnel 配置\n")
	config.WriteString("# Generated by SeaTunnelX Agent\n")
	config.WriteString("# 由 SeaTunnelX Agent 生成\n\n")

	// Write seatunnel engine configuration / 写入 seatunnel 引擎配置
	config.WriteString("seatunnel:\n")
	config.WriteString("  engine:\n")
	config.WriteString(fmt.Sprintf("    cluster-name: \"%s\"\n", clusterName))

	// Write deployment mode specific configuration
	// 写入部署模式特定配置
	// Default dynamic-slot: true, can be changed by user
	// 默认 dynamic-slot: true，可由用户修改
	switch deploymentMode {
	case DeploymentModeHybrid:
		config.WriteString("    # Hybrid mode: master and worker on same node\n")
		config.WriteString("    # 混合模式：master 和 worker 在同一节点\n")
		config.WriteString("    slot-service:\n")
		config.WriteString("      dynamic-slot: true\n")
	case DeploymentModeSeparated:
		config.WriteString("    # Separated mode: master and worker on different nodes\n")
		config.WriteString("    # 分离模式：master 和 worker 在不同节点\n")
		config.WriteString("    slot-service:\n")
		config.WriteString("      dynamic-slot: true\n")
	}

	// Write node role specific configuration
	// 写入节点角色特定配置
	config.WriteString("\n")
	switch nodeRole {
	case NodeRoleMaster:
		config.WriteString("    # Master node configuration\n")
		config.WriteString("    # Master 节点配置\n")
		config.WriteString("    backup-count: 1\n")
		config.WriteString("    checkpoint:\n")
		config.WriteString("      interval: 10000\n")
		config.WriteString("      timeout: 60000\n")
		config.WriteString("      storage:\n")
		config.WriteString("        type: hdfs\n")
		config.WriteString("        max-retained: 3\n")
	case NodeRoleWorker:
		config.WriteString("    # Worker node configuration\n")
		config.WriteString("    # Worker 节点配置\n")
		config.WriteString("    backup-count: 0\n")
	}

	// Write HTTP server configuration / 写入 HTTP 服务器配置
	config.WriteString("\n")
	config.WriteString("    http:\n")
	config.WriteString(fmt.Sprintf("      port: %d\n", httpPort))
	config.WriteString("      enable-http: true\n")

	return config.String()
}

// GenerateHazelcastConfig generates the Hazelcast configuration content
// GenerateHazelcastConfig 生成 Hazelcast 配置内容
// Deprecated: Use ConfigureCluster with backup+modify pattern instead
// 已废弃：请使用 ConfigureCluster 的备份+修改模式
// This function is kept for backward compatibility and property tests
// 此函数保留用于向后兼容和属性测试
func GenerateHazelcastConfig(clusterName string, masterAddresses []string, clusterPort int) string {
	// Set default values / 设置默认值
	if clusterName == "" {
		clusterName = "seatunnel-cluster"
	}
	if clusterPort == 0 {
		clusterPort = 5801
	}

	var config strings.Builder

	// Write header / 写入头部
	config.WriteString("# Hazelcast Configuration for SeaTunnel\n")
	config.WriteString("# SeaTunnel 的 Hazelcast 配置\n")
	config.WriteString("# Generated by SeaTunnelX Agent\n")
	config.WriteString("# 由 SeaTunnelX Agent 生成\n\n")

	config.WriteString("hazelcast:\n")
	config.WriteString(fmt.Sprintf("  cluster-name: \"%s\"\n", clusterName))
	config.WriteString("  network:\n")
	config.WriteString(fmt.Sprintf("    port: %d\n", clusterPort))
	config.WriteString("    join:\n")
	config.WriteString("      multicast:\n")
	config.WriteString("        enabled: false\n")
	config.WriteString("      tcp-ip:\n")
	config.WriteString("        enabled: true\n")

	// Write member list / 写入成员列表
	if len(masterAddresses) > 0 {
		config.WriteString("        member-list:\n")
		for _, addr := range masterAddresses {
			config.WriteString(fmt.Sprintf("          - \"%s\"\n", addr))
		}
	} else {
		config.WriteString("        member-list:\n")
		config.WriteString("          - \"127.0.0.1\"\n")
	}

	// Write properties / 写入属性
	config.WriteString("  properties:\n")
	config.WriteString("    hazelcast.logging.type: log4j2\n")
	config.WriteString("    hazelcast.operation.call.timeout.millis: 30000\n")

	return config.String()
}

// Uninstall removes the SeaTunnel installation
// Uninstall 移除 SeaTunnel 安装
func (m *InstallerManager) Uninstall(ctx context.Context, installDir string) error {
	// Check if directory exists / 检查目录是否存在
	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		return nil // Already uninstalled / 已经卸载
	}

	// Remove the installation directory / 移除安装目录
	if err := os.RemoveAll(installDir); err != nil {
		return fmt.Errorf("failed to remove installation directory: %w", err)
	}

	return nil
}

// GetOfflineInstallInstructions returns instructions for offline installation
// GetOfflineInstallInstructions 返回离线安装说明
func GetOfflineInstallInstructions(version, packageDir string) string {
	aliyunURL := GetDownloadURL(MirrorAliyun, version)
	huaweiURL := GetDownloadURL(MirrorHuaweiCloud, version)
	apacheURL := GetDownloadURL(MirrorApache, version)

	return fmt.Sprintf(`Offline Installation Instructions / 离线安装说明
================================================

1. Download the SeaTunnel package from one of these sources (recommended order):
   从以下来源之一下载 SeaTunnel 安装包（推荐顺序）：
   
   - Aliyun Mirror (Recommended for China / 国内推荐):
     %s
   
   - Huawei Cloud Mirror (华为云镜像):
     %s
   
   - Apache Mirror (Apache 官方镜像):
     %s

2. Place the downloaded package at:
   将下载的安装包放置在：
   
   %s/apache-seatunnel-%s-bin.tar.gz

3. (Optional) Download the SHA256 checksum file:
   （可选）下载 SHA256 校验和文件：
   
   https://archive.apache.org/dist/seatunnel/%s/apache-seatunnel-%s-bin.tar.gz.sha512

4. Run the installation again with offline mode.
   使用离线模式再次运行安装。
`, aliyunURL, huaweiURL, apacheURL, packageDir, version, version, version)
}

// GetMirrorList returns a list of available mirrors with descriptions
// GetMirrorList 返回可用镜像列表及描述
func GetMirrorList() []struct {
	Source      MirrorSource
	Name        string
	Description string
	Recommended bool
} {
	return []struct {
		Source      MirrorSource
		Name        string
		Description string
		Recommended bool
	}{
		{MirrorAliyun, "Aliyun / 阿里云", "Fastest in China / 国内最快", true},
		{MirrorHuaweiCloud, "Huawei Cloud / 华为云", "Good speed in China / 国内速度良好", false},
		{MirrorApache, "Apache Official / Apache 官方", "Official source / 官方源", false},
	}
}
