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
	PluginCategorySource    PluginCategory = "source"    // 数据源 / Data source
	PluginCategorySink      PluginCategory = "sink"      // 数据目标 / Data sink
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

// InstalledPlugin represents a plugin installed on a host (GORM model).
// InstalledPlugin 表示安装在主机上的插件（GORM 模型）。
type InstalledPlugin struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	HostID      uint           `gorm:"index;not null" json:"host_id"`                                // 主机 ID / Host ID
	PluginName  string         `gorm:"size:100;not null;index" json:"plugin_name"`                   // 插件名称 / Plugin name
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
	HostID   uint           `json:"host_id,omitempty"`   // 主机 ID / Host ID
	Category PluginCategory `json:"category,omitempty"`  // 分类 / Category
	Status   PluginStatus   `json:"status,omitempty"`    // 状态 / Status
	Keyword  string         `json:"keyword,omitempty"`   // 搜索关键词 / Search keyword
	Page     int            `json:"page,omitempty"`      // 页码 / Page number
	PageSize int            `json:"page_size,omitempty"` // 每页数量 / Page size
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
