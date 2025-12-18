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
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/seatunnel/seatunnelX/internal/logger"
)

// Handler provides HTTP handlers for plugin management.
// Handler 提供插件管理的 HTTP 处理器。
type Handler struct {
	service *Service
}

// NewHandler creates a new Handler instance.
// NewHandler 创建一个新的 Handler 实例。
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ==================== Available Plugins APIs 可用插件 API ====================

// ListPluginsResponse represents the response for listing plugins.
// ListPluginsResponse 表示获取插件列表的响应。
type ListPluginsResponse struct {
	ErrorMsg string                    `json:"error_msg"`
	Data     *AvailablePluginsResponse `json:"data"`
}

// ListAvailablePlugins handles GET /api/v1/plugins - lists available plugins.
// ListAvailablePlugins 处理 GET /api/v1/plugins - 获取可用插件列表。
// @Tags plugins
// @Produce json
// @Param version query string false "SeaTunnel 版本"
// @Param mirror query string false "镜像源 (apache/aliyun/huaweicloud)"
// @Success 200 {object} ListPluginsResponse
// @Router /api/v1/plugins [get]
func (h *Handler) ListAvailablePlugins(c *gin.Context) {
	version := c.Query("version")
	mirror := MirrorSource(c.Query("mirror"))

	result, err := h.service.ListAvailablePlugins(c.Request.Context(), version, mirror)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ListPluginsResponse{ErrorMsg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ListPluginsResponse{Data: result})
}


// GetPluginInfoResponse represents the response for getting plugin info.
// GetPluginInfoResponse 表示获取插件信息的响应。
type GetPluginInfoResponse struct {
	ErrorMsg string  `json:"error_msg"`
	Data     *Plugin `json:"data"`
}

// GetPluginInfo handles GET /api/v1/plugins/:name - gets plugin info.
// GetPluginInfo 处理 GET /api/v1/plugins/:name - 获取插件详情。
// @Tags plugins
// @Produce json
// @Param name path string true "插件名称"
// @Param version query string false "SeaTunnel 版本"
// @Success 200 {object} GetPluginInfoResponse
// @Router /api/v1/plugins/{name} [get]
func (h *Handler) GetPluginInfo(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, GetPluginInfoResponse{ErrorMsg: "插件名称不能为空 / Plugin name is required"})
		return
	}

	version := c.Query("version")

	plugin, err := h.service.GetPluginInfo(c.Request.Context(), name, version)
	if err != nil {
		c.JSON(http.StatusNotFound, GetPluginInfoResponse{ErrorMsg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, GetPluginInfoResponse{Data: plugin})
}

// ==================== Installed Plugins APIs 已安装插件 API ====================

// ListInstalledPluginsResponse represents the response for listing installed plugins.
// ListInstalledPluginsResponse 表示获取已安装插件列表的响应。
type ListInstalledPluginsResponse struct {
	ErrorMsg string            `json:"error_msg"`
	Data     []InstalledPlugin `json:"data"`
}

// ListInstalledPlugins handles GET /api/v1/clusters/:id/plugins - lists installed plugins on a cluster.
// ListInstalledPlugins 处理 GET /api/v1/clusters/:id/plugins - 获取集群已安装插件列表。
// @Tags plugins
// @Produce json
// @Param id path int true "集群ID"
// @Success 200 {object} ListInstalledPluginsResponse
// @Router /api/v1/clusters/{id}/plugins [get]
func (h *Handler) ListInstalledPlugins(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ListInstalledPluginsResponse{ErrorMsg: "无效的集群 ID / Invalid cluster ID"})
		return
	}

	plugins, err := h.service.ListInstalledPlugins(c.Request.Context(), uint(clusterID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ListInstalledPluginsResponse{ErrorMsg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ListInstalledPluginsResponse{Data: plugins})
}

// ==================== Plugin Installation APIs 插件安装 API ====================

// InstallPluginResponse represents the response for installing a plugin.
// InstallPluginResponse 表示安装插件的响应。
type InstallPluginResponse struct {
	ErrorMsg string           `json:"error_msg"`
	Data     *InstalledPlugin `json:"data"`
}

// InstallPlugin handles POST /api/v1/clusters/:id/plugins - installs a plugin on a cluster.
// InstallPlugin 处理 POST /api/v1/clusters/:id/plugins - 在集群上安装插件。
// @Tags plugins
// @Accept json
// @Produce json
// @Param id path int true "集群ID"
// @Param request body InstallPluginRequest true "安装请求"
// @Success 200 {object} InstallPluginResponse
// @Router /api/v1/clusters/{id}/plugins [post]
func (h *Handler) InstallPlugin(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, InstallPluginResponse{ErrorMsg: "无效的集群 ID / Invalid cluster ID"})
		return
	}

	var req InstallPluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, InstallPluginResponse{ErrorMsg: err.Error()})
		return
	}

	installed, err := h.service.InstallPlugin(c.Request.Context(), uint(clusterID), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, InstallPluginResponse{ErrorMsg: err.Error()})
		return
	}

	logger.InfoF(c.Request.Context(), "[Plugin] 安装插件成功: cluster=%d, plugin=%s", clusterID, req.PluginName)
	c.JSON(http.StatusOK, InstallPluginResponse{Data: installed})
}

// UninstallPluginResponse represents the response for uninstalling a plugin.
// UninstallPluginResponse 表示卸载插件的响应。
type UninstallPluginResponse struct {
	ErrorMsg string `json:"error_msg"`
	Data     any    `json:"data"`
}

// UninstallPlugin handles DELETE /api/v1/clusters/:id/plugins/:name - uninstalls a plugin from a cluster.
// UninstallPlugin 处理 DELETE /api/v1/clusters/:id/plugins/:name - 从集群卸载插件。
// @Tags plugins
// @Produce json
// @Param id path int true "集群ID"
// @Param name path string true "插件名称"
// @Success 200 {object} UninstallPluginResponse
// @Router /api/v1/clusters/{id}/plugins/{name} [delete]
func (h *Handler) UninstallPlugin(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, UninstallPluginResponse{ErrorMsg: "无效的集群 ID / Invalid cluster ID"})
		return
	}

	pluginName := c.Param("name")
	if pluginName == "" {
		c.JSON(http.StatusBadRequest, UninstallPluginResponse{ErrorMsg: "插件名称不能为空 / Plugin name is required"})
		return
	}

	if err := h.service.UninstallPlugin(c.Request.Context(), uint(clusterID), pluginName); err != nil {
		c.JSON(http.StatusInternalServerError, UninstallPluginResponse{ErrorMsg: err.Error()})
		return
	}

	logger.InfoF(c.Request.Context(), "[Plugin] 卸载插件成功: cluster=%d, plugin=%s", clusterID, pluginName)
	c.JSON(http.StatusOK, UninstallPluginResponse{})
}

// EnableDisablePluginResponse represents the response for enabling/disabling a plugin.
// EnableDisablePluginResponse 表示启用/禁用插件的响应。
type EnableDisablePluginResponse struct {
	ErrorMsg string           `json:"error_msg"`
	Data     *InstalledPlugin `json:"data"`
}

// EnablePlugin handles PUT /api/v1/clusters/:id/plugins/:name/enable - enables a plugin.
// EnablePlugin 处理 PUT /api/v1/clusters/:id/plugins/:name/enable - 启用插件。
// @Tags plugins
// @Produce json
// @Param id path int true "集群ID"
// @Param name path string true "插件名称"
// @Success 200 {object} EnableDisablePluginResponse
// @Router /api/v1/clusters/{id}/plugins/{name}/enable [put]
func (h *Handler) EnablePlugin(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, EnableDisablePluginResponse{ErrorMsg: "无效的集群 ID / Invalid cluster ID"})
		return
	}

	pluginName := c.Param("name")
	if pluginName == "" {
		c.JSON(http.StatusBadRequest, EnableDisablePluginResponse{ErrorMsg: "插件名称不能为空 / Plugin name is required"})
		return
	}

	plugin, err := h.service.EnablePlugin(c.Request.Context(), uint(clusterID), pluginName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, EnableDisablePluginResponse{ErrorMsg: err.Error()})
		return
	}

	logger.InfoF(c.Request.Context(), "[Plugin] 启用插件成功: cluster=%d, plugin=%s", clusterID, pluginName)
	c.JSON(http.StatusOK, EnableDisablePluginResponse{Data: plugin})
}

// DisablePlugin handles PUT /api/v1/clusters/:id/plugins/:name/disable - disables a plugin.
// DisablePlugin 处理 PUT /api/v1/clusters/:id/plugins/:name/disable - 禁用插件。
// @Tags plugins
// @Produce json
// @Param id path int true "集群ID"
// @Param name path string true "插件名称"
// @Success 200 {object} EnableDisablePluginResponse
// @Router /api/v1/clusters/{id}/plugins/{name}/disable [put]
func (h *Handler) DisablePlugin(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, EnableDisablePluginResponse{ErrorMsg: "无效的集群 ID / Invalid cluster ID"})
		return
	}

	pluginName := c.Param("name")
	if pluginName == "" {
		c.JSON(http.StatusBadRequest, EnableDisablePluginResponse{ErrorMsg: "插件名称不能为空 / Plugin name is required"})
		return
	}

	plugin, err := h.service.DisablePlugin(c.Request.Context(), uint(clusterID), pluginName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, EnableDisablePluginResponse{ErrorMsg: err.Error()})
		return
	}

	logger.InfoF(c.Request.Context(), "[Plugin] 禁用插件成功: cluster=%d, plugin=%s", clusterID, pluginName)
	c.JSON(http.StatusOK, EnableDisablePluginResponse{Data: plugin})
}

// ==================== Plugin Download APIs 插件下载 API ====================

// DownloadPluginRequest represents a request to download a plugin.
// DownloadPluginRequest 表示下载插件的请求。
type DownloadPluginRequest struct {
	Version string       `json:"version" binding:"required"` // 版本号 / Version
	Mirror  MirrorSource `json:"mirror,omitempty"`           // 镜像源 / Mirror source
}

// DownloadPluginResponse represents the response for downloading a plugin.
// DownloadPluginResponse 表示下载插件的响应。
type DownloadPluginResponse struct {
	ErrorMsg string            `json:"error_msg"`
	Data     *DownloadProgress `json:"data"`
}

// DownloadPlugin handles POST /api/v1/plugins/:name/download - downloads a plugin to Control Plane.
// DownloadPlugin 处理 POST /api/v1/plugins/:name/download - 下载插件到 Control Plane。
// @Tags plugins
// @Accept json
// @Produce json
// @Param name path string true "插件名称"
// @Param request body DownloadPluginRequest true "下载请求"
// @Success 200 {object} DownloadPluginResponse
// @Router /api/v1/plugins/{name}/download [post]
func (h *Handler) DownloadPlugin(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, DownloadPluginResponse{ErrorMsg: "插件名称不能为空 / Plugin name is required"})
		return
	}

	var req DownloadPluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, DownloadPluginResponse{ErrorMsg: err.Error()})
		return
	}

	progress, err := h.service.DownloadPlugin(c.Request.Context(), name, req.Version, req.Mirror)
	if err != nil {
		c.JSON(http.StatusInternalServerError, DownloadPluginResponse{ErrorMsg: err.Error()})
		return
	}

	logger.InfoF(c.Request.Context(), "[Plugin] 开始下载插件: plugin=%s, version=%s", name, req.Version)
	c.JSON(http.StatusOK, DownloadPluginResponse{Data: progress})
}

// GetDownloadStatusResponse represents the response for getting download status.
// GetDownloadStatusResponse 表示获取下载状态的响应。
type GetDownloadStatusResponse struct {
	ErrorMsg string            `json:"error_msg"`
	Data     *DownloadProgress `json:"data"`
}

// GetDownloadStatus handles GET /api/v1/plugins/:name/download/status - gets download status.
// GetDownloadStatus 处理 GET /api/v1/plugins/:name/download/status - 获取下载状态。
// @Tags plugins
// @Produce json
// @Param name path string true "插件名称"
// @Param version query string true "版本号"
// @Success 200 {object} GetDownloadStatusResponse
// @Router /api/v1/plugins/{name}/download/status [get]
func (h *Handler) GetDownloadStatus(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, GetDownloadStatusResponse{ErrorMsg: "插件名称不能为空 / Plugin name is required"})
		return
	}

	version := c.Query("version")
	if version == "" {
		c.JSON(http.StatusBadRequest, GetDownloadStatusResponse{ErrorMsg: "版本号不能为空 / Version is required"})
		return
	}

	progress := h.service.GetDownloadStatus(name, version)
	c.JSON(http.StatusOK, GetDownloadStatusResponse{Data: progress})
}

// ListLocalPluginsResponse represents the response for listing local plugins.
// ListLocalPluginsResponse 表示获取本地插件列表的响应。
type ListLocalPluginsResponse struct {
	ErrorMsg string        `json:"error_msg"`
	Data     []LocalPlugin `json:"data"`
}

// ListLocalPlugins handles GET /api/v1/plugins/local - lists locally downloaded plugins.
// ListLocalPlugins 处理 GET /api/v1/plugins/local - 获取已下载的本地插件列表。
// @Tags plugins
// @Produce json
// @Success 200 {object} ListLocalPluginsResponse
// @Router /api/v1/plugins/local [get]
func (h *Handler) ListLocalPlugins(c *gin.Context) {
	plugins, err := h.service.ListLocalPlugins()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ListLocalPluginsResponse{ErrorMsg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ListLocalPluginsResponse{Data: plugins})
}

// ListActiveDownloadsResponse represents the response for listing active downloads.
// ListActiveDownloadsResponse 表示获取活动下载列表的响应。
type ListActiveDownloadsResponse struct {
	ErrorMsg string              `json:"error_msg"`
	Data     []*DownloadProgress `json:"data"`
}

// ListActiveDownloads handles GET /api/v1/plugins/downloads - lists active download tasks.
// ListActiveDownloads 处理 GET /api/v1/plugins/downloads - 获取活动下载任务列表。
// @Tags plugins
// @Produce json
// @Success 200 {object} ListActiveDownloadsResponse
// @Router /api/v1/plugins/downloads [get]
func (h *Handler) ListActiveDownloads(c *gin.Context) {
	downloads := h.service.ListActiveDownloads()
	c.JSON(http.StatusOK, ListActiveDownloadsResponse{Data: downloads})
}

// DeleteLocalPluginResponse represents the response for deleting a local plugin.
// DeleteLocalPluginResponse 表示删除本地插件的响应。
type DeleteLocalPluginResponse struct {
	ErrorMsg string `json:"error_msg"`
	Data     any    `json:"data"`
}

// DeleteLocalPlugin handles DELETE /api/v1/plugins/:name/local - deletes a locally downloaded plugin.
// DeleteLocalPlugin 处理 DELETE /api/v1/plugins/:name/local - 删除本地已下载的插件。
// @Tags plugins
// @Produce json
// @Param name path string true "插件名称"
// @Param version query string true "版本号"
// @Success 200 {object} DeleteLocalPluginResponse
// @Router /api/v1/plugins/{name}/local [delete]
func (h *Handler) DeleteLocalPlugin(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, DeleteLocalPluginResponse{ErrorMsg: "插件名称不能为空 / Plugin name is required"})
		return
	}

	version := c.Query("version")
	if version == "" {
		c.JSON(http.StatusBadRequest, DeleteLocalPluginResponse{ErrorMsg: "版本号不能为空 / Version is required"})
		return
	}

	if err := h.service.DeleteLocalPlugin(name, version); err != nil {
		c.JSON(http.StatusInternalServerError, DeleteLocalPluginResponse{ErrorMsg: err.Error()})
		return
	}

	logger.InfoF(c.Request.Context(), "[Plugin] 删除本地插件成功: plugin=%s, version=%s", name, version)
	c.JSON(http.StatusOK, DeleteLocalPluginResponse{})
}

// ==================== Cluster Plugin Installation Progress API 集群插件安装进度 API ====================

// GetInstallProgressResponse represents the response for getting plugin installation progress.
// GetInstallProgressResponse 表示获取插件安装进度的响应。
type GetInstallProgressResponse struct {
	ErrorMsg string               `json:"error_msg"`
	Data     *PluginInstallStatus `json:"data"`
}

// GetInstallProgress handles GET /api/v1/clusters/:id/plugins/:name/progress - gets plugin installation progress.
// GetInstallProgress 处理 GET /api/v1/clusters/:id/plugins/:name/progress - 获取插件安装进度。
// @Tags plugins
// @Produce json
// @Param id path int true "集群ID"
// @Param name path string true "插件名称"
// @Success 200 {object} GetInstallProgressResponse
// @Router /api/v1/clusters/{id}/plugins/{name}/progress [get]
func (h *Handler) GetInstallProgress(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, GetInstallProgressResponse{ErrorMsg: "无效的集群 ID / Invalid cluster ID"})
		return
	}

	pluginName := c.Param("name")
	if pluginName == "" {
		c.JSON(http.StatusBadRequest, GetInstallProgressResponse{ErrorMsg: "插件名称不能为空 / Plugin name is required"})
		return
	}

	progress := h.service.GetInstallProgress(uint(clusterID), pluginName)
	c.JSON(http.StatusOK, GetInstallProgressResponse{Data: progress})
}
