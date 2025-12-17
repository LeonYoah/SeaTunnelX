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

// Package task provides task management for Control Plane to Agent communication.
// task 包提供 Control Plane 到 Agent 通信的任务管理功能。
package task

import (
	"time"
)

// TaskType represents the type of task
// TaskType 表示任务类型
type TaskType string

const (
	// TaskTypeInstall is for SeaTunnel installation
	// TaskTypeInstall 用于 SeaTunnel 安装
	TaskTypeInstall TaskType = "install"

	// TaskTypeUninstall is for SeaTunnel uninstallation
	// TaskTypeUninstall 用于 SeaTunnel 卸载
	TaskTypeUninstall TaskType = "uninstall"

	// TaskTypeUpgrade is for SeaTunnel upgrade
	// TaskTypeUpgrade 用于 SeaTunnel 升级
	TaskTypeUpgrade TaskType = "upgrade"

	// TaskTypeStart is for starting SeaTunnel process
	// TaskTypeStart 用于启动 SeaTunnel 进程
	TaskTypeStart TaskType = "start"

	// TaskTypeStop is for stopping SeaTunnel process
	// TaskTypeStop 用于停止 SeaTunnel 进程
	TaskTypeStop TaskType = "stop"

	// TaskTypeRestart is for restarting SeaTunnel process
	// TaskTypeRestart 用于重启 SeaTunnel 进程
	TaskTypeRestart TaskType = "restart"

	// TaskTypePrecheck is for running precheck
	// TaskTypePrecheck 用于运行预检查
	TaskTypePrecheck TaskType = "precheck"

	// TaskTypeCollectLogs is for collecting logs
	// TaskTypeCollectLogs 用于收集日志
	TaskTypeCollectLogs TaskType = "collect_logs"

	// TaskTypeTransferPackage is for transferring package to Agent
	// TaskTypeTransferPackage 用于传输安装包到 Agent
	TaskTypeTransferPackage TaskType = "transfer_package"

	// TaskTypeInstallPlugin is for installing plugins
	// TaskTypeInstallPlugin 用于安装插件
	TaskTypeInstallPlugin TaskType = "install_plugin"

	// TaskTypeUninstallPlugin is for uninstalling plugins
	// TaskTypeUninstallPlugin 用于卸载插件
	TaskTypeUninstallPlugin TaskType = "uninstall_plugin"
)

// TaskStatus represents the status of a task
// TaskStatus 表示任务状态
type TaskStatus string

const (
	// TaskStatusPending indicates the task is waiting to be executed
	// TaskStatusPending 表示任务等待执行
	TaskStatusPending TaskStatus = "pending"

	// TaskStatusRunning indicates the task is being executed
	// TaskStatusRunning 表示任务正在执行
	TaskStatusRunning TaskStatus = "running"

	// TaskStatusSuccess indicates the task completed successfully
	// TaskStatusSuccess 表示任务执行成功
	TaskStatusSuccess TaskStatus = "success"

	// TaskStatusFailed indicates the task failed
	// TaskStatusFailed 表示任务执行失败
	TaskStatusFailed TaskStatus = "failed"

	// TaskStatusCancelled indicates the task was cancelled
	// TaskStatusCancelled 表示任务已取消
	TaskStatusCancelled TaskStatus = "cancelled"

	// TaskStatusTimeout indicates the task timed out
	// TaskStatusTimeout 表示任务超时
	TaskStatusTimeout TaskStatus = "timeout"
)

// Task represents a task to be executed by an Agent
// Task 表示要由 Agent 执行的任务
type Task struct {
	// ID is the unique task identifier
	// ID 是唯一的任务标识符
	ID string `json:"id"`

	// Type is the task type
	// Type 是任务类型
	Type TaskType `json:"type"`

	// HostID is the target host ID
	// HostID 是目标主机 ID
	HostID uint `json:"host_id"`

	// AgentID is the target agent ID
	// AgentID 是目标 Agent ID
	AgentID string `json:"agent_id"`

	// Status is the current task status
	// Status 是当前任务状态
	Status TaskStatus `json:"status"`

	// Progress is the task progress (0-100)
	// Progress 是任务进度（0-100）
	Progress int `json:"progress"`

	// Message is the current status message
	// Message 是当前状态消息
	Message string `json:"message,omitempty"`

	// Error is the error message if failed
	// Error 是失败时的错误消息
	Error string `json:"error,omitempty"`

	// Params contains task-specific parameters
	// Params 包含任务特定的参数
	Params map[string]interface{} `json:"params,omitempty"`

	// Result contains task execution result
	// Result 包含任务执行结果
	Result map[string]interface{} `json:"result,omitempty"`

	// CreatedAt is when the task was created
	// CreatedAt 是任务创建时间
	CreatedAt time.Time `json:"created_at"`

	// StartedAt is when the task started executing
	// StartedAt 是任务开始执行时间
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt is when the task completed
	// CompletedAt 是任务完成时间
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// TimeoutSeconds is the task timeout in seconds (0 means no timeout)
	// TimeoutSeconds 是任务超时时间（秒），0 表示无超时
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`

	// Retryable indicates if the task can be retried
	// Retryable 表示任务是否可重试
	Retryable bool `json:"retryable"`

	// RetryCount is the number of retry attempts
	// RetryCount 是重试次数
	RetryCount int `json:"retry_count"`

	// MaxRetries is the maximum number of retries
	// MaxRetries 是最大重试次数
	MaxRetries int `json:"max_retries"`

	// CreatedBy is the user who created the task
	// CreatedBy 是创建任务的用户
	CreatedBy string `json:"created_by,omitempty"`
}

// TaskStep represents a step within a task
// TaskStep 表示任务中的一个步骤
type TaskStep struct {
	// Name is the step name
	// Name 是步骤名称
	Name string `json:"name"`

	// Description is the step description
	// Description 是步骤描述
	Description string `json:"description"`

	// Status is the step status
	// Status 是步骤状态
	Status TaskStatus `json:"status"`

	// Progress is the step progress (0-100)
	// Progress 是步骤进度（0-100）
	Progress int `json:"progress"`

	// Message is the current status message
	// Message 是当前状态消息
	Message string `json:"message,omitempty"`

	// Error is the error message if failed
	// Error 是失败时的错误消息
	Error string `json:"error,omitempty"`

	// StartedAt is when the step started
	// StartedAt 是步骤开始时间
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt is when the step completed
	// CompletedAt 是步骤完成时间
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// TaskProgress represents a progress update from Agent
// TaskProgress 表示来自 Agent 的进度更新
type TaskProgress struct {
	// TaskID is the task identifier
	// TaskID 是任务标识符
	TaskID string `json:"task_id"`

	// Status is the current status
	// Status 是当前状态
	Status TaskStatus `json:"status"`

	// Progress is the overall progress (0-100)
	// Progress 是总体进度（0-100）
	Progress int `json:"progress"`

	// Message is the status message
	// Message 是状态消息
	Message string `json:"message,omitempty"`

	// CurrentStep is the current step name
	// CurrentStep 是当前步骤名称
	CurrentStep string `json:"current_step,omitempty"`

	// Steps contains all step statuses
	// Steps 包含所有步骤状态
	Steps []TaskStep `json:"steps,omitempty"`

	// Error is the error message if failed
	// Error 是失败时的错误消息
	Error string `json:"error,omitempty"`

	// Timestamp is when this progress was reported
	// Timestamp 是进度上报时间
	Timestamp time.Time `json:"timestamp"`
}

// CreateTaskRequest represents a request to create a new task
// CreateTaskRequest 表示创建新任务的请求
type CreateTaskRequest struct {
	// Type is the task type
	// Type 是任务类型
	Type TaskType `json:"type" binding:"required"`

	// HostID is the target host ID
	// HostID 是目标主机 ID
	HostID uint `json:"host_id" binding:"required"`

	// Params contains task-specific parameters
	// Params 包含任务特定的参数
	Params map[string]interface{} `json:"params,omitempty"`

	// TimeoutSeconds is the task timeout in seconds
	// TimeoutSeconds 是任务超时时间（秒）
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
}

// TaskListResponse represents a list of tasks
// TaskListResponse 表示任务列表
type TaskListResponse struct {
	// Tasks is the list of tasks
	// Tasks 是任务列表
	Tasks []*Task `json:"tasks"`

	// Total is the total number of tasks
	// Total 是任务总数
	Total int `json:"total"`
}
