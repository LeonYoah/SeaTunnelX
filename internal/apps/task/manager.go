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

package task

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors
var (
	ErrTaskNotFound      = errors.New("task not found / 任务未找到")
	ErrAgentNotConnected = errors.New("agent not connected / Agent 未连接")
	ErrTaskCancelled     = errors.New("task cancelled / 任务已取消")
	ErrTaskTimeout       = errors.New("task timeout / 任务超时")
)

// ProgressCallback is called when task progress is updated
// ProgressCallback 在任务进度更新时被调用
type ProgressCallback func(progress *TaskProgress)

// Manager manages tasks for Control Plane to Agent communication
// Manager 管理 Control Plane 到 Agent 通信的任务
type Manager struct {
	// tasks stores all tasks by ID
	// tasks 按 ID 存储所有任务
	tasks map[string]*Task
	mu    sync.RWMutex

	// hostTasks stores task IDs by host ID for quick lookup
	// hostTasks 按主机 ID 存储任务 ID，用于快速查找
	hostTasks map[uint][]string

	// progressCallbacks stores callbacks for task progress updates
	// progressCallbacks 存储任务进度更新的回调
	progressCallbacks map[string][]ProgressCallback
	callbackMu        sync.RWMutex

	// agentManager is used to send commands to agents
	// agentManager 用于向 Agent 发送命令
	// agentManager *agent.Manager // TODO: inject agent manager
}

// NewManager creates a new task Manager
// NewManager 创建新的任务管理器
func NewManager() *Manager {
	return &Manager{
		tasks:             make(map[string]*Task),
		hostTasks:         make(map[uint][]string),
		progressCallbacks: make(map[string][]ProgressCallback),
	}
}

// CreateTask creates a new task and returns it
// CreateTask 创建新任务并返回
func (m *Manager) CreateTask(ctx context.Context, req *CreateTaskRequest, createdBy string) (*Task, error) {
	task := &Task{
		ID:             uuid.New().String(),
		Type:           req.Type,
		HostID:         req.HostID,
		Status:         TaskStatusPending,
		Progress:       0,
		Message:        "任务已创建 / Task created",
		Params:         req.Params,
		CreatedAt:      time.Now(),
		TimeoutSeconds: req.TimeoutSeconds,
		Retryable:      true,
		MaxRetries:     3,
		CreatedBy:      createdBy,
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.hostTasks[req.HostID] = append(m.hostTasks[req.HostID], task.ID)
	m.mu.Unlock()

	return task, nil
}

// GetTask returns a task by ID
// GetTask 根据 ID 返回任务
func (m *Manager) GetTask(ctx context.Context, taskID string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}

	return task, nil
}

// ListTasks returns tasks for a host
// ListTasks 返回主机的任务列表
func (m *Manager) ListTasks(ctx context.Context, hostID uint, limit int) ([]*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	taskIDs, ok := m.hostTasks[hostID]
	if !ok {
		return []*Task{}, nil
	}

	tasks := make([]*Task, 0, len(taskIDs))
	for _, id := range taskIDs {
		if task, ok := m.tasks[id]; ok {
			tasks = append(tasks, task)
		}
	}

	// Sort by created time descending and limit
	// 按创建时间降序排序并限制数量
	if limit > 0 && len(tasks) > limit {
		tasks = tasks[len(tasks)-limit:]
	}

	return tasks, nil
}

// ListAllTasks returns all tasks with optional filtering
// ListAllTasks 返回所有任务，支持可选过滤
func (m *Manager) ListAllTasks(ctx context.Context, status *TaskStatus, taskType *TaskType, limit int) ([]*Task, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*Task, 0)
	for _, task := range m.tasks {
		// Filter by status
		if status != nil && task.Status != *status {
			continue
		}
		// Filter by type
		if taskType != nil && task.Type != *taskType {
			continue
		}
		tasks = append(tasks, task)
	}

	total := len(tasks)

	// Limit results
	if limit > 0 && len(tasks) > limit {
		tasks = tasks[:limit]
	}

	return tasks, total, nil
}

// StartTask starts executing a task
// StartTask 开始执行任务
func (m *Manager) StartTask(ctx context.Context, taskID string) error {
	m.mu.Lock()
	task, ok := m.tasks[taskID]
	if !ok {
		m.mu.Unlock()
		return ErrTaskNotFound
	}

	now := time.Now()
	task.Status = TaskStatusRunning
	task.StartedAt = &now
	task.Message = "任务开始执行 / Task started"
	m.mu.Unlock()

	// TODO: Send command to Agent via gRPC
	// TODO: 通过 gRPC 向 Agent 发送命令

	// Notify progress callbacks
	m.notifyProgress(&TaskProgress{
		TaskID:    taskID,
		Status:    TaskStatusRunning,
		Progress:  0,
		Message:   task.Message,
		Timestamp: now,
	})

	return nil
}

// UpdateProgress updates task progress from Agent
// UpdateProgress 更新来自 Agent 的任务进度
func (m *Manager) UpdateProgress(ctx context.Context, progress *TaskProgress) error {
	m.mu.Lock()
	task, ok := m.tasks[progress.TaskID]
	if !ok {
		m.mu.Unlock()
		return ErrTaskNotFound
	}

	task.Status = progress.Status
	task.Progress = progress.Progress
	task.Message = progress.Message

	if progress.Error != "" {
		task.Error = progress.Error
	}

	// Update completion time if task is done
	if progress.Status == TaskStatusSuccess || progress.Status == TaskStatusFailed ||
		progress.Status == TaskStatusCancelled || progress.Status == TaskStatusTimeout {
		now := time.Now()
		task.CompletedAt = &now
	}

	m.mu.Unlock()

	// Notify progress callbacks
	m.notifyProgress(progress)

	return nil
}

// CancelTask cancels a running task
// CancelTask 取消正在运行的任务
func (m *Manager) CancelTask(ctx context.Context, taskID string) error {
	m.mu.Lock()
	task, ok := m.tasks[taskID]
	if !ok {
		m.mu.Unlock()
		return ErrTaskNotFound
	}

	if task.Status != TaskStatusPending && task.Status != TaskStatusRunning {
		m.mu.Unlock()
		return nil // Already completed
	}

	now := time.Now()
	task.Status = TaskStatusCancelled
	task.Message = "任务已取消 / Task cancelled"
	task.CompletedAt = &now
	m.mu.Unlock()

	// TODO: Send cancel command to Agent
	// TODO: 向 Agent 发送取消命令

	// Notify progress callbacks
	m.notifyProgress(&TaskProgress{
		TaskID:    taskID,
		Status:    TaskStatusCancelled,
		Progress:  task.Progress,
		Message:   task.Message,
		Timestamp: now,
	})

	return nil
}

// RetryTask retries a failed task
// RetryTask 重试失败的任务
func (m *Manager) RetryTask(ctx context.Context, taskID string) (*Task, error) {
	m.mu.Lock()
	oldTask, ok := m.tasks[taskID]
	if !ok {
		m.mu.Unlock()
		return nil, ErrTaskNotFound
	}

	if oldTask.Status != TaskStatusFailed && oldTask.Status != TaskStatusTimeout {
		m.mu.Unlock()
		return oldTask, nil // Not a failed task
	}

	if oldTask.RetryCount >= oldTask.MaxRetries {
		m.mu.Unlock()
		return nil, errors.New("max retries exceeded / 超过最大重试次数")
	}

	// Create a new task based on the old one
	newTask := &Task{
		ID:             uuid.New().String(),
		Type:           oldTask.Type,
		HostID:         oldTask.HostID,
		Status:         TaskStatusPending,
		Progress:       0,
		Message:        "重试任务 / Retrying task",
		Params:         oldTask.Params,
		CreatedAt:      time.Now(),
		TimeoutSeconds: oldTask.TimeoutSeconds,
		Retryable:      oldTask.Retryable,
		RetryCount:     oldTask.RetryCount + 1,
		MaxRetries:     oldTask.MaxRetries,
		CreatedBy:      oldTask.CreatedBy,
	}

	m.tasks[newTask.ID] = newTask
	m.hostTasks[newTask.HostID] = append(m.hostTasks[newTask.HostID], newTask.ID)
	m.mu.Unlock()

	return newTask, nil
}

// RegisterProgressCallback registers a callback for task progress updates
// RegisterProgressCallback 注册任务进度更新的回调
func (m *Manager) RegisterProgressCallback(taskID string, callback ProgressCallback) {
	m.callbackMu.Lock()
	defer m.callbackMu.Unlock()

	m.progressCallbacks[taskID] = append(m.progressCallbacks[taskID], callback)
}

// UnregisterProgressCallbacks removes all callbacks for a task
// UnregisterProgressCallbacks 移除任务的所有回调
func (m *Manager) UnregisterProgressCallbacks(taskID string) {
	m.callbackMu.Lock()
	defer m.callbackMu.Unlock()

	delete(m.progressCallbacks, taskID)
}

// notifyProgress notifies all registered callbacks of progress update
// notifyProgress 通知所有注册的回调进度更新
func (m *Manager) notifyProgress(progress *TaskProgress) {
	m.callbackMu.RLock()
	callbacks := m.progressCallbacks[progress.TaskID]
	m.callbackMu.RUnlock()

	for _, cb := range callbacks {
		go cb(progress)
	}
}

// CleanupOldTasks removes completed tasks older than the specified duration
// CleanupOldTasks 移除超过指定时间的已完成任务
func (m *Manager) CleanupOldTasks(ctx context.Context, maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, task := range m.tasks {
		// Only remove completed tasks
		if task.Status != TaskStatusSuccess && task.Status != TaskStatusFailed &&
			task.Status != TaskStatusCancelled && task.Status != TaskStatusTimeout {
			continue
		}

		// Check if task is old enough
		if task.CompletedAt != nil && task.CompletedAt.Before(cutoff) {
			delete(m.tasks, id)
			removed++

			// Remove from hostTasks
			hostTaskIDs := m.hostTasks[task.HostID]
			for i, tid := range hostTaskIDs {
				if tid == id {
					m.hostTasks[task.HostID] = append(hostTaskIDs[:i], hostTaskIDs[i+1:]...)
					break
				}
			}
		}
	}

	return removed
}
