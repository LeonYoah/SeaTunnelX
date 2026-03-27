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

package sync

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// Repository provides persistence operations for sync tasks and instances.
type Repository struct {
	db *gorm.DB
}

// DeleteTaskVersionsByTaskIDs deletes snapshots for the provided task ids.
func (r *Repository) DeleteTaskVersionsByTaskIDs(ctx context.Context, taskIDs []uint) error {
	if len(taskIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("task_id IN ?", taskIDs).Delete(&TaskVersion{}).Error
}

// DeleteJobInstancesByTaskIDs deletes run history for the provided task ids.
func (r *Repository) DeleteJobInstancesByTaskIDs(ctx context.Context, taskIDs []uint) error {
	if len(taskIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("task_id IN ?", taskIDs).Delete(&JobInstance{}).Error
}

// DeleteTasksByIDs deletes the provided workspace nodes.
func (r *Repository) DeleteTasksByIDs(ctx context.Context, taskIDs []uint) error {
	if len(taskIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("id IN ?", taskIDs).Delete(&Task{}).Error
}

// NewRepository creates a new sync repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Transaction executes fn inside one database transaction.
func (r *Repository) Transaction(ctx context.Context, fn func(tx *Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Repository{db: tx})
	})
}

// CreateTask creates a sync task.
func (r *Repository) CreateTask(ctx context.Context, task *Task) error {
	return r.db.WithContext(ctx).Create(task).Error
}

// GetTaskByID returns one task by id.
func (r *Repository) GetTaskByID(ctx context.Context, id uint) (*Task, error) {
	var task Task
	if err := r.db.WithContext(ctx).First(&task, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

// ListTasks lists tasks with filter and pagination.
func (r *Repository) ListTasks(ctx context.Context, filter *TaskFilter) ([]*Task, int64, error) {
	query := r.db.WithContext(ctx).Model(&Task{})
	if filter != nil {
		if filter.Name != "" {
			query = query.Where("name LIKE ?", "%"+filter.Name+"%")
		}
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter != nil && filter.Size > 0 {
		offset := 0
		if filter.Page > 1 {
			offset = (filter.Page - 1) * filter.Size
		}
		query = query.Offset(offset).Limit(filter.Size)
	}

	var tasks []*Task
	if err := query.Order("node_type ASC").Order("sort_order ASC").Order("updated_at DESC").Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

// ListAllTasks returns all nodes for tree building.
func (r *Repository) ListAllTasks(ctx context.Context) ([]*Task, error) {
	var tasks []*Task
	if err := r.db.WithContext(ctx).
		Order("sort_order ASC").
		Order("CASE WHEN parent_id IS NULL THEN 0 ELSE 1 END ASC").
		Order("node_type ASC").
		Order("name ASC").
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// UpdateTask updates a sync task.
func (r *Repository) UpdateTask(ctx context.Context, task *Task) error {
	result := r.db.WithContext(ctx).Save(task)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTaskNotFound
	}
	return nil
}

// ExistsSiblingTaskName reports whether the same folder already contains a node with the provided name.
// ExistsSiblingTaskName 返回同级目录下是否已存在同名节点。
func (r *Repository) ExistsSiblingTaskName(ctx context.Context, parentID *uint, name string, excludeID *uint) (bool, error) {
	query := r.db.WithContext(ctx).Model(&Task{}).Where("name = ?", strings.TrimSpace(name))
	if parentID == nil || *parentID == 0 {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}
	if excludeID != nil && *excludeID > 0 {
		query = query.Where("id <> ?", *excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateTaskVersion stores one immutable task snapshot.
func (r *Repository) CreateTaskVersion(ctx context.Context, version *TaskVersion) error {
	return r.db.WithContext(ctx).Create(version).Error
}

// ListTaskVersionsByTaskID lists immutable snapshots for one task.
func (r *Repository) ListTaskVersionsByTaskID(ctx context.Context, taskID uint) ([]*TaskVersion, error) {
	items, _, err := r.ListTaskVersionsByTaskIDPaginated(ctx, taskID, 0, 0)
	return items, err
}

// ListTaskVersionsByTaskIDPaginated lists immutable snapshots for one task with pagination.
func (r *Repository) ListTaskVersionsByTaskIDPaginated(ctx context.Context, taskID uint, page, size int) ([]*TaskVersion, int64, error) {
	query := r.db.WithContext(ctx).Model(&TaskVersion{}).Where("task_id = ?", taskID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var versions []*TaskVersion
	if size > 0 {
		offset := 0
		if page > 1 {
			offset = (page - 1) * size
		}
		query = query.Offset(offset).Limit(size)
	}
	if err := query.Order("version DESC, created_at DESC").Find(&versions).Error; err != nil {
		return nil, 0, err
	}
	return versions, total, nil
}

// GetTaskVersionByID returns one immutable task snapshot.
func (r *Repository) GetTaskVersionByID(ctx context.Context, taskID uint, versionID uint) (*TaskVersion, error) {
	var version TaskVersion
	if err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		First(&version, versionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskVersionNotFound
		}
		return nil, err
	}
	return &version, nil
}

// DeleteTaskVersion removes one immutable task snapshot.
func (r *Repository) DeleteTaskVersion(ctx context.Context, taskID uint, versionID uint) error {
	result := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Delete(&TaskVersion{}, versionID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTaskVersionNotFound
	}
	return nil
}

// CreateJobInstance creates a new job instance record.
func (r *Repository) CreateJobInstance(ctx context.Context, instance *JobInstance) error {
	return r.db.WithContext(ctx).Create(instance).Error
}

// GetJobInstanceByID returns one job instance by id.
func (r *Repository) GetJobInstanceByID(ctx context.Context, id uint) (*JobInstance, error) {
	var instance JobInstance
	if err := r.db.WithContext(ctx).First(&instance, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJobInstanceNotFound
		}
		return nil, err
	}
	return &instance, nil
}

// ListJobInstances lists job instances with filter and pagination.
func (r *Repository) ListJobInstances(ctx context.Context, filter *JobFilter) ([]*JobInstance, int64, error) {
	query := r.db.WithContext(ctx).Model(&JobInstance{})
	if filter != nil {
		if filter.TaskID > 0 {
			query = query.Where("task_id = ?", filter.TaskID)
		}
		if filter.RunType != "" {
			query = query.Where("run_type = ?", filter.RunType)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter != nil && filter.Size > 0 {
		offset := 0
		if filter.Page > 1 {
			offset = (filter.Page - 1) * filter.Size
		}
		query = query.Offset(offset).Limit(filter.Size)
	}

	var instances []*JobInstance
	if err := query.Order("created_at DESC").Find(&instances).Error; err != nil {
		return nil, 0, err
	}
	return instances, total, nil
}

// GetPreviewJobInstanceByPlatformOrEngineJobID retrieves one preview job by platform or engine job id.
func (r *Repository) GetPreviewJobInstanceByPlatformOrEngineJobID(ctx context.Context, platformJobID string, engineJobID string) (*JobInstance, error) {
	query := r.db.WithContext(ctx).Where("run_type = ?", RunTypePreview)
	if strings.TrimSpace(platformJobID) != "" {
		query = query.Where("platform_job_id = ?", strings.TrimSpace(platformJobID))
	} else if strings.TrimSpace(engineJobID) != "" {
		query = query.Where("engine_job_id = ?", strings.TrimSpace(engineJobID))
	} else {
		return nil, ErrJobInstanceNotFound
	}
	var instance JobInstance
	if err := query.Order("created_at DESC").First(&instance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJobInstanceNotFound
		}
		return nil, err
	}
	return &instance, nil
}

// UpdateJobInstance updates one job instance.
func (r *Repository) UpdateJobInstance(ctx context.Context, instance *JobInstance) error {
	result := r.db.WithContext(ctx).Save(instance)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrJobInstanceNotFound
	}
	return nil
}

// ListGlobalVariables returns all global variables ordered by key.
func (r *Repository) ListGlobalVariables(ctx context.Context) ([]*GlobalVariable, error) {
	items, _, err := r.ListGlobalVariablesPaginated(ctx, 0, 0)
	return items, err
}

// ListGlobalVariablesPaginated returns global variables ordered by key with pagination.
func (r *Repository) ListGlobalVariablesPaginated(ctx context.Context, page, size int) ([]*GlobalVariable, int64, error) {
	query := r.db.WithContext(ctx).Model(&GlobalVariable{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []*GlobalVariable
	if size > 0 {
		offset := 0
		if page > 1 {
			offset = (page - 1) * size
		}
		query = query.Offset(offset).Limit(size)
	}
	if err := query.Order("key ASC").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetGlobalVariableByID returns one global variable by id.
func (r *Repository) GetGlobalVariableByID(ctx context.Context, id uint) (*GlobalVariable, error) {
	var item GlobalVariable
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGlobalVariableNotFound
		}
		return nil, err
	}
	return &item, nil
}

// GetGlobalVariableByKey returns one global variable by key.
func (r *Repository) GetGlobalVariableByKey(ctx context.Context, key string) (*GlobalVariable, error) {
	var item GlobalVariable
	if err := r.db.WithContext(ctx).Where("key = ?", strings.TrimSpace(key)).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGlobalVariableNotFound
		}
		return nil, err
	}
	return &item, nil
}

// CreateGlobalVariable creates one global variable.
func (r *Repository) CreateGlobalVariable(ctx context.Context, item *GlobalVariable) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// UpdateGlobalVariable updates one global variable.
func (r *Repository) UpdateGlobalVariable(ctx context.Context, item *GlobalVariable) error {
	result := r.db.WithContext(ctx).Save(item)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGlobalVariableNotFound
	}
	return nil
}

// DeleteGlobalVariable deletes one global variable.
func (r *Repository) DeleteGlobalVariable(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&GlobalVariable{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGlobalVariableNotFound
	}
	return nil
}
