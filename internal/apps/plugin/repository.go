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

	"gorm.io/gorm"
)

// Common errors / 常见错误
var (
	ErrPluginNotFound = errors.New("plugin not found / 插件未找到")
)

// Repository provides data access for installed plugins.
// Repository 提供已安装插件的数据访问。
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository instance.
// NewRepository 创建一个新的 Repository 实例。
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create creates a new installed plugin record.
// Create 创建一个新的已安装插件记录。
func (r *Repository) Create(ctx context.Context, plugin *InstalledPlugin) error {
	return r.db.WithContext(ctx).Create(plugin).Error
}

// GetByID retrieves an installed plugin by ID.
// GetByID 通过 ID 获取已安装插件。
func (r *Repository) GetByID(ctx context.Context, id uint) (*InstalledPlugin, error) {
	var plugin InstalledPlugin
	if err := r.db.WithContext(ctx).First(&plugin, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPluginNotFound
		}
		return nil, err
	}
	return &plugin, nil
}


// GetByClusterAndName retrieves an installed plugin by cluster ID and plugin name.
// GetByClusterAndName 通过集群 ID 和插件名称获取已安装插件。
func (r *Repository) GetByClusterAndName(ctx context.Context, clusterID uint, pluginName string) (*InstalledPlugin, error) {
	var plugin InstalledPlugin
	if err := r.db.WithContext(ctx).
		Where("cluster_id = ? AND plugin_name = ?", clusterID, pluginName).
		First(&plugin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPluginNotFound
		}
		return nil, err
	}
	return &plugin, nil
}

// List retrieves installed plugins with optional filters.
// List 获取已安装插件列表，支持可选过滤条件。
func (r *Repository) List(ctx context.Context, filter *PluginFilter) ([]InstalledPlugin, int64, error) {
	var plugins []InstalledPlugin
	var total int64

	query := r.db.WithContext(ctx).Model(&InstalledPlugin{})

	// Apply filters / 应用过滤条件
	if filter != nil {
		if filter.ClusterID > 0 {
			query = query.Where("cluster_id = ?", filter.ClusterID)
		}
		if filter.Category != "" {
			query = query.Where("category = ?", filter.Category)
		}
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
		if filter.Keyword != "" {
			query = query.Where("plugin_name LIKE ?", "%"+filter.Keyword+"%")
		}
	}

	// Count total / 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination / 应用分页
	if filter != nil && filter.PageSize > 0 {
		offset := 0
		if filter.Page > 1 {
			offset = (filter.Page - 1) * filter.PageSize
		}
		query = query.Offset(offset).Limit(filter.PageSize)
	}

	// Execute query / 执行查询
	if err := query.Order("installed_at DESC").Find(&plugins).Error; err != nil {
		return nil, 0, err
	}

	return plugins, total, nil
}

// ListByCluster retrieves all installed plugins for a specific cluster.
// ListByCluster 获取指定集群的所有已安装插件。
func (r *Repository) ListByCluster(ctx context.Context, clusterID uint) ([]InstalledPlugin, error) {
	var plugins []InstalledPlugin
	if err := r.db.WithContext(ctx).
		Where("cluster_id = ?", clusterID).
		Order("installed_at DESC").
		Find(&plugins).Error; err != nil {
		return nil, err
	}
	return plugins, nil
}

// Update updates an installed plugin record.
// Update 更新已安装插件记录。
func (r *Repository) Update(ctx context.Context, plugin *InstalledPlugin) error {
	return r.db.WithContext(ctx).Save(plugin).Error
}

// UpdateStatus updates the status of an installed plugin.
// UpdateStatus 更新已安装插件的状态。
func (r *Repository) UpdateStatus(ctx context.Context, id uint, status PluginStatus) error {
	return r.db.WithContext(ctx).
		Model(&InstalledPlugin{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// Delete deletes an installed plugin record.
// Delete 删除已安装插件记录。
func (r *Repository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&InstalledPlugin{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPluginNotFound
	}
	return nil
}

// DeleteByClusterAndName deletes an installed plugin by cluster ID and plugin name.
// DeleteByClusterAndName 通过集群 ID 和插件名称删除已安装插件。
func (r *Repository) DeleteByClusterAndName(ctx context.Context, clusterID uint, pluginName string) error {
	result := r.db.WithContext(ctx).
		Where("cluster_id = ? AND plugin_name = ?", clusterID, pluginName).
		Delete(&InstalledPlugin{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPluginNotFound
	}
	return nil
}

// ExistsByClusterAndName checks if a plugin is installed on a cluster.
// ExistsByClusterAndName 检查插件是否已安装在集群上。
func (r *Repository) ExistsByClusterAndName(ctx context.Context, clusterID uint, pluginName string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&InstalledPlugin{}).
		Where("cluster_id = ? AND plugin_name = ?", clusterID, pluginName).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ==================== Plugin Dependency Config Methods 插件依赖配置方法 ====================

// ListDependencies retrieves all dependencies for a plugin.
// ListDependencies 获取插件的所有依赖。
func (r *Repository) ListDependencies(ctx context.Context, pluginName string) ([]PluginDependencyConfig, error) {
	var deps []PluginDependencyConfig
	if err := r.db.WithContext(ctx).
		Where("plugin_name = ?", pluginName).
		Order("created_at ASC").
		Find(&deps).Error; err != nil {
		return nil, err
	}
	return deps, nil
}

// CreateDependency creates a new dependency configuration.
// CreateDependency 创建新的依赖配置。
func (r *Repository) CreateDependency(ctx context.Context, dep *PluginDependencyConfig) error {
	return r.db.WithContext(ctx).Create(dep).Error
}

// DeleteDependency deletes a dependency configuration by ID.
// DeleteDependency 通过 ID 删除依赖配置。
func (r *Repository) DeleteDependency(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&PluginDependencyConfig{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("dependency not found / 依赖未找到")
	}
	return nil
}

// GetDependencyByID retrieves a dependency by ID.
// GetDependencyByID 通过 ID 获取依赖。
func (r *Repository) GetDependencyByID(ctx context.Context, id uint) (*PluginDependencyConfig, error) {
	var dep PluginDependencyConfig
	if err := r.db.WithContext(ctx).First(&dep, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("dependency not found / 依赖未找到")
		}
		return nil, err
	}
	return &dep, nil
}
