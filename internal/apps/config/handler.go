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

package config

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler 配置管理 HTTP 处理器
type Handler struct {
	service *Service
}

// NewHandler 创建处理器实例
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetClusterConfigs 获取集群所有配置
// @Summary 获取集群配置列表
// @Tags Config
// @Produce json
// @Param id path int true "集群ID"
// @Success 200 {array} ConfigInfo
// @Router /api/v1/clusters/{id}/configs [get]
func (h *Handler) GetClusterConfigs(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster id"})
		return
	}

	configs, err := h.service.GetByCluster(c.Request.Context(), uint(clusterID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, configs)
}

// GetConfig 获取配置详情
// @Summary 获取配置详情
// @Tags Config
// @Produce json
// @Param id path int true "配置ID"
// @Success 200 {object} ConfigInfo
// @Router /api/v1/configs/{id} [get]
func (h *Handler) GetConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	config, err := h.service.Get(c.Request.Context(), uint(id))
	if err != nil {
		if err == ErrConfigNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, config)
}

// UpdateConfig 更新配置
// @Summary 更新配置
// @Tags Config
// @Accept json
// @Produce json
// @Param id path int true "配置ID"
// @Param body body UpdateConfigRequest true "更新内容"
// @Success 200 {object} ConfigInfo
// @Router /api/v1/configs/{id} [put]
func (h *Handler) UpdateConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := getUserID(c)
	config, err := h.service.Update(c.Request.Context(), uint(id), &req, userID)
	if err != nil {
		if err == ErrConfigNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, config)
}

// GetConfigVersions 获取配置版本历史
// @Summary 获取配置版本历史
// @Tags Config
// @Produce json
// @Param id path int true "配置ID"
// @Success 200 {array} ConfigVersionInfo
// @Router /api/v1/configs/{id}/versions [get]
func (h *Handler) GetConfigVersions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	versions, err := h.service.GetVersions(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, versions)
}

// RollbackConfig 回滚配置到指定版本
// @Summary 回滚配置
// @Tags Config
// @Accept json
// @Produce json
// @Param id path int true "配置ID"
// @Param body body RollbackConfigRequest true "回滚请求"
// @Success 200 {object} ConfigInfo
// @Router /api/v1/configs/{id}/rollback [post]
func (h *Handler) RollbackConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	var req RollbackConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := getUserID(c)
	config, err := h.service.Rollback(c.Request.Context(), uint(id), &req, userID)
	if err != nil {
		if err == ErrConfigNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
			return
		}
		if err == ErrVersionNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, config)
}

// PromoteConfig 推广配置到集群
// @Summary 推广配置到集群
// @Tags Config
// @Accept json
// @Produce json
// @Param id path int true "配置ID"
// @Param body body PromoteConfigRequest true "推广请求"
// @Success 200 {object} gin.H
// @Router /api/v1/configs/{id}/promote [post]
func (h *Handler) PromoteConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	var req PromoteConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空 body
		req = PromoteConfigRequest{}
	}

	userID := getUserID(c)
	if err := h.service.Promote(c.Request.Context(), uint(id), &req, userID); err != nil {
		if err == ErrConfigNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
			return
		}
		if err == ErrCannotPromoteTemplate {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot promote template config"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "config promoted to cluster successfully"})
}

// SyncFromTemplate 从集群模板同步
// @Summary 从集群模板同步配置
// @Tags Config
// @Accept json
// @Produce json
// @Param id path int true "配置ID"
// @Param body body SyncConfigRequest true "同步请求"
// @Success 200 {object} ConfigInfo
// @Router /api/v1/configs/{id}/sync [post]
func (h *Handler) SyncFromTemplate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	var req SyncConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空 body
		req = SyncConfigRequest{}
	}

	userID := getUserID(c)
	config, err := h.service.SyncFromTemplate(c.Request.Context(), uint(id), &req, userID)
	if err != nil {
		if err == ErrConfigNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
			return
		}
		if err == ErrTemplateNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster template not found"})
			return
		}
		if err == ErrCannotSyncTemplate {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot sync template config"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, config)
}

// InitClusterConfigsRequest 初始化集群配置请求
type InitClusterConfigsRequest struct {
	HostID     uint   `json:"host_id" binding:"required"`
	InstallDir string `json:"install_dir" binding:"required"`
}

// InitClusterConfigs 初始化集群配置（从节点拉取）
// @Summary 初始化集群配置
// @Tags Config
// @Accept json
// @Produce json
// @Param id path int true "集群ID"
// @Param body body InitClusterConfigsRequest true "初始化请求"
// @Success 200 {object} gin.H
// @Router /api/v1/clusters/{id}/configs/init [post]
func (h *Handler) InitClusterConfigs(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster id"})
		return
	}

	var req InitClusterConfigsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := getUserID(c)
	if err := h.service.InitClusterConfigs(c.Request.Context(), uint(clusterID), req.HostID, req.InstallDir, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "cluster configs initialized successfully"})
}

// PushConfigRequest 推送配置请求
type PushConfigRequest struct {
	InstallDir string `json:"install_dir" binding:"required"`
}

// PushConfigToNode 推送配置到节点
// @Summary 推送配置到节点
// @Tags Config
// @Accept json
// @Produce json
// @Param id path int true "配置ID"
// @Param body body PushConfigRequest true "推送请求"
// @Success 200 {object} gin.H
// @Router /api/v1/configs/{id}/push [post]
func (h *Handler) PushConfigToNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	var req PushConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.PushConfigToNode(c.Request.Context(), uint(id), req.InstallDir); err != nil {
		if err == ErrConfigNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "config pushed to node successfully"})
}

// getUserID 从上下文获取用户ID
func getUserID(c *gin.Context) uint {
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(uint); ok {
			return id
		}
	}
	return 0
}
