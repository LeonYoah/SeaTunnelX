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

// DeepWikiRequest represents a request to fetch DeepWiki documentation.
// DeepWikiRequest 表示获取 DeepWiki 文档的请求。
type DeepWikiRequest struct {
	// URL is the repository URL or path (e.g., "apache/seatunnel")
	// URL 是仓库地址或路径（如 "apache/seatunnel"）
	URL string `json:"url" binding:"required"`

	// Mode is the output mode: "aggregate" (single document) or "pages" (structured)
	// Mode 是输出模式："aggregate"（单文档）或 "pages"（结构化）
	Mode string `json:"mode,omitempty"`

	// MaxDepth is the maximum depth of pages to crawl (default: 1)
	// MaxDepth 是爬取页面的最大深度（默认：1）
	MaxDepth int `json:"max_depth,omitempty"`

	// Query is an optional search query to filter results
	// Query 是可选的搜索查询，用于过滤结果
	Query string `json:"query,omitempty"`
}

// DeepWikiResponse represents the response from DeepWiki.
// DeepWikiResponse 表示 DeepWiki 的响应。
type DeepWikiResponse struct {
	// Status is the response status: "ok", "partial", or "error"
	// Status 是响应状态："ok"、"partial" 或 "error"
	Status string `json:"status"`

	// Data contains the markdown content (aggregate mode) or page array (pages mode)
	// Data 包含 markdown 内容（aggregate 模式）或页面数组（pages 模式）
	Data interface{} `json:"data,omitempty"`

	// TotalPages is the number of pages fetched
	// TotalPages 是获取的页面数量
	TotalPages int `json:"total_pages,omitempty"`

	// TotalBytes is the total size of fetched content
	// TotalBytes 是获取内容的总大小
	TotalBytes int `json:"total_bytes,omitempty"`

	// ElapsedMs is the time taken in milliseconds
	// ElapsedMs 是耗时（毫秒）
	ElapsedMs int `json:"elapsed_ms,omitempty"`

	// Errors contains any errors encountered during fetching
	// Errors 包含获取过程中遇到的错误
	Errors []DeepWikiError `json:"errors,omitempty"`

	// Code is the error code (only present on error)
	// Code 是错误代码（仅在错误时存在）
	Code string `json:"code,omitempty"`

	// Message is the error message (only present on error)
	// Message 是错误消息（仅在错误时存在）
	Message string `json:"message,omitempty"`
}

// DeepWikiError represents an error for a specific page.
// DeepWikiError 表示特定页面的错误。
type DeepWikiError struct {
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

// DeepWikiPage represents a single page in pages mode.
// DeepWikiPage 表示 pages 模式下的单个页面。
type DeepWikiPage struct {
	Path     string `json:"path"`
	Markdown string `json:"markdown"`
}

// SearchRequest represents a search request for DeepWiki.
// SearchRequest 表示 DeepWiki 的搜索请求。
type SearchRequest struct {
	// Query is the search query
	// Query 是搜索查询
	Query string `json:"query" binding:"required"`

	// Repo is the repository to search (default: "apache/seatunnel")
	// Repo 是要搜索的仓库（默认："apache/seatunnel"）
	Repo string `json:"repo,omitempty"`
}

// SearchResponse represents the search response.
// SearchResponse 表示搜索响应。
type SearchResponse struct {
	// Query is the original search query
	// Query 是原始搜索查询
	Query string `json:"query"`

	// Results contains the search results
	// Results 包含搜索结果
	Results string `json:"results"`

	// Source indicates the data source
	// Source 表示数据来源
	Source string `json:"source"`
}
