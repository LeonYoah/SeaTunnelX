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
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// CreateDiagnosticTask persists one diagnostics task.
// CreateDiagnosticTask 持久化一条诊断任务。
func (r *Repository) CreateDiagnosticTask(ctx context.Context, task *DiagnosticTask) error {
	if r == nil || r.db == nil {
		return ErrDiagnosticsRepositoryUnavailable
	}
	return r.db.WithContext(ctx).Create(task).Error
}

// GetDiagnosticTaskByID fetches one diagnostics task with its related steps and node executions.
// GetDiagnosticTaskByID 获取一条诊断任务及其关联步骤和节点执行。
func (r *Repository) GetDiagnosticTaskByID(ctx context.Context, id uint) (*DiagnosticTask, error) {
	if r == nil || r.db == nil {
		return nil, ErrDiagnosticsRepositoryUnavailable
	}
	var task DiagnosticTask
	query := r.db.WithContext(ctx).
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("sequence ASC")
		}).
		Preload("NodeExecutions", func(db *gorm.DB) *gorm.DB {
			return db.Order("host_id ASC, role ASC")
		})
	if err := query.First(&task, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDiagnosticTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

// UpdateDiagnosticTask updates one diagnostics task.
// UpdateDiagnosticTask 更新一条诊断任务。
func (r *Repository) UpdateDiagnosticTask(ctx context.Context, task *DiagnosticTask) error {
	if r == nil || r.db == nil {
		return ErrDiagnosticsRepositoryUnavailable
	}
	result := r.db.WithContext(ctx).Save(task)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDiagnosticTaskNotFound
	}
	return nil
}

// GetLatestDiagnosticTaskByInspectionReportID returns the most recent diagnostic task
// whose source_ref.inspection_report_id equals reportID, with steps and node executions preloaded.
// Returns nil, nil when no such task exists.
func (r *Repository) GetLatestDiagnosticTaskByInspectionReportID(ctx context.Context, reportID uint) (*DiagnosticTask, error) {
	if r == nil || r.db == nil {
		return nil, ErrDiagnosticsRepositoryUnavailable
	}
	if reportID == 0 {
		return nil, nil
	}
	dialector := r.db.Dialector.Name()
	var query *gorm.DB
	switch dialector {
	case "sqlite":
		query = r.db.WithContext(ctx).Where(
			"trigger_source = ? AND json_extract(source_ref, '$.inspection_report_id') = ?",
			DiagnosticTaskSourceInspectionFinding, reportID,
		)
	case "mysql":
		query = r.db.WithContext(ctx).Where(
			"trigger_source = ? AND CAST(JSON_UNQUOTE(JSON_EXTRACT(source_ref, '$.inspection_report_id')) AS UNSIGNED) = ?",
			DiagnosticTaskSourceInspectionFinding, reportID,
		)
	default:
		query = r.db.WithContext(ctx).Where(
			"trigger_source = ? AND (source_ref->>'inspection_report_id')::bigint = ?",
			DiagnosticTaskSourceInspectionFinding, reportID,
		)
	}
	var task DiagnosticTask
	err := query.
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("sequence ASC")
		}).
		Preload("NodeExecutions", func(db *gorm.DB) *gorm.DB {
			return db.Order("host_id ASC, role ASC")
		}).
		Order("created_at DESC").
		First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest diagnostic task by inspection_report_id: %w", err)
	}
	return &task, nil
}

// ListDiagnosticTasks queries diagnostics task summaries with filters and pagination.
// ListDiagnosticTasks 按过滤条件分页查询诊断任务摘要。
func (r *Repository) ListDiagnosticTasks(ctx context.Context, filter *DiagnosticTaskListFilter) ([]*DiagnosticTaskSummary, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, ErrDiagnosticsRepositoryUnavailable
	}
	query := r.db.WithContext(ctx).Model(&DiagnosticTask{})
	if filter != nil {
		if filter.ClusterID > 0 {
			query = query.Where("cluster_id = ?", filter.ClusterID)
		}
		if status := strings.TrimSpace(string(filter.Status)); status != "" {
			query = query.Where("status = ?", status)
		}
		if triggerSource := strings.TrimSpace(string(filter.TriggerSource)); triggerSource != "" {
			query = query.Where("trigger_source = ?", triggerSource)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := 1, 10
	if filter != nil {
		page, pageSize = normalizePagination(filter.Page, filter.PageSize)
	}

	items := make([]*DiagnosticTaskSummary, 0)
	err := query.
		Select([]string{
			"id",
			"cluster_id",
			"trigger_source",
			"status",
			"current_step",
			"failure_step",
			"failure_reason",
			"summary",
			"started_at",
			"completed_at",
			"created_by",
			"created_by_name",
			"created_at",
			"updated_at",
		}).
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error
	return items, total, err
}

// CreateDiagnosticTaskSteps creates diagnostics task steps in batch.
// CreateDiagnosticTaskSteps 批量创建诊断任务步骤。
func (r *Repository) CreateDiagnosticTaskSteps(ctx context.Context, steps []*DiagnosticTaskStep) error {
	if r == nil || r.db == nil {
		return ErrDiagnosticsRepositoryUnavailable
	}
	if len(steps) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&steps).Error
}

// UpdateDiagnosticTaskStep updates one diagnostics task step.
// UpdateDiagnosticTaskStep 更新一条诊断任务步骤。
func (r *Repository) UpdateDiagnosticTaskStep(ctx context.Context, step *DiagnosticTaskStep) error {
	if r == nil || r.db == nil {
		return ErrDiagnosticsRepositoryUnavailable
	}
	result := r.db.WithContext(ctx).Save(step)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDiagnosticTaskStepNotFound
	}
	return nil
}

// ListDiagnosticTaskSteps lists steps of one diagnostics task.
// ListDiagnosticTaskSteps 获取单个诊断任务的步骤列表。
func (r *Repository) ListDiagnosticTaskSteps(ctx context.Context, taskID uint) ([]*DiagnosticTaskStep, error) {
	if r == nil || r.db == nil {
		return nil, ErrDiagnosticsRepositoryUnavailable
	}
	var steps []*DiagnosticTaskStep
	err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("sequence ASC").Find(&steps).Error
	return steps, err
}

// CreateDiagnosticNodeExecutions creates diagnostics node execution records in batch.
// CreateDiagnosticNodeExecutions 批量创建诊断节点执行记录。
func (r *Repository) CreateDiagnosticNodeExecutions(ctx context.Context, nodes []*DiagnosticNodeExecution) error {
	if r == nil || r.db == nil {
		return ErrDiagnosticsRepositoryUnavailable
	}
	if len(nodes) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&nodes).Error
}

// UpdateDiagnosticNodeExecution updates one diagnostics node execution.
// UpdateDiagnosticNodeExecution 更新一条诊断节点执行记录。
func (r *Repository) UpdateDiagnosticNodeExecution(ctx context.Context, node *DiagnosticNodeExecution) error {
	if r == nil || r.db == nil {
		return ErrDiagnosticsRepositoryUnavailable
	}
	result := r.db.WithContext(ctx).Save(node)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDiagnosticNodeExecutionNotFound
	}
	return nil
}

// ListDiagnosticNodeExecutions lists node executions of one diagnostics task.
// ListDiagnosticNodeExecutions 获取单个诊断任务的节点执行列表。
func (r *Repository) ListDiagnosticNodeExecutions(ctx context.Context, taskID uint) ([]*DiagnosticNodeExecution, error) {
	if r == nil || r.db == nil {
		return nil, ErrDiagnosticsRepositoryUnavailable
	}
	var nodes []*DiagnosticNodeExecution
	err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("host_id ASC, role ASC").Find(&nodes).Error
	return nodes, err
}

// CreateDiagnosticStepLog creates one diagnostics step log.
// CreateDiagnosticStepLog 创建一条诊断步骤日志。
func (r *Repository) CreateDiagnosticStepLog(ctx context.Context, log *DiagnosticStepLog) error {
	if r == nil || r.db == nil {
		return ErrDiagnosticsRepositoryUnavailable
	}
	return r.db.WithContext(ctx).Create(log).Error
}

// ListDiagnosticStepLogs queries diagnostics task logs with filters and pagination.
// ListDiagnosticStepLogs 按过滤条件分页查询诊断任务日志。
func (r *Repository) ListDiagnosticStepLogs(ctx context.Context, filter *DiagnosticTaskLogFilter) ([]*DiagnosticStepLog, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, ErrDiagnosticsRepositoryUnavailable
	}
	query := r.db.WithContext(ctx).Model(&DiagnosticStepLog{})
	if filter != nil {
		if filter.TaskID > 0 {
			query = query.Where("task_id = ?", filter.TaskID)
		}
		if filter.StepCode != "" {
			query = query.Where("step_code = ?", filter.StepCode)
		}
		if filter.NodeExecutionID != nil {
			query = query.Where("node_execution_id = ?", *filter.NodeExecutionID)
		}
		if filter.Level != "" {
			query = query.Where("level = ?", filter.Level)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := 1, 50
	if filter != nil {
		page, pageSize = normalizePagination(filter.Page, filter.PageSize)
	}

	logs := make([]*DiagnosticStepLog, 0)
	err := query.
		Order("created_at ASC, id ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs).Error
	return logs, total, err
}
