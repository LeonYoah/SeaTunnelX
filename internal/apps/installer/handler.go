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

// Package installer provides SeaTunnel installation management APIs for Control Plane.
// installer 包提供 Control Plane 的 SeaTunnel 安装管理 API。
package installer

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/seatunnel/seatunnelX/internal/logger"
)

// Handler provides HTTP handlers for installation management.
// Handler 提供安装管理的 HTTP 处理器。
type Handler struct {
	service *Service
}

// NewHandler creates a new Handler instance.
// NewHandler 创建一个新的 Handler 实例。
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ==================== Package Management APIs 安装包管理 API ====================

// ListPackagesResponse represents the response for listing packages.
// ListPackagesResponse 表示获取安装包列表的响应。
type ListPackagesResponse struct {
	ErrorMsg string             `json:"error_msg"`
	Data     *AvailableVersions `json:"data"`
}

// ListPackages handles GET /api/v1/packages - lists available packages.
// ListPackages 处理 GET /api/v1/packages - 获取可用安装包列表。
// @Tags packages
// @Produce json
// @Success 200 {object} ListPackagesResponse
// @Router /api/v1/packages [get]
func (h *Handler) ListPackages(c *gin.Context) {
	versions, err := h.service.ListAvailableVersions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ListPackagesResponse{ErrorMsg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ListPackagesResponse{Data: versions})
}

// GetPackageInfoResponse represents the response for getting package info.
// GetPackageInfoResponse 表示获取安装包信息的响应。
type GetPackageInfoResponse struct {
	ErrorMsg string       `json:"error_msg"`
	Data     *PackageInfo `json:"data"`
}

// GetPackageInfo handles GET /api/v1/packages/:version - gets package info.
// GetPackageInfo 处理 GET /api/v1/packages/:version - 获取安装包信息。
// @Tags packages
// @Produce json
// @Param version path string true "版本号"
// @Success 200 {object} GetPackageInfoResponse
// @Router /api/v1/packages/{version} [get]
func (h *Handler) GetPackageInfo(c *gin.Context) {
	version := c.Param("version")
	if version == "" {
		c.JSON(http.StatusBadRequest, GetPackageInfoResponse{ErrorMsg: "版本号不能为空 / Version is required"})
		return
	}

	info, err := h.service.GetPackageInfo(c.Request.Context(), version)
	if err != nil {
		c.JSON(http.StatusNotFound, GetPackageInfoResponse{ErrorMsg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, GetPackageInfoResponse{Data: info})
}

// UploadPackageResponse represents the response for uploading a package.
// UploadPackageResponse 表示上传安装包的响应。
type UploadPackageResponse struct {
	ErrorMsg string       `json:"error_msg"`
	Data     *PackageInfo `json:"data"`
}

// UploadPackage handles POST /api/v1/packages/upload - uploads a package.
// UploadPackage 处理 POST /api/v1/packages/upload - 上传安装包。
// @Tags packages
// @Accept multipart/form-data
// @Produce json
// @Param file formance file true "安装包文件"
// @Param version formData string true "版本号"
// @Success 200 {object} UploadPackageResponse
// @Router /api/v1/packages/upload [post]
func (h *Handler) UploadPackage(c *gin.Context) {
	version := c.PostForm("version")
	if version == "" {
		c.JSON(http.StatusBadRequest, UploadPackageResponse{ErrorMsg: "版本号不能为空 / Version is required"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, UploadPackageResponse{ErrorMsg: "文件上传失败 / File upload failed: " + err.Error()})
		return
	}

	info, err := h.service.UploadPackage(c.Request.Context(), version, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, UploadPackageResponse{ErrorMsg: err.Error()})
		return
	}

	logger.InfoF(c.Request.Context(), "[Installer] 上传安装包成功: %s", version)
	c.JSON(http.StatusOK, UploadPackageResponse{Data: info})
}

// DeletePackageResponse represents the response for deleting a package.
// DeletePackageResponse 表示删除安装包的响应。
type DeletePackageResponse struct {
	ErrorMsg string `json:"error_msg"`
	Data     any    `json:"data"`
}

// DeletePackage handles DELETE /api/v1/packages/:version - deletes a local package.
// DeletePackage 处理 DELETE /api/v1/packages/:version - 删除本地安装包。
// @Tags packages
// @Produce json
// @Param version path string true "版本号"
// @Success 200 {object} DeletePackageResponse
// @Router /api/v1/packages/{version} [delete]
func (h *Handler) DeletePackage(c *gin.Context) {
	version := c.Param("version")
	if version == "" {
		c.JSON(http.StatusBadRequest, DeletePackageResponse{ErrorMsg: "版本号不能为空 / Version is required"})
		return
	}

	if err := h.service.DeletePackage(c.Request.Context(), version); err != nil {
		c.JSON(http.StatusInternalServerError, DeletePackageResponse{ErrorMsg: err.Error()})
		return
	}

	logger.InfoF(c.Request.Context(), "[Installer] 删除安装包成功: %s", version)
	c.JSON(http.StatusOK, DeletePackageResponse{})
}

// ==================== Precheck APIs 预检查 API ====================

// PrecheckRequest represents the request for precheck.
// PrecheckRequest 表示预检查请求。
type PrecheckRequest struct {
	MinMemoryMB    int64 `json:"min_memory_mb"`
	MinCPUCores    int   `json:"min_cpu_cores"`
	MinDiskSpaceMB int64 `json:"min_disk_space_mb"`
	InstallDir     string `json:"install_dir"`
	Ports          []int  `json:"ports"`
}

// PrecheckResponse represents the response for precheck.
// PrecheckResponse 表示预检查响应。
type PrecheckResponse struct {
	ErrorMsg string          `json:"error_msg"`
	Data     *PrecheckResult `json:"data"`
}

// RunPrecheck handles POST /api/v1/hosts/:id/precheck - runs precheck on a host.
// RunPrecheck 处理 POST /api/v1/hosts/:id/precheck - 在主机上运行预检查。
// @Tags installation
// @Accept json
// @Produce json
// @Param id path int true "主机ID"
// @Param request body PrecheckRequest false "预检查参数"
// @Success 200 {object} PrecheckResponse
// @Router /api/v1/hosts/{id}/precheck [post]
func (h *Handler) RunPrecheck(c *gin.Context) {
	hostID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, PrecheckResponse{ErrorMsg: "无效的主机 ID / Invalid host ID"})
		return
	}

	var req PrecheckRequest
	// Use default values if not provided / 如果未提供则使用默认值
	if err := c.ShouldBindJSON(&req); err != nil {
		// Ignore binding errors, use defaults / 忽略绑定错误，使用默认值
	}

	result, err := h.service.RunPrecheck(c.Request.Context(), uint(hostID), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, PrecheckResponse{ErrorMsg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, PrecheckResponse{Data: result})
}

// ==================== Installation APIs 安装 API ====================

// InstallResponse represents the response for installation.
// InstallResponse 表示安装响应。
type InstallResponse struct {
	ErrorMsg string              `json:"error_msg"`
	Data     *InstallationStatus `json:"data"`
}

// StartInstallation handles POST /api/v1/hosts/:id/install - starts installation.
// StartInstallation 处理 POST /api/v1/hosts/:id/install - 开始安装。
// @Tags installation
// @Accept json
// @Produce json
// @Param id path int true "主机ID"
// @Param request body InstallationRequest true "安装请求"
// @Success 200 {object} InstallResponse
// @Router /api/v1/hosts/{id}/install [post]
func (h *Handler) StartInstallation(c *gin.Context) {
	hostID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, InstallResponse{ErrorMsg: "无效的主机 ID / Invalid host ID"})
		return
	}

	var req InstallationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, InstallResponse{ErrorMsg: err.Error()})
		return
	}

	// Set host ID from path / 从路径设置主机 ID
	req.HostID = strconv.FormatUint(hostID, 10)

	status, err := h.service.StartInstallation(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, InstallResponse{ErrorMsg: err.Error()})
		return
	}

	logger.InfoF(c.Request.Context(), "[Installer] 开始安装: host=%d, version=%s", hostID, req.Version)
	c.JSON(http.StatusOK, InstallResponse{Data: status})
}

// GetInstallationStatus handles GET /api/v1/hosts/:id/install/status - gets installation status.
// GetInstallationStatus 处理 GET /api/v1/hosts/:id/install/status - 获取安装状态。
// @Tags installation
// @Produce json
// @Param id path int true "主机ID"
// @Success 200 {object} InstallResponse
// @Router /api/v1/hosts/{id}/install/status [get]
func (h *Handler) GetInstallationStatus(c *gin.Context) {
	hostID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, InstallResponse{ErrorMsg: "无效的主机 ID / Invalid host ID"})
		return
	}

	status, err := h.service.GetInstallationStatus(c.Request.Context(), uint(hostID))
	if err != nil {
		c.JSON(http.StatusNotFound, InstallResponse{ErrorMsg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, InstallResponse{Data: status})
}

// RetryStepRequest represents the request for retrying a step.
// RetryStepRequest 表示重试步骤的请求。
type RetryStepRequest struct {
	Step string `json:"step" binding:"required"`
}

// RetryStep handles POST /api/v1/hosts/:id/install/retry - retries a failed step.
// RetryStep 处理 POST /api/v1/hosts/:id/install/retry - 重试失败的步骤。
// @Tags installation
// @Accept json
// @Produce json
// @Param id path int true "主机ID"
// @Param request body RetryStepRequest true "重试请求"
// @Success 200 {object} InstallResponse
// @Router /api/v1/hosts/{id}/install/retry [post]
func (h *Handler) RetryStep(c *gin.Context) {
	hostID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, InstallResponse{ErrorMsg: "无效的主机 ID / Invalid host ID"})
		return
	}

	var req RetryStepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, InstallResponse{ErrorMsg: err.Error()})
		return
	}

	status, err := h.service.RetryStep(c.Request.Context(), uint(hostID), req.Step)
	if err != nil {
		c.JSON(http.StatusInternalServerError, InstallResponse{ErrorMsg: err.Error()})
		return
	}

	logger.InfoF(c.Request.Context(), "[Installer] 重试步骤: host=%d, step=%s", hostID, req.Step)
	c.JSON(http.StatusOK, InstallResponse{Data: status})
}

// CancelInstallation handles POST /api/v1/hosts/:id/install/cancel - cancels installation.
// CancelInstallation 处理 POST /api/v1/hosts/:id/install/cancel - 取消安装。
// @Tags installation
// @Produce json
// @Param id path int true "主机ID"
// @Success 200 {object} InstallResponse
// @Router /api/v1/hosts/{id}/install/cancel [post]
func (h *Handler) CancelInstallation(c *gin.Context) {
	hostID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, InstallResponse{ErrorMsg: "无效的主机 ID / Invalid host ID"})
		return
	}

	status, err := h.service.CancelInstallation(c.Request.Context(), uint(hostID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, InstallResponse{ErrorMsg: err.Error()})
		return
	}

	logger.InfoF(c.Request.Context(), "[Installer] 取消安装: host=%d", hostID)
	c.JSON(http.StatusOK, InstallResponse{Data: status})
}
