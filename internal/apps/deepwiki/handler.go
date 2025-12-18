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

package deepwiki

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler handles DeepWiki HTTP requests.
// Handler 处理 DeepWiki HTTP 请求。
type Handler struct {
	service *Service
}

// NewHandler creates a new DeepWiki handler.
// NewHandler 创建新的 DeepWiki 处理器。
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Response is the standard API response.
// Response 是标准 API 响应。
type Response struct {
	Data     interface{} `json:"data,omitempty"`
	ErrorMsg string      `json:"error,omitempty"`
}

// GetDocs godoc
// @Summary Get SeaTunnel documentation from DeepWiki
// @Description Fetch SeaTunnel documentation from DeepWiki
// @Tags DeepWiki
// @Accept json
// @Produce json
// @Success 200 {object} Response{data=DeepWikiResponse}
// @Failure 500 {object} Response
// @Router /api/v1/deepwiki/docs [get]
func (h *Handler) GetDocs(c *gin.Context) {
	result, err := h.service.GetSeaTunnelDocs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			ErrorMsg: "Failed to fetch documentation: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{Data: result})
}

// FetchDocs godoc
// @Summary Fetch documentation from DeepWiki
// @Description Fetch documentation from DeepWiki for a specific repository
// @Tags DeepWiki
// @Accept json
// @Produce json
// @Param request body DeepWikiRequest true "Fetch request"
// @Success 200 {object} Response{data=DeepWikiResponse}
// @Failure 400 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/deepwiki/fetch [post]
func (h *Handler) FetchDocs(c *gin.Context) {
	var req DeepWikiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			ErrorMsg: "Invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.service.FetchDocs(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			ErrorMsg: "Failed to fetch documentation: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{Data: result})
}

// Search godoc
// @Summary Search SeaTunnel documentation
// @Description Search SeaTunnel documentation in DeepWiki
// @Tags DeepWiki
// @Accept json
// @Produce json
// @Param request body SearchRequest true "Search request"
// @Success 200 {object} Response{data=SearchResponse}
// @Failure 400 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/deepwiki/search [post]
func (h *Handler) Search(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			ErrorMsg: "Invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.service.Search(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			ErrorMsg: "Failed to search documentation: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{Data: result})
}
