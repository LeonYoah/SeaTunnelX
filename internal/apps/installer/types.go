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
	"time"
)

// MirrorSource represents the download mirror source
// MirrorSource 表示下载镜像源
type MirrorSource string

const (
	MirrorAliyun      MirrorSource = "aliyun"
	MirrorApache      MirrorSource = "apache"
	MirrorHuaweiCloud MirrorSource = "huaweicloud"
)

// InstallMode represents the installation mode
// InstallMode 表示安装模式
type InstallMode string

const (
	InstallModeOnline  InstallMode = "online"
	InstallModeOffline InstallMode = "offline"
)

// DeploymentMode represents the deployment mode
// DeploymentMode 表示部署模式
type DeploymentMode string

const (
	DeploymentModeHybrid    DeploymentMode = "hybrid"
	DeploymentModeSeparated DeploymentMode = "separated"
)

// NodeRole represents the node role
// NodeRole 表示节点角色
type NodeRole string

const (
	NodeRoleMaster NodeRole = "master"
	NodeRoleWorker NodeRole = "worker"
)

// StepStatus represents the status of an installation step
// StepStatus 表示安装步骤的状态
type StepStatus string

const (
	StepStatusPending StepStatus = "pending"
	StepStatusRunning StepStatus = "running"
	StepStatusSuccess StepStatus = "success"
	StepStatusFailed  StepStatus = "failed"
	StepStatusSkipped StepStatus = "skipped"
)

// InstallStep represents an installation step
// InstallStep 表示安装步骤
type InstallStep string

const (
	InstallStepDownload            InstallStep = "download"
	InstallStepVerify              InstallStep = "verify"
	InstallStepExtract             InstallStep = "extract"
	InstallStepConfigureCluster    InstallStep = "configure_cluster"
	InstallStepConfigureCheckpoint InstallStep = "configure_checkpoint"
	InstallStepConfigureJVM        InstallStep = "configure_jvm"
	InstallStepInstallPlugins      InstallStep = "install_plugins"
	InstallStepRegisterCluster     InstallStep = "register_cluster"
	InstallStepComplete            InstallStep = "complete"
)

// CheckStatus represents the status of a precheck item
// CheckStatus 表示预检查项的状态
type CheckStatus string

const (
	CheckStatusPassed  CheckStatus = "passed"
	CheckStatusFailed  CheckStatus = "failed"
	CheckStatusWarning CheckStatus = "warning"
)

// PackageInfo contains information about a SeaTunnel package
// PackageInfo 包含 SeaTunnel 安装包信息
type PackageInfo struct {
	Version      string                  `json:"version"`
	FileName     string                  `json:"file_name"`
	FileSize     int64                   `json:"file_size"`
	Checksum     string                  `json:"checksum,omitempty"`
	DownloadURLs map[MirrorSource]string `json:"download_urls"`
	IsLocal      bool                    `json:"is_local"`
	LocalPath    string                  `json:"local_path,omitempty"`
	UploadedAt   *time.Time              `json:"uploaded_at,omitempty"`
}

// AvailableVersions contains available SeaTunnel versions
// AvailableVersions 包含可用的 SeaTunnel 版本
type AvailableVersions struct {
	Versions           []string      `json:"versions"`
	RecommendedVersion string        `json:"recommended_version"`
	LocalPackages      []PackageInfo `json:"local_packages"`
}

// JVMConfig contains JVM memory configuration
// JVMConfig 包含 JVM 内存配置
type JVMConfig struct {
	HybridHeapSize int `json:"hybrid_heap_size"`
	MasterHeapSize int `json:"master_heap_size"`
	WorkerHeapSize int `json:"worker_heap_size"`
}

// CheckpointStorageType represents the checkpoint storage type
// CheckpointStorageType 表示检查点存储类型
type CheckpointStorageType string

const (
	CheckpointStorageLocalFile CheckpointStorageType = "LOCAL_FILE"
	CheckpointStorageHDFS      CheckpointStorageType = "HDFS"
	CheckpointStorageOSS       CheckpointStorageType = "OSS"
	CheckpointStorageS3        CheckpointStorageType = "S3"
)

// CheckpointConfig contains checkpoint storage configuration
// CheckpointConfig 包含检查点存储配置
type CheckpointConfig struct {
	StorageType      CheckpointStorageType `json:"storage_type"`
	Namespace        string                `json:"namespace"`
	HDFSNameNodeHost string                `json:"hdfs_namenode_host,omitempty"`
	HDFSNameNodePort int                   `json:"hdfs_namenode_port,omitempty"`
	StorageEndpoint  string                `json:"storage_endpoint,omitempty"`
	StorageAccessKey string                `json:"storage_access_key,omitempty"`
	StorageSecretKey string                `json:"storage_secret_key,omitempty"`
	StorageBucket    string                `json:"storage_bucket,omitempty"`
}

// ConnectorConfig contains connector installation configuration
// ConnectorConfig 包含连接器安装配置
type ConnectorConfig struct {
	InstallConnectors bool         `json:"install_connectors"`
	Connectors        []string     `json:"connectors,omitempty"`
	PluginRepo        MirrorSource `json:"plugin_repo,omitempty"`
}

// InstallationRequest is the request for one-click installation
// InstallationRequest 是一键安装的请求
type InstallationRequest struct {
	HostID         string            `json:"host_id"`
	ClusterID      string            `json:"cluster_id"`
	Version        string            `json:"version" binding:"required"`
	InstallMode    InstallMode       `json:"install_mode"`
	Mirror         MirrorSource      `json:"mirror,omitempty"`
	PackagePath    string            `json:"package_path,omitempty"`
	DeploymentMode DeploymentMode    `json:"deployment_mode"`
	NodeRole       NodeRole          `json:"node_role"`
	JVM            *JVMConfig        `json:"jvm,omitempty"`
	Checkpoint     *CheckpointConfig `json:"checkpoint,omitempty"`
	Connector      *ConnectorConfig  `json:"connector,omitempty"`
}

// StepInfo contains information about an installation step
// StepInfo 包含安装步骤的信息
type StepInfo struct {
	Step        InstallStep `json:"step"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Status      StepStatus  `json:"status"`
	Progress    int         `json:"progress"`
	Message     string      `json:"message,omitempty"`
	Error       string      `json:"error,omitempty"`
	StartTime   *time.Time  `json:"start_time,omitempty"`
	EndTime     *time.Time  `json:"end_time,omitempty"`
	Retryable   bool        `json:"retryable"`
}

// InstallationStatus represents the current installation status
// InstallationStatus 表示当前安装状态
type InstallationStatus struct {
	ID          string      `json:"id"`
	HostID      string      `json:"host_id"`
	Status      StepStatus  `json:"status"`
	CurrentStep InstallStep `json:"current_step"`
	Steps       []StepInfo  `json:"steps"`
	Progress    int         `json:"progress"`
	Message     string      `json:"message,omitempty"`
	Error       string      `json:"error,omitempty"`
	StartTime   time.Time   `json:"start_time"`
	EndTime     *time.Time  `json:"end_time,omitempty"`
}

// PrecheckItem represents a single precheck result item
// PrecheckItem 表示单个预检查结果项
type PrecheckItem struct {
	Name    string                 `json:"name"`
	Status  CheckStatus            `json:"status"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// PrecheckResult contains all precheck results
// PrecheckResult 包含所有预检查结果
type PrecheckResult struct {
	Items         []PrecheckItem `json:"items"`
	OverallStatus CheckStatus    `json:"overall_status"`
	Summary       string         `json:"summary"`
}

// DownloadStatus represents the status of a download task
// DownloadStatus 表示下载任务的状态
type DownloadStatus string

const (
	DownloadStatusPending    DownloadStatus = "pending"
	DownloadStatusDownloading DownloadStatus = "downloading"
	DownloadStatusCompleted  DownloadStatus = "completed"
	DownloadStatusFailed     DownloadStatus = "failed"
	DownloadStatusCancelled  DownloadStatus = "cancelled"
)

// DownloadTask represents a package download task
// DownloadTask 表示安装包下载任务
type DownloadTask struct {
	ID              string         `json:"id"`
	Version         string         `json:"version"`
	Mirror          MirrorSource   `json:"mirror"`
	DownloadURL     string         `json:"download_url"`
	Status          DownloadStatus `json:"status"`
	Progress        int            `json:"progress"`          // 0-100
	DownloadedBytes int64          `json:"downloaded_bytes"`
	TotalBytes      int64          `json:"total_bytes"`
	Speed           int64          `json:"speed"`             // bytes per second
	Message         string         `json:"message,omitempty"`
	Error           string         `json:"error,omitempty"`
	StartTime       time.Time      `json:"start_time"`
	EndTime         *time.Time     `json:"end_time,omitempty"`
}

// DownloadRequest represents a request to download a package
// DownloadRequest 表示下载安装包的请求
type DownloadRequest struct {
	Version string       `json:"version" binding:"required"`
	Mirror  MirrorSource `json:"mirror"`
}
