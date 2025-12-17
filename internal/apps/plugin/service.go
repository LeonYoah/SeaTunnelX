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
	"time"
)

// Common service errors / 常见服务错误
var (
	ErrInvalidVersion     = errors.New("invalid version / 无效的版本号")
	ErrInvalidMirror      = errors.New("invalid mirror source / 无效的镜像源")
	ErrPluginNotAvailable = errors.New("plugin not available / 插件不可用")
	ErrHostNotFound       = errors.New("host not found / 主机未找到")
)

// Service provides plugin management functionality.
// Service 提供插件管理功能。
type Service struct {
	repo *Repository
	// agentManager is used to communicate with agents for plugin installation
	// agentManager 用于与 Agent 通信进行插件安装
	// agentManager *agent.Manager // TODO: inject agent manager
}

// NewService creates a new Service instance.
// NewService 创建一个新的 Service 实例。
func NewService(repo *Repository) *Service {
	return &Service{
		repo: repo,
	}
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

	// Get predefined plugin list for the version
	// 获取该版本的预定义插件列表
	plugins := getAvailablePluginsForVersion(version)

	return &AvailablePluginsResponse{
		Plugins: plugins,
		Total:   len(plugins),
		Version: version,
		Mirror:  string(mirror),
	}, nil
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

// ListInstalledPlugins returns installed plugins for a host.
// ListInstalledPlugins 返回主机上已安装的插件列表。
func (s *Service) ListInstalledPlugins(ctx context.Context, hostID uint) ([]InstalledPlugin, error) {
	return s.repo.ListByHost(ctx, hostID)
}

// GetInstalledPlugin returns an installed plugin by host and name.
// GetInstalledPlugin 通过主机和名称获取已安装插件。
func (s *Service) GetInstalledPlugin(ctx context.Context, hostID uint, pluginName string) (*InstalledPlugin, error) {
	return s.repo.GetByHostAndName(ctx, hostID, pluginName)
}

// ==================== Plugin Installation 插件安装 ====================

// InstallPlugin installs a plugin on a host via Agent.
// InstallPlugin 通过 Agent 在主机上安装插件。
func (s *Service) InstallPlugin(ctx context.Context, hostID uint, req *InstallPluginRequest) (*InstalledPlugin, error) {
	// Check if plugin already installed / 检查插件是否已安装
	exists, err := s.repo.ExistsByHostAndName(ctx, hostID, req.PluginName)
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
		HostID:      hostID,
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

// UninstallPlugin uninstalls a plugin from a host.
// UninstallPlugin 从主机上卸载插件。
func (s *Service) UninstallPlugin(ctx context.Context, hostID uint, pluginName string) error {
	// Check if plugin exists / 检查插件是否存在
	_, err := s.repo.GetByHostAndName(ctx, hostID, pluginName)
	if err != nil {
		return err
	}

	// TODO: Send uninstall command to Agent via gRPC
	// TODO: 通过 gRPC 向 Agent 发送卸载命令

	// Delete installed plugin record / 删除已安装插件记录
	return s.repo.DeleteByHostAndName(ctx, hostID, pluginName)
}

// EnablePlugin enables an installed plugin.
// EnablePlugin 启用已安装的插件。
func (s *Service) EnablePlugin(ctx context.Context, hostID uint, pluginName string) (*InstalledPlugin, error) {
	plugin, err := s.repo.GetByHostAndName(ctx, hostID, pluginName)
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
func (s *Service) DisablePlugin(ctx context.Context, hostID uint, pluginName string) (*InstalledPlugin, error) {
	plugin, err := s.repo.GetByHostAndName(ctx, hostID, pluginName)
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
