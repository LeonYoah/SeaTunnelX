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

package monitor

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Repository provides data access for monitor configurations and events.
// Repository 提供监控配置和事件的数据访问。
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new monitor repository.
// NewRepository 创建新的监控仓库。
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ==================== MonitorConfig CRUD 监控配置增删改查 ====================

// CreateConfig creates a new monitor configuration.
// CreateConfig 创建新的监控配置。
func (r *Repository) CreateConfig(ctx context.Context, config *MonitorConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

// GetConfigByClusterID retrieves monitor config by cluster ID.
// GetConfigByClusterID 根据集群 ID 获取监控配置。
func (r *Repository) GetConfigByClusterID(ctx context.Context, clusterID uint) (*MonitorConfig, error) {
	var config MonitorConfig
	err := r.db.WithContext(ctx).Where("cluster_id = ?", clusterID).First(&config).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}
	return &config, nil
}

// UpdateConfig updates an existing monitor configuration.
// UpdateConfig 更新现有的监控配置。
func (r *Repository) UpdateConfig(ctx context.Context, config *MonitorConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

// DeleteConfigByClusterID deletes monitor config by cluster ID.
// DeleteConfigByClusterID 根据集群 ID 删除监控配置。
func (r *Repository) DeleteConfigByClusterID(ctx context.Context, clusterID uint) error {
	return r.db.WithContext(ctx).Where("cluster_id = ?", clusterID).Delete(&MonitorConfig{}).Error
}

// ==================== ProcessEvent CRUD 进程事件增删改查 ====================

// CreateEvent creates a new process event.
// CreateEvent 创建新的进程事件。
func (r *Repository) CreateEvent(ctx context.Context, event *ProcessEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// GetEventByID retrieves a process event by ID.
// GetEventByID 根据 ID 获取进程事件。
func (r *Repository) GetEventByID(ctx context.Context, id uint) (*ProcessEvent, error) {
	var event ProcessEvent
	err := r.db.WithContext(ctx).First(&event, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrEventNotFound
		}
		return nil, err
	}
	return &event, nil
}

// ListEvents retrieves process events with filtering and pagination.
// ListEvents 获取带过滤和分页的进程事件列表。
func (r *Repository) ListEvents(ctx context.Context, filter *ProcessEventFilter) ([]*ProcessEvent, int64, error) {
	var events []*ProcessEvent
	var total int64

	query := r.db.WithContext(ctx).Model(&ProcessEvent{})

	// Apply filters / 应用过滤条件
	if filter.ClusterID > 0 {
		query = query.Where("cluster_id = ?", filter.ClusterID)
	}
	if filter.NodeID > 0 {
		query = query.Where("node_id = ?", filter.NodeID)
	}
	if filter.HostID > 0 {
		query = query.Where("host_id = ?", filter.HostID)
	}
	if filter.EventType != "" {
		query = query.Where("event_type = ?", filter.EventType)
	}
	if filter.StartTime != nil {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("created_at <= ?", filter.EndTime)
	}

	// Count total / 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination / 应用分页
	if filter.Page > 0 && filter.PageSize > 0 {
		offset := (filter.Page - 1) * filter.PageSize
		query = query.Offset(offset).Limit(filter.PageSize)
	}

	// Order by created_at desc / 按创建时间倒序
	query = query.Order("created_at DESC")

	if err := query.Find(&events).Error; err != nil {
		return nil, 0, err
	}

	return events, total, nil
}

// ListEventsByClusterID retrieves all events for a cluster.
// ListEventsByClusterID 获取集群的所有事件。
func (r *Repository) ListEventsByClusterID(ctx context.Context, clusterID uint, limit int) ([]*ProcessEvent, error) {
	var events []*ProcessEvent
	query := r.db.WithContext(ctx).Where("cluster_id = ?", clusterID).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// DeleteEventsByClusterID deletes all events for a cluster.
// DeleteEventsByClusterID 删除集群的所有事件。
func (r *Repository) DeleteEventsByClusterID(ctx context.Context, clusterID uint) error {
	return r.db.WithContext(ctx).Where("cluster_id = ?", clusterID).Delete(&ProcessEvent{}).Error
}

// GetLatestEventByNodeID retrieves the latest event for a node.
// GetLatestEventByNodeID 获取节点的最新事件。
func (r *Repository) GetLatestEventByNodeID(ctx context.Context, nodeID uint) (*ProcessEvent, error) {
	var event ProcessEvent
	err := r.db.WithContext(ctx).Where("node_id = ?", nodeID).Order("created_at DESC").First(&event).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &event, nil
}

// CountEventsByType counts events by type for a cluster within a time range.
// CountEventsByType 统计集群在时间范围内按类型分组的事件数量。
func (r *Repository) CountEventsByType(ctx context.Context, clusterID uint, eventType ProcessEventType, since *time.Time) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&ProcessEvent{}).Where("cluster_id = ? AND event_type = ?", clusterID, eventType)
	if since != nil {
		query = query.Where("created_at >= ?", since)
	}
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
