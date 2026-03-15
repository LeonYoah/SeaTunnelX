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

package diagnostics

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// CreateAutoPolicy persists one new auto-inspection policy.
// CreateAutoPolicy 持久化一条新的自动巡检策略。
func (r *Repository) CreateAutoPolicy(ctx context.Context, policy *InspectionAutoPolicy) error {
	if r == nil || r.db == nil {
		return ErrDiagnosticsRepositoryUnavailable
	}
	return r.db.WithContext(ctx).Create(policy).Error
}

// UpdateAutoPolicy updates one existing auto-inspection policy.
// UpdateAutoPolicy 更新一条已有的自动巡检策略。
func (r *Repository) UpdateAutoPolicy(ctx context.Context, policy *InspectionAutoPolicy) error {
	if r == nil || r.db == nil {
		return ErrDiagnosticsRepositoryUnavailable
	}
	return r.db.WithContext(ctx).Save(policy).Error
}

// DeleteAutoPolicy deletes one auto-inspection policy by identifier.
// DeleteAutoPolicy 根据标识删除一条自动巡检策略。
func (r *Repository) DeleteAutoPolicy(ctx context.Context, id uint) error {
	if r == nil || r.db == nil {
		return ErrDiagnosticsRepositoryUnavailable
	}
	result := r.db.WithContext(ctx).Delete(&InspectionAutoPolicy{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAutoPolicyNotFound
	}
	return nil
}

// GetAutoPolicyByID retrieves one auto-inspection policy by identifier.
// GetAutoPolicyByID 根据标识获取一条自动巡检策略。
func (r *Repository) GetAutoPolicyByID(ctx context.Context, id uint) (*InspectionAutoPolicy, error) {
	if r == nil || r.db == nil {
		return nil, ErrDiagnosticsRepositoryUnavailable
	}
	var policy InspectionAutoPolicy
	if err := r.db.WithContext(ctx).First(&policy, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAutoPolicyNotFound
		}
		return nil, err
	}
	return &policy, nil
}

// ListAutoPolicies returns paginated auto-inspection policies with optional cluster filter.
// clusterID nil = all, clusterID pointing to 0 = global only.
// ListAutoPolicies 返回分页自动巡检策略，支持可选集群过滤。
// clusterID 为 nil 表示全部，指向 0 表示仅全局策略。
func (r *Repository) ListAutoPolicies(ctx context.Context, clusterID *uint, page, pageSize int) ([]*InspectionAutoPolicy, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, ErrDiagnosticsRepositoryUnavailable
	}

	query := r.db.WithContext(ctx).Model(&InspectionAutoPolicy{})
	if clusterID != nil {
		query = query.Where("cluster_id = ?", *clusterID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize = normalizePagination(page, pageSize)
	var policies []*InspectionAutoPolicy
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&policies).Error; err != nil {
		return nil, 0, err
	}
	return policies, total, nil
}

// ListEnabledPoliciesForCluster returns enabled policies applicable to a cluster.
// It returns policies where ClusterID equals the given clusterID OR ClusterID is 0 (global).
// ListEnabledPoliciesForCluster 返回适用于指定集群的已启用策略。
// 返回 ClusterID 等于给定值或 ClusterID 为 0（全局）的策略。
func (r *Repository) ListEnabledPoliciesForCluster(ctx context.Context, clusterID uint) ([]*InspectionAutoPolicy, error) {
	if r == nil || r.db == nil {
		return nil, ErrDiagnosticsRepositoryUnavailable
	}
	var policies []*InspectionAutoPolicy
	if err := r.db.WithContext(ctx).
		Where("enabled = ? AND (cluster_id = ? OR cluster_id = 0)", true, clusterID).
		Order("cluster_id DESC, id ASC").
		Find(&policies).Error; err != nil {
		return nil, err
	}
	return policies, nil
}

// GetLastInspectionReportForCluster returns the most recent inspection report for a cluster.
// GetLastInspectionReportForCluster 返回指定集群的最近一条巡检报告。
func (r *Repository) GetLastInspectionReportForCluster(ctx context.Context, clusterID uint) (*ClusterInspectionReport, error) {
	if r == nil || r.db == nil {
		return nil, ErrDiagnosticsRepositoryUnavailable
	}
	var report ClusterInspectionReport
	if err := r.db.WithContext(ctx).
		Where("cluster_id = ?", clusterID).
		Order("created_at DESC").
		First(&report).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInspectionReportNotFound
		}
		return nil, err
	}
	return &report, nil
}
