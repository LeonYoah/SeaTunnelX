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
	"time"
)

// PluginCategory represents the category of a plugin.
// PluginCategory 表示插件的分类。
type PluginCategory string

const (
	PluginCategorySource    PluginCategory = "source"    // Source / Data source
	PluginCategorySink      PluginCategory = "sink"      // Sink / Data sink
	PluginCategoryTransform PluginCategory = "transform" // 数据转换 / Data transform
)

// PluginStatus represents the status of a plugin.
// PluginStatus 表示插件的状态。
type PluginStatus string

const (
	PluginStatusAvailable PluginStatus = "available" // 可用 / Available
	PluginStatusInstalled PluginStatus = "installed" // 已安装 / Installed
	PluginStatusEnabled   PluginStatus = "enabled"   // 已启用 / Enabled
	PluginStatusDisabled  PluginStatus = "disabled"  // 已禁用 / Disabled
)

// MirrorSource represents the Maven repository mirror source.
// MirrorSource 表示 Maven 仓库镜像源。
type MirrorSource string

const (
	MirrorSourceApache      MirrorSource = "apache"      // Apache 官方仓库
	MirrorSourceAliyun      MirrorSource = "aliyun"      // 阿里云镜像
	MirrorSourceHuaweiCloud MirrorSource = "huaweicloud" // 华为云镜像
)

// MirrorURLs maps mirror sources to their Maven repository base URLs.
// MirrorURLs 将镜像源映射到其 Maven 仓库基础 URL。
var MirrorURLs = map[MirrorSource]string{
	MirrorSourceApache:      "https://repo.maven.apache.org/maven2",
	MirrorSourceAliyun:      "https://maven.aliyun.com/repository/public",
	MirrorSourceHuaweiCloud: "https://repo.huaweicloud.com/repository/maven",
}


// PluginDependency represents a dependency of a plugin.
// PluginDependency 表示插件的依赖项。
type PluginDependency struct {
	GroupID    string `json:"group_id"`    // Maven groupId
	ArtifactID string `json:"artifact_id"` // Maven artifactId
	Version    string `json:"version"`     // 版本号 / Version
	TargetDir  string `json:"target_dir"`  // 目标目录 (connectors/ 或 lib/) / Target directory
}

// Plugin represents a SeaTunnel plugin.
// Plugin 表示一个 SeaTunnel 插件。
type Plugin struct {
	Name         string             `json:"name"`                   // 插件名称 / Plugin name
	DisplayName  string             `json:"display_name"`           // 显示名称 / Display name
	Category     PluginCategory     `json:"category"`               // 分类 / Category
	Version      string             `json:"version"`                // 版本号（与 SeaTunnel 主版本一致）/ Version
	Description  string             `json:"description"`            // 描述 / Description
	GroupID      string             `json:"group_id"`               // Maven groupId
	ArtifactID   string             `json:"artifact_id"`            // Maven artifactId
	Dependencies []PluginDependency `json:"dependencies,omitempty"` // 依赖库列表 / Dependencies
	Icon         string             `json:"icon,omitempty"`         // 图标 URL / Icon URL
	DocURL       string             `json:"doc_url,omitempty"`      // 文档链接 / Documentation URL
}

// InstalledPlugin represents a plugin installed on a cluster (GORM model).
// InstalledPlugin 表示安装在集群上的插件（GORM 模型）。
// Note: Plugins are managed at cluster level, not host level.
// 注意：插件在集群级别管理，而非主机级别。
type InstalledPlugin struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	ClusterID   uint           `gorm:"index;not null" json:"cluster_id"`                             // 集群 ID / Cluster ID
	PluginName  string         `gorm:"size:100;not null;index" json:"plugin_name"`                   // 插件名称 / Plugin name
	ArtifactID  string         `gorm:"size:100" json:"artifact_id"`                                  // Maven artifact ID (e.g., connector-cdc-mysql)
	Category    PluginCategory `gorm:"size:20;not null" json:"category"`                             // 分类 / Category
	Version     string         `gorm:"size:20;not null" json:"version"`                              // 版本号 / Version
	Status      PluginStatus   `gorm:"size:20;not null;default:installed" json:"status"`             // 状态 / Status
	InstallPath string         `gorm:"size:255" json:"install_path"`                                 // 安装路径 / Install path
	InstalledAt time.Time      `gorm:"not null" json:"installed_at"`                                 // 安装时间 / Installed at
	UpdatedAt   time.Time      `json:"updated_at"`                                                   // 更新时间 / Updated at
	InstalledBy uint           `json:"installed_by,omitempty"`                                       // 安装者 ID / Installed by
}

// TableName returns the table name for InstalledPlugin.
// TableName 返回 InstalledPlugin 的表名。
func (InstalledPlugin) TableName() string {
	return "installed_plugins"
}

// PluginFilter represents filter options for querying plugins.
// PluginFilter 表示查询插件的过滤选项。
type PluginFilter struct {
	ClusterID uint           `json:"cluster_id,omitempty"` // 集群 ID / Cluster ID
	Category  PluginCategory `json:"category,omitempty"`   // 分类 / Category
	Status    PluginStatus   `json:"status,omitempty"`     // 状态 / Status
	Keyword   string         `json:"keyword,omitempty"`    // 搜索关键词 / Search keyword
	Page      int            `json:"page,omitempty"`       // 页码 / Page number
	PageSize  int            `json:"page_size,omitempty"`  // 每页数量 / Page size
}

// InstallPluginRequest represents a request to install a plugin.
// InstallPluginRequest 表示安装插件的请求。
type InstallPluginRequest struct {
	PluginName string       `json:"plugin_name" binding:"required"` // 插件名称 / Plugin name
	Version    string       `json:"version" binding:"required"`     // 版本号 / Version
	Mirror     MirrorSource `json:"mirror,omitempty"`               // 镜像源 / Mirror source
}

// PluginInstallStatus represents the installation status of a plugin.
// PluginInstallStatus 表示插件的安装状态。
type PluginInstallStatus struct {
	PluginName string `json:"plugin_name"`          // 插件名称 / Plugin name
	Status     string `json:"status"`               // 状态 / Status
	Progress   int    `json:"progress"`             // 进度 (0-100) / Progress
	Message    string `json:"message,omitempty"`    // 消息 / Message
	Error      string `json:"error,omitempty"`      // 错误信息 / Error message
}

// AvailablePluginsResponse represents the response for listing available plugins.
// AvailablePluginsResponse 表示获取可用插件列表的响应。
type AvailablePluginsResponse struct {
	Plugins []Plugin `json:"plugins"`       // 插件列表 / Plugin list
	Total   int      `json:"total"`         // 总数 / Total count
	Version string   `json:"version"`       // SeaTunnel 版本 / SeaTunnel version
	Mirror  string   `json:"mirror"`        // 当前镜像源 / Current mirror
}


// ==================== Plugin Download Types 插件下载类型 ====================

// PluginDownloadStatus represents the status of a plugin download task.
// PluginDownloadStatus 表示插件下载任务的状态。
type PluginDownloadStatus string

const (
	PluginDownloadPending     PluginDownloadStatus = "pending"     // 等待中 / Pending
	PluginDownloadDownloading PluginDownloadStatus = "downloading" // 下载中 / Downloading
	PluginDownloadCompleted   PluginDownloadStatus = "completed"   // 已完成 / Completed
	PluginDownloadFailed      PluginDownloadStatus = "failed"      // 失败 / Failed
	PluginDownloadCancelled   PluginDownloadStatus = "cancelled"   // 已取消 / Cancelled
)

// PluginDownloadTask represents a plugin download task.
// PluginDownloadTask 表示插件下载任务。
type PluginDownloadTask struct {
	ID              string               `json:"id"`                         // 任务 ID / Task ID
	ClusterID       uint                 `json:"cluster_id"`                 // 集群 ID / Cluster ID
	PluginName      string               `json:"plugin_name"`                // 插件名称 / Plugin name
	Version         string               `json:"version"`                    // 版本号 / Version
	Mirror          MirrorSource         `json:"mirror"`                     // 镜像源 / Mirror source
	Status          PluginDownloadStatus `json:"status"`                     // 状态 / Status
	Progress        int                  `json:"progress"`                   // 进度 (0-100) / Progress
	CurrentStep     string               `json:"current_step,omitempty"`     // 当前步骤 / Current step
	DownloadedBytes int64                `json:"downloaded_bytes,omitempty"` // 已下载字节 / Downloaded bytes
	TotalBytes      int64                `json:"total_bytes,omitempty"`      // 总字节 / Total bytes
	Speed           int64                `json:"speed,omitempty"`            // 下载速度 (bytes/s) / Download speed
	Message         string               `json:"message,omitempty"`          // 消息 / Message
	Error           string               `json:"error,omitempty"`            // 错误信息 / Error message
	StartTime       time.Time            `json:"start_time"`                 // 开始时间 / Start time
	EndTime         *time.Time           `json:"end_time,omitempty"`         // 结束时间 / End time
}

// PluginDownloadRequest represents a request to download/install a plugin.
// PluginDownloadRequest 表示下载/安装插件的请求。
type PluginDownloadRequest struct {
	PluginName string       `json:"plugin_name" binding:"required"` // 插件名称 / Plugin name
	Version    string       `json:"version" binding:"required"`     // 版本号 / Version
	Mirror     MirrorSource `json:"mirror,omitempty"`               // 镜像源 / Mirror source
}

// AvailableVersionsResponse represents the response for listing available versions.
// AvailableVersionsResponse 表示获取可用版本列表的响应。
type AvailableVersionsResponse struct {
	Versions           []string `json:"versions"`            // 版本列表 / Version list
	RecommendedVersion string   `json:"recommended_version"` // 推荐版本 / Recommended version
	Warning            string   `json:"warning,omitempty"`   // 警告信息 / Warning message
}
