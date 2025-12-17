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
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler provides HTTP handlers for task management
// Handler 提供任务管理的 HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler creates a new Handler instance
// NewHandler 创建新的 Handler 实例
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// ==================== Response Types 响应类型 ====================

// TaskResponse represents a single task response
// TaskResponse 表示单个任务响应
type TaskResponse struct {
	ErrorMsg string `json:"error_msg"`
	Data     *Task  `json:"data"`
}

// TaskListAPIResponse represents a task list response
// TaskListAPIResponse 表示任务列表响应
type TaskListAPIResponse struct {
	ErrorMsg string  `json:"error_msg"`
	Data     *TaskListResponse `json:"data"`
}

// ==================== API Handlers API 处理器 ====================

// CreateTask handles POST /api/v1/tasks - creates a new task
// CreateTask 处理 POST /api/v1/tasks - 创建新任务
// @Tags tasks
// @Accept json
// @Produce json
// @Param request body CreateTaskRequest true "创建任务请求"
// @Success 200 {object} TaskResponse
// @Router /api/v1/tasks [post]
func (h *Handler) CreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, TaskResponse{ErrorMsg: err.Error()})
		return
	}

	// Get current user from context
	createdBy := ""
	if user, exists := c.Get("user"); exists {
		if u, ok := user.(map[string]interface{}); ok {
			if username, ok := u["username"].(string); ok {
				createdBy = username
			}
		}
	}

	task, err := h.manager.CreateTask(c.Request.Context(), &req, createdBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, TaskResponse{ErrorMsg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, TaskResponse{Data: task})
}

// GetTask handles GET /api/v1/tasks/:id - gets a task by ID
// GetTask 处理 GET /api/v1/tasks/:id - 根据 ID 获取任务
// @Tags tasks
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} TaskResponse
// @Router /api/v1/tasks/{id} [get]
func (h *Handler) GetTask(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, TaskResponse{ErrorMsg: "任务 ID 不能为空 / Task ID is required"})
		return
	}

	task, err := h.manager.GetTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, TaskResponse{ErrorMsg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, TaskResponse{Data: task})
}

// ListTasks handles GET /api/v1/tasks - lists all tasks
// ListTasks 处理 GET /api/v1/tasks - 获取所有任务
// @Tags tasks
// @Produce json
// @Param status query string false "任务状态过滤"
// @Param type query string false "任务类型过滤"
// @Param limit query int false "返回数量限制"
// @Success 200 {object} TaskListAPIResponse
// @Router /api/v1/tasks [get]
func (h *Handler) ListTasks(c *gin.Context) {
	var status *TaskStatus
	var taskType *TaskType
	limit := 100

	if s := c.Query("status"); s != "" {
		st := TaskStatus(s)
		status = &st
	}

	if t := c.Query("type"); t != "" {
		tt := TaskType(t)
		taskType = &tt
	}

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	tasks, total, err := h.manager.ListAllTasks(c.Request.Context(), status, taskType, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, TaskListAPIResponse{ErrorMsg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, TaskListAPIResponse{
		Data: &TaskListResponse{
			Tasks: tasks,
			Total: total,
		},
	})
}

// ListHostTasks handles GET /api/v1/hosts/:id/tasks - lists tasks for a host
// ListHostTasks 处理 GET /api/v1/hosts/:id/tasks - 获取主机的任务列表
// @Tags tasks
// @Produce json
// @Param id path int true "主机ID"
// @Param limit query int false "返回数量限制"
// @Success 200 {object} TaskListAPIResponse
// @Router /api/v1/hosts/{id}/tasks [get]
func (h *Handler) ListHostTasks(c *gin.Context) {
	hostID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, TaskListAPIResponse{ErrorMsg: "无效的主机 ID / Invalid host ID"})
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	tasks, err := h.manager.ListTasks(c.Request.Context(), uint(hostID), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, TaskListAPIResponse{ErrorMsg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, TaskListAPIResponse{
		Data: &TaskListResponse{
			Tasks: tasks,
			Total: len(tasks),
		},
	})
}

// StartTask handles POST /api/v1/tasks/:id/start - starts a task
// StartTask 处理 POST /api/v1/tasks/:id/start - 开始执行任务
// @Tags tasks
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} TaskResponse
// @Router /api/v1/tasks/{id}/start [post]
func (h *Handler) StartTask(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, TaskResponse{ErrorMsg: "任务 ID 不能为空 / Task ID is required"})
		return
	}

	if err := h.manager.StartTask(c.Request.Context(), taskID); err != nil {
		c.JSON(http.StatusInternalServerError, TaskResponse{ErrorMsg: err.Error()})
		return
	}

	task, _ := h.manager.GetTask(c.Request.Context(), taskID)
	c.JSON(http.StatusOK, TaskResponse{Data: task})
}

// CancelTask handles POST /api/v1/tasks/:id/cancel - cancels a task
// CancelTask 处理 POST /api/v1/tasks/:id/cancel - 取消任务
// @Tags tasks
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} TaskResponse
// @Router /api/v1/tasks/{id}/cancel [post]
func (h *Handler) CancelTask(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, TaskResponse{ErrorMsg: "任务 ID 不能为空 / Task ID is required"})
		return
	}

	if err := h.manager.CancelTask(c.Request.Context(), taskID); err != nil {
		c.JSON(http.StatusInternalServerError, TaskResponse{ErrorMsg: err.Error()})
		return
	}

	task, _ := h.manager.GetTask(c.Request.Context(), taskID)
	c.JSON(http.StatusOK, TaskResponse{Data: task})
}

// RetryTask handles POST /api/v1/tasks/:id/retry - retries a failed task
// RetryTask 处理 POST /api/v1/tasks/:id/retry - 重试失败的任务
// @Tags tasks
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} TaskResponse
// @Router /api/v1/tasks/{id}/retry [post]
func (h *Handler) RetryTask(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, TaskResponse{ErrorMsg: "任务 ID 不能为空 / Task ID is required"})
		return
	}

	task, err := h.manager.RetryTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, TaskResponse{ErrorMsg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, TaskResponse{Data: task})
}
