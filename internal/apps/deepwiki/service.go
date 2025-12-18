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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultRepo is the default repository for SeaTunnel documentation.
	// DefaultRepo 是 SeaTunnel 文档的默认仓库。
	DefaultRepo = "apache/seatunnel"

	// DeepWikiBaseURL is the base URL for DeepWiki.
	// DeepWikiBaseURL 是 DeepWiki 的基础 URL。
	DeepWikiBaseURL = "https://deepwiki.com"

	// MCPServerURL is the URL for the DeepWiki MCP server (if running locally).
	// MCPServerURL 是 DeepWiki MCP 服务器的 URL（如果本地运行）。
	MCPServerURL = "http://localhost:3000/mcp"
)

// Service provides DeepWiki documentation services.
// Service 提供 DeepWiki 文档服务。
type Service struct {
	httpClient   *http.Client
	mcpServerURL string
	useMCP       bool
}

// ServiceConfig contains configuration for the DeepWiki service.
// ServiceConfig 包含 DeepWiki 服务的配置。
type ServiceConfig struct {
	// MCPServerURL is the URL of the MCP server (optional)
	// MCPServerURL 是 MCP 服务器的 URL（可选）
	MCPServerURL string

	// UseMCP indicates whether to use MCP server or direct HTTP
	// UseMCP 表示是否使用 MCP 服务器或直接 HTTP
	UseMCP bool

	// Timeout is the HTTP client timeout
	// Timeout 是 HTTP 客户端超时时间
	Timeout time.Duration
}

// NewService creates a new DeepWiki service.
// NewService 创建新的 DeepWiki 服务。
func NewService(config ServiceConfig) *Service {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	mcpURL := config.MCPServerURL
	if mcpURL == "" {
		mcpURL = MCPServerURL
	}

	return &Service{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		mcpServerURL: mcpURL,
		useMCP:       config.UseMCP,
	}
}

// MCPRequest represents a request to the MCP server.
// MCPRequest 表示发送到 MCP 服务器的请求。
type MCPRequest struct {
	ID     string            `json:"id"`
	Action string            `json:"action"`
	Params map[string]string `json:"params"`
}

// FetchDocs fetches documentation from DeepWiki.
// FetchDocs 从 DeepWiki 获取文档。
func (s *Service) FetchDocs(ctx context.Context, req *DeepWikiRequest) (*DeepWikiResponse, error) {
	if s.useMCP {
		return s.fetchViaMCP(ctx, req)
	}
	return s.fetchViaHTTP(ctx, req)
}

// fetchViaMCP fetches documentation via the MCP server.
// fetchViaMCP 通过 MCP 服务器获取文档。
func (s *Service) fetchViaMCP(ctx context.Context, req *DeepWikiRequest) (*DeepWikiResponse, error) {
	// Build MCP request
	// 构建 MCP 请求
	mode := req.Mode
	if mode == "" {
		mode = "aggregate"
	}

	maxDepth := "1"
	if req.MaxDepth > 0 {
		maxDepth = fmt.Sprintf("%d", req.MaxDepth)
	}

	url := req.URL
	if !strings.HasPrefix(url, "https://") {
		url = fmt.Sprintf("%s/%s", DeepWikiBaseURL, url)
	}

	mcpReq := MCPRequest{
		ID:     fmt.Sprintf("req-%d", time.Now().UnixNano()),
		Action: "deepwiki_fetch",
		Params: map[string]string{
			"url":      url,
			"mode":     mode,
			"maxDepth": maxDepth,
		},
	}

	body, err := json.Marshal(mcpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MCP request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.mcpServerURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send MCP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read MCP response: %w", err)
	}

	var result DeepWikiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal MCP response: %w", err)
	}

	return &result, nil
}

// fetchViaHTTP fetches documentation via direct HTTP (fallback).
// fetchViaHTTP 通过直接 HTTP 获取文档（备用方案）。
func (s *Service) fetchViaHTTP(ctx context.Context, req *DeepWikiRequest) (*DeepWikiResponse, error) {
	// Build URL for DeepWiki API
	// 构建 DeepWiki API 的 URL
	url := req.URL
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		url = fmt.Sprintf("%s/%s", DeepWikiBaseURL, url)
	}

	// Add query parameter if provided
	// 如果提供了查询参数则添加
	if req.Query != "" {
		if strings.Contains(url, "?") {
			url += "&q=" + req.Query
		} else {
			url += "?q=" + req.Query
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from DeepWiki: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &DeepWikiResponse{
			Status:  "error",
			Code:    fmt.Sprintf("HTTP_%d", resp.StatusCode),
			Message: fmt.Sprintf("HTTP error: %s", resp.Status),
		}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return &DeepWikiResponse{
		Status:     "ok",
		Data:       string(body),
		TotalBytes: len(body),
	}, nil
}

// Search searches for documentation in DeepWiki.
// Search 在 DeepWiki 中搜索文档。
func (s *Service) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	repo := req.Repo
	if repo == "" {
		repo = DefaultRepo
	}

	// Fetch docs with query
	// 使用查询获取文档
	fetchReq := &DeepWikiRequest{
		URL:      repo,
		Mode:     "aggregate",
		MaxDepth: 0,
		Query:    req.Query,
	}

	result, err := s.FetchDocs(ctx, fetchReq)
	if err != nil {
		return nil, err
	}

	// Extract relevant content
	// 提取相关内容
	var content string
	switch v := result.Data.(type) {
	case string:
		content = v
	default:
		contentBytes, _ := json.Marshal(result.Data)
		content = string(contentBytes)
	}

	return &SearchResponse{
		Query:   req.Query,
		Results: content,
		Source:  "deepwiki",
	}, nil
}

// GetSeaTunnelDocs fetches SeaTunnel documentation.
// GetSeaTunnelDocs 获取 SeaTunnel 文档。
func (s *Service) GetSeaTunnelDocs(ctx context.Context) (*DeepWikiResponse, error) {
	return s.FetchDocs(ctx, &DeepWikiRequest{
		URL:      DefaultRepo,
		Mode:     "aggregate",
		MaxDepth: 0,
	})
}
