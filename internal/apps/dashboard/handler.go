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

package dashboard

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// OverviewHandler handles dashboard overview HTTP requests.
// OverviewHandler 处理仪表盘概览 HTTP 请求。
type OverviewHandler struct {
	service *OverviewService
}

// NewOverviewHandler creates a new dashboard overview handler.
// NewOverviewHandler 创建新的仪表盘概览处理器。
func NewOverviewHandler(service *OverviewService) *OverviewHandler {
	return &OverviewHandler{service: service}
}

// GetOverviewStats godoc
// @Summary Get dashboard overview statistics
// @Description Get dashboard overview statistics including hosts, clusters, nodes, and agents
// @Tags Dashboard
// @Accept json
// @Produce json
// @Success 200 {object} DashboardDataResponse{data=OverviewStats}
// @Failure 500 {object} DashboardDataResponse
// @Router /api/v1/dashboard/overview/stats [get]
func (h *OverviewHandler) GetOverviewStats(c *gin.Context) {
	stats, err := h.service.GetOverviewStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, DashboardDataResponse{
			ErrorMsg: "Failed to get overview stats: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, DashboardDataResponse{Data: stats})
}

// GetClusterSummaries godoc
// @Summary Get cluster summaries
// @Description Get cluster summaries for dashboard
// @Tags Dashboard
// @Accept json
// @Produce json
// @Param limit query int false "Limit" default(5)
// @Success 200 {object} DashboardDataResponse{data=[]ClusterSummary}
// @Failure 500 {object} DashboardDataResponse
// @Router /api/v1/dashboard/overview/clusters [get]
func (h *OverviewHandler) GetClusterSummaries(c *gin.Context) {
	limit := 5

	summaries, err := h.service.GetClusterSummaries(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, DashboardDataResponse{
			ErrorMsg: "Failed to get cluster summaries: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, DashboardDataResponse{Data: summaries})
}

// GetHostSummaries godoc
// @Summary Get host summaries
// @Description Get host summaries for dashboard
// @Tags Dashboard
// @Accept json
// @Produce json
// @Param limit query int false "Limit" default(5)
// @Success 200 {object} DashboardDataResponse{data=[]HostSummary}
// @Failure 500 {object} DashboardDataResponse
// @Router /api/v1/dashboard/overview/hosts [get]
func (h *OverviewHandler) GetHostSummaries(c *gin.Context) {
	limit := 5

	summaries, err := h.service.GetHostSummaries(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, DashboardDataResponse{
			ErrorMsg: "Failed to get host summaries: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, DashboardDataResponse{Data: summaries})
}

// GetRecentActivities godoc
// @Summary Get recent activities
// @Description Get recent audit log activities for dashboard
// @Tags Dashboard
// @Accept json
// @Produce json
// @Param limit query int false "Limit" default(10)
// @Success 200 {object} DashboardDataResponse{data=[]RecentActivity}
// @Failure 500 {object} DashboardDataResponse
// @Router /api/v1/dashboard/overview/activities [get]
func (h *OverviewHandler) GetRecentActivities(c *gin.Context) {
	limit := 10

	activities, err := h.service.GetRecentActivities(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, DashboardDataResponse{
			ErrorMsg: "Failed to get recent activities: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, DashboardDataResponse{Data: activities})
}

// GetOverviewData godoc
// @Summary Get complete dashboard overview data
// @Description Get complete dashboard overview data including stats, clusters, hosts, and activities
// @Tags Dashboard
// @Accept json
// @Produce json
// @Success 200 {object} DashboardDataResponse{data=OverviewData}
// @Failure 500 {object} DashboardDataResponse
// @Router /api/v1/dashboard/overview [get]
func (h *OverviewHandler) GetOverviewData(c *gin.Context) {
	data, err := h.service.GetOverviewData(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, DashboardDataResponse{
			ErrorMsg: "Failed to get overview data: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, DashboardDataResponse{Data: data})
}
