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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestFetchDocsViaMCP tests fetching documentation via MCP server.
// TestFetchDocsViaMCP 测试通过 MCP 服务器获取文档。
func TestFetchDocsViaMCP(t *testing.T) {
	// Create mock MCP server
	// 创建模拟 MCP 服务器
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		// 验证请求
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Parse request body
		// 解析请求体
		var req MCPRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		// Verify request parameters
		// 验证请求参数
		if req.Action != "deepwiki_fetch" {
			t.Errorf("Expected action deepwiki_fetch, got %s", req.Action)
		}

		if req.Params["url"] != "https://deepwiki.com/apache/seatunnel" {
			t.Errorf("Expected URL https://deepwiki.com/apache/seatunnel, got %s", req.Params["url"])
		}

		// Return mock response
		// 返回模拟响应
		response := DeepWikiResponse{
			Status:     "ok",
			Data:       "# Apache SeaTunnel\n\nThis is mock documentation content.",
			TotalPages: 1,
			TotalBytes: 100,
			ElapsedMs:  50,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create service with mock server
	// 使用模拟服务器创建服务
	service := NewService(ServiceConfig{
		MCPServerURL: mockServer.URL,
		UseMCP:       true,
		Timeout:      10 * time.Second,
	})

	// Test fetch
	// 测试获取
	ctx := context.Background()
	req := &DeepWikiRequest{
		URL:      "apache/seatunnel",
		Mode:     "aggregate",
		MaxDepth: 1,
	}

	result, err := service.FetchDocs(ctx, req)
	if err != nil {
		t.Fatalf("FetchDocs failed: %v", err)
	}

	// Verify response
	// 验证响应
	if result.Status != "ok" {
		t.Errorf("Expected status ok, got %s", result.Status)
	}

	data, ok := result.Data.(string)
	if !ok {
		t.Errorf("Expected data to be string, got %T", result.Data)
	}

	if data == "" {
		t.Error("Expected non-empty data")
	}

	if result.TotalPages != 1 {
		t.Errorf("Expected TotalPages 1, got %d", result.TotalPages)
	}

	t.Logf("Successfully fetched docs: %d bytes, %d pages", result.TotalBytes, result.TotalPages)
}

// TestFetchDocsViaHTTP tests fetching documentation via direct HTTP.
// TestFetchDocsViaHTTP 测试通过直接 HTTP 获取文档。
func TestFetchDocsViaHTTP(t *testing.T) {
	// Create mock HTTP server
	// 创建模拟 HTTP 服务器
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return mock HTML/Markdown content
		// 返回模拟 HTML/Markdown 内容
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<html>
			<head><title>Apache SeaTunnel</title></head>
			<body>
				<h1>Apache SeaTunnel</h1>
				<p>A distributed data integration platform.</p>
			</body>
			</html>
		`))
	}))
	defer mockServer.Close()

	// Create service without MCP
	// 创建不使用 MCP 的服务
	service := NewService(ServiceConfig{
		UseMCP:  false,
		Timeout: 10 * time.Second,
	})

	// Test fetch with mock server URL
	// 使用模拟服务器 URL 测试获取
	ctx := context.Background()
	req := &DeepWikiRequest{
		URL:  mockServer.URL,
		Mode: "aggregate",
	}

	result, err := service.FetchDocs(ctx, req)
	if err != nil {
		t.Fatalf("FetchDocs failed: %v", err)
	}

	// Verify response
	// 验证响应
	if result.Status != "ok" {
		t.Errorf("Expected status ok, got %s", result.Status)
	}

	if result.TotalBytes == 0 {
		t.Error("Expected non-zero TotalBytes")
	}

	t.Logf("Successfully fetched docs via HTTP: %d bytes", result.TotalBytes)
}

// TestSearch tests the search functionality.
// TestSearch 测试搜索功能。
func TestSearch(t *testing.T) {
	// Create mock server
	// 创建模拟服务器
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for query parameter
		// 检查查询参数
		query := r.URL.Query().Get("q")
		if query == "" {
			t.Log("No query parameter provided")
		}

		// Return mock search results
		// 返回模拟搜索结果
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<h1>Search Results for: ` + query + `</h1>
			<p>Found relevant documentation about connectors.</p>
		`))
	}))
	defer mockServer.Close()

	// Create service
	// 创建服务
	service := NewService(ServiceConfig{
		UseMCP:  false,
		Timeout: 10 * time.Second,
	})

	// Test search
	// 测试搜索
	ctx := context.Background()
	req := &SearchRequest{
		Query: "jdbc connector",
		Repo:  mockServer.URL,
	}

	result, err := service.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Verify response
	// 验证响应
	if result.Query != "jdbc connector" {
		t.Errorf("Expected query 'jdbc connector', got '%s'", result.Query)
	}

	if result.Results == "" {
		t.Error("Expected non-empty results")
	}

	if result.Source != "deepwiki" {
		t.Errorf("Expected source 'deepwiki', got '%s'", result.Source)
	}

	t.Logf("Search completed successfully: %d bytes of results", len(result.Results))
}

// TestMCPRequestFormat tests the MCP request format.
// TestMCPRequestFormat 测试 MCP 请求格式。
func TestMCPRequestFormat(t *testing.T) {
	req := MCPRequest{
		ID:     "test-123",
		Action: "deepwiki_fetch",
		Params: map[string]string{
			"url":      "https://deepwiki.com/apache/seatunnel",
			"mode":     "aggregate",
			"maxDepth": "1",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal MCPRequest: %v", err)
	}

	// Verify JSON format
	// 验证 JSON 格式
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if parsed["id"] != "test-123" {
		t.Errorf("Expected id 'test-123', got '%v'", parsed["id"])
	}

	if parsed["action"] != "deepwiki_fetch" {
		t.Errorf("Expected action 'deepwiki_fetch', got '%v'", parsed["action"])
	}

	params, ok := parsed["params"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected params to be a map")
	}

	if params["url"] != "https://deepwiki.com/apache/seatunnel" {
		t.Errorf("Expected url 'https://deepwiki.com/apache/seatunnel', got '%v'", params["url"])
	}

	t.Logf("MCP request format is correct: %s", string(data))
}

// TestServiceConfig tests service configuration.
// TestServiceConfig 测试服务配置。
func TestServiceConfig(t *testing.T) {
	// Test default configuration
	// 测试默认配置
	service := NewService(ServiceConfig{})

	if service.httpClient == nil {
		t.Error("Expected httpClient to be initialized")
	}

	if service.mcpServerURL != MCPServerURL {
		t.Errorf("Expected default MCPServerURL %s, got %s", MCPServerURL, service.mcpServerURL)
	}

	// Test custom configuration
	// 测试自定义配置
	customURL := "http://custom-mcp:8080/mcp"
	service2 := NewService(ServiceConfig{
		MCPServerURL: customURL,
		UseMCP:       true,
		Timeout:      60 * time.Second,
	})

	if service2.mcpServerURL != customURL {
		t.Errorf("Expected custom MCPServerURL %s, got %s", customURL, service2.mcpServerURL)
	}

	if !service2.useMCP {
		t.Error("Expected useMCP to be true")
	}

	t.Log("Service configuration tests passed")
}

// TestDeepWikiResponseParsing tests parsing of DeepWiki responses.
// TestDeepWikiResponseParsing 测试 DeepWiki 响应解析。
func TestDeepWikiResponseParsing(t *testing.T) {
	// Test successful response
	// 测试成功响应
	successJSON := `{
		"status": "ok",
		"data": "# Documentation\n\nContent here.",
		"totalPages": 5,
		"totalBytes": 25000,
		"elapsedMs": 1200
	}`

	var successResp DeepWikiResponse
	if err := json.Unmarshal([]byte(successJSON), &successResp); err != nil {
		t.Fatalf("Failed to parse success response: %v", err)
	}

	if successResp.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", successResp.Status)
	}

	// Test error response
	// 测试错误响应
	errorJSON := `{
		"status": "error",
		"code": "DOMAIN_NOT_ALLOWED",
		"message": "Only deepwiki.com domains are allowed"
	}`

	var errorResp DeepWikiResponse
	if err := json.Unmarshal([]byte(errorJSON), &errorResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errorResp.Status != "error" {
		t.Errorf("Expected status 'error', got '%s'", errorResp.Status)
	}

	if errorResp.Code != "DOMAIN_NOT_ALLOWED" {
		t.Errorf("Expected code 'DOMAIN_NOT_ALLOWED', got '%s'", errorResp.Code)
	}

	// Test partial response
	// 测试部分响应
	partialJSON := `{
		"status": "partial",
		"data": "# Partial Content",
		"errors": [
			{"url": "https://deepwiki.com/user/repo/page2", "reason": "HTTP error: 404"}
		],
		"totalPages": 1,
		"totalBytes": 5000,
		"elapsedMs": 950
	}`

	var partialResp DeepWikiResponse
	if err := json.Unmarshal([]byte(partialJSON), &partialResp); err != nil {
		t.Fatalf("Failed to parse partial response: %v", err)
	}

	if partialResp.Status != "partial" {
		t.Errorf("Expected status 'partial', got '%s'", partialResp.Status)
	}

	if len(partialResp.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(partialResp.Errors))
	}

	t.Log("Response parsing tests passed")
}
