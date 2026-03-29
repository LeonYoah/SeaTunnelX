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
	"net/http"

	"github.com/gin-gonic/gin"
)

type PluginFactoryListResponse struct {
	ErrorMsg string                       `json:"error_msg"`
	Data     *SyncPluginFactoryListResult `json:"data"`
}

type PluginOptionSchemaResponse struct {
	ErrorMsg string                        `json:"error_msg"`
	Data     *SyncPluginOptionSchemaResult `json:"data"`
}

type PluginTemplateResponse struct {
	ErrorMsg string                    `json:"error_msg"`
	Data     *SyncPluginTemplateResult `json:"data"`
}

type PluginEnumValuesResponse struct {
	ErrorMsg string                      `json:"error_msg"`
	Data     *SyncPluginEnumValuesResult `json:"data"`
}

type PluginEnumCatalogResponse struct {
	ErrorMsg string                       `json:"error_msg"`
	Data     *SyncPluginEnumCatalogResult `json:"data"`
}

// ListPluginFactories handles POST /api/v1/sync/plugins/list.
func (h *Handler) ListPluginFactories(c *gin.Context) {
	var req ListSyncPluginFactoriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, PluginFactoryListResponse{ErrorMsg: err.Error()})
		return
	}
	result, err := h.service.ListPluginFactories(c.Request.Context(), &req)
	if err != nil {
		c.JSON(h.getStatusCodeForError(err), PluginFactoryListResponse{ErrorMsg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, PluginFactoryListResponse{Data: result})
}

// GetPluginOptions handles POST /api/v1/sync/plugins/options.
func (h *Handler) GetPluginOptions(c *gin.Context) {
	var req GetSyncPluginOptionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, PluginOptionSchemaResponse{ErrorMsg: err.Error()})
		return
	}
	result, err := h.service.GetPluginOptions(c.Request.Context(), &req)
	if err != nil {
		c.JSON(h.getStatusCodeForError(err), PluginOptionSchemaResponse{ErrorMsg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, PluginOptionSchemaResponse{Data: result})
}

// RenderPluginTemplate handles POST /api/v1/sync/plugins/template.
func (h *Handler) RenderPluginTemplate(c *gin.Context) {
	var req RenderSyncPluginTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, PluginTemplateResponse{ErrorMsg: err.Error()})
		return
	}
	result, err := h.service.RenderPluginTemplate(c.Request.Context(), &req)
	if err != nil {
		c.JSON(h.getStatusCodeForError(err), PluginTemplateResponse{ErrorMsg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, PluginTemplateResponse{Data: result})
}

// ListPluginEnumValues handles POST /api/v1/sync/plugins/enum-values.
func (h *Handler) ListPluginEnumValues(c *gin.Context) {
	var req ListSyncPluginEnumValuesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, PluginEnumValuesResponse{ErrorMsg: err.Error()})
		return
	}
	result, err := h.service.ListPluginEnumValues(c.Request.Context(), &req)
	if err != nil {
		c.JSON(h.getStatusCodeForError(err), PluginEnumValuesResponse{ErrorMsg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, PluginEnumValuesResponse{Data: result})
}

// ListPluginEnumCatalog handles POST /api/v1/sync/plugins/enum-catalog.
func (h *Handler) ListPluginEnumCatalog(c *gin.Context) {
	var req ListSyncPluginEnumCatalogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, PluginEnumCatalogResponse{ErrorMsg: err.Error()})
		return
	}
	result, err := h.service.ListPluginEnumCatalog(c.Request.Context(), &req)
	if err != nil {
		c.JSON(h.getStatusCodeForError(err), PluginEnumCatalogResponse{ErrorMsg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, PluginEnumCatalogResponse{Data: result})
}
