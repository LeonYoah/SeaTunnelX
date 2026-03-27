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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	clusterapp "github.com/seatunnel/seatunnelX/internal/apps/cluster"
)

// ConfigToolClient talks to seatunnelx-java-proxy config APIs.
type ConfigToolClient interface {
	InspectDAG(ctx context.Context, endpoint string, req *ConfigToolContentRequest) (*ConfigToolDAGResponse, error)
	InspectWebUIDAG(ctx context.Context, endpoint string, req *ConfigToolContentRequest) (*ConfigToolWebUIDAGResponse, error)
	ValidateConfig(ctx context.Context, endpoint string, req *ConfigToolValidateRequest) (*ConfigToolValidateResponse, error)
	DeriveSourcePreview(ctx context.Context, endpoint string, req *ConfigToolPreviewRequest) (*ConfigToolPreviewResponse, error)
	DeriveTransformPreview(ctx context.Context, endpoint string, req *ConfigToolPreviewRequest) (*ConfigToolPreviewResponse, error)
}

// ConfigToolResolver resolves java-proxy endpoint for a sync task.
type ConfigToolResolver interface {
	ResolveConfigToolEndpoint(ctx context.Context, clusterID uint, taskDefinition JSONMap) (string, error)
}

// ConfigToolContentRequest represents shared content/filePath input.
type ConfigToolContentRequest struct {
	Content       string   `json:"content,omitempty"`
	ContentFormat string   `json:"contentFormat,omitempty"`
	FilePath      string   `json:"filePath,omitempty"`
	Variables     []string `json:"variables,omitempty"`
}

// ConfigToolPreviewRequest represents preview derive request.
type ConfigToolPreviewRequest struct {
	ConfigToolContentRequest
	SourceNodeID          string                 `json:"sourceNodeId,omitempty"`
	SourceIndex           *int                   `json:"sourceIndex,omitempty"`
	TransformNodeID       string                 `json:"transformNodeId,omitempty"`
	TransformIndex        *int                   `json:"transformIndex,omitempty"`
	OutputFormat          string                 `json:"outputFormat,omitempty"`
	MetadataOutputDataset string                 `json:"metadataOutputDataset,omitempty"`
	MetadataFields        map[string]interface{} `json:"metadataFields,omitempty"`
	EnvOverrides          map[string]interface{} `json:"envOverrides,omitempty"`
	HttpSink              map[string]interface{} `json:"httpSink,omitempty"`
}

// ConfigToolValidateRequest represents config validation / connection test input.
type ConfigToolValidateRequest struct {
	ConfigToolContentRequest
	TestConnection bool `json:"testConnection"`
}

// ConfigToolGraph mirrors java-proxy DAG payload.
type ConfigToolGraph struct {
	Nodes []map[string]interface{} `json:"nodes"`
	Edges []map[string]interface{} `json:"edges"`
}

// ConfigToolDAGResponse mirrors java-proxy dag response.
type ConfigToolDAGResponse struct {
	OK             bool            `json:"ok"`
	Message        string          `json:"message,omitempty"`
	SimpleGraph    bool            `json:"simpleGraph"`
	SourceCount    int             `json:"sourceCount"`
	TransformCount int             `json:"transformCount"`
	SinkCount      int             `json:"sinkCount"`
	Warnings       []string        `json:"warnings"`
	Graph          ConfigToolGraph `json:"graph"`
}

// ConfigToolPreviewResponse mirrors java-proxy preview response.
type ConfigToolPreviewResponse struct {
	OK             bool                   `json:"ok"`
	Message        string                 `json:"message,omitempty"`
	Mode           string                 `json:"mode,omitempty"`
	SelectedNodeID string                 `json:"selectedNodeId,omitempty"`
	SelectedIndex  int                    `json:"selectedIndex,omitempty"`
	Warnings       []string               `json:"warnings"`
	Content        string                 `json:"content,omitempty"`
	ContentFormat  string                 `json:"contentFormat,omitempty"`
	Config         map[string]interface{} `json:"config,omitempty"`
	Graph          ConfigToolGraph        `json:"graph"`
	SimpleGraph    bool                   `json:"simpleGraph"`
}

// ConfigToolWebUIDAGVertexInfo mirrors webui vertex payload from java-proxy.
type ConfigToolWebUIDAGVertexInfo struct {
	VertexID      int      `json:"vertexId"`
	Type          string   `json:"type"`
	ConnectorType string   `json:"connectorType"`
	TablePaths    []string `json:"tablePaths"`
}

// ConfigToolWebUIDAGEdge mirrors webui edge payload from java-proxy.
type ConfigToolWebUIDAGEdge struct {
	InputVertexID  int `json:"inputVertexId"`
	TargetVertexID int `json:"targetVertexId"`
}

// ConfigToolWebUIJobDAG mirrors webui compatible jobDag payload.
type ConfigToolWebUIJobDAG struct {
	JobID         string                                  `json:"jobId"`
	PipelineEdges map[string][]ConfigToolWebUIDAGEdge     `json:"pipelineEdges"`
	VertexInfoMap map[string]ConfigToolWebUIDAGVertexInfo `json:"vertexInfoMap"`
	EnvOptions    map[string]interface{}                  `json:"envOptions"`
}

// ConfigToolWebUIDAGResponse mirrors java-proxy webui dag preview response.
type ConfigToolWebUIDAGResponse struct {
	JobID          string                 `json:"jobId"`
	JobName        string                 `json:"jobName"`
	JobStatus      string                 `json:"jobStatus"`
	ErrorMsg       string                 `json:"errorMsg"`
	CreateTime     string                 `json:"createTime"`
	FinishTime     string                 `json:"finishTime"`
	JobDag         ConfigToolWebUIJobDAG  `json:"jobDag"`
	Metrics        map[string]interface{} `json:"metrics"`
	PluginJarsUrls []string               `json:"pluginJarsUrls"`
	SimpleGraph    bool                   `json:"simpleGraph"`
	Warnings       []string               `json:"warnings"`
}

// ConfigToolValidationCheck represents one connector validation result.
type ConfigToolValidationCheck struct {
	NodeID        string `json:"nodeId"`
	Kind          string `json:"kind"`
	ConnectorType string `json:"connectorType"`
	Target        string `json:"target"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

// ConfigToolValidateResponse mirrors java-proxy config validation response.
type ConfigToolValidateResponse struct {
	OK       bool                        `json:"ok"`
	Valid    bool                        `json:"valid"`
	Summary  string                      `json:"summary"`
	Errors   []string                    `json:"errors"`
	Warnings []string                    `json:"warnings"`
	Checks   []ConfigToolValidationCheck `json:"checks"`
}

// ConfigToolError captures structured proxy request failures.
type ConfigToolError struct {
	StatusCode int
	Path       string
	Message    string
	RawBody    string
	ErrorType  string
}

func (e *ConfigToolError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("sync: config tool request failed: status=%d body=%s", e.StatusCode, e.RawBody)
}

// DefaultConfigToolClient is the default java-proxy HTTP client.
type DefaultConfigToolClient struct {
	httpClient *http.Client
}

// NewDefaultConfigToolClient creates a config tool client.
func NewDefaultConfigToolClient() *DefaultConfigToolClient {
	return &DefaultConfigToolClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// InspectDAG calls /api/v1/config/dag.
func (c *DefaultConfigToolClient) InspectDAG(ctx context.Context, endpoint string, req *ConfigToolContentRequest) (*ConfigToolDAGResponse, error) {
	var result ConfigToolDAGResponse
	if err := c.postJSON(ctx, endpoint, "/api/v1/config/dag", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// InspectWebUIDAG calls /api/v1/config/webui-dag.
func (c *DefaultConfigToolClient) InspectWebUIDAG(ctx context.Context, endpoint string, req *ConfigToolContentRequest) (*ConfigToolWebUIDAGResponse, error) {
	var result ConfigToolWebUIDAGResponse
	if err := c.postJSON(ctx, endpoint, "/api/v1/config/webui-dag", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ValidateConfig calls /api/v1/config/validate.
func (c *DefaultConfigToolClient) ValidateConfig(ctx context.Context, endpoint string, req *ConfigToolValidateRequest) (*ConfigToolValidateResponse, error) {
	var result ConfigToolValidateResponse
	if err := c.postJSON(ctx, endpoint, "/api/v1/config/validate", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeriveSourcePreview calls /api/v1/config/preview/source.
func (c *DefaultConfigToolClient) DeriveSourcePreview(ctx context.Context, endpoint string, req *ConfigToolPreviewRequest) (*ConfigToolPreviewResponse, error) {
	var result ConfigToolPreviewResponse
	if err := c.postJSON(ctx, endpoint, "/api/v1/config/preview/source", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeriveTransformPreview calls /api/v1/config/preview/transform.
func (c *DefaultConfigToolClient) DeriveTransformPreview(ctx context.Context, endpoint string, req *ConfigToolPreviewRequest) (*ConfigToolPreviewResponse, error) {
	var result ConfigToolPreviewResponse
	if err := c.postJSON(ctx, endpoint, "/api/v1/config/preview/transform", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *DefaultConfigToolClient) postJSON(ctx context.Context, endpoint string, path string, payload interface{}, out interface{}) error {
	if strings.TrimSpace(endpoint) == "" {
		return fmt.Errorf("sync: config tool endpoint is required")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	targetURL := strings.TrimRight(strings.TrimSpace(endpoint), "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseConfigToolError(resp.StatusCode, path, respBody)
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return err
	}
	return nil
}

func parseConfigToolError(statusCode int, path string, body []byte) error {
	trimmed := strings.TrimSpace(string(body))
	payload := struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
	}{}
	if err := json.Unmarshal(body, &payload); err == nil && strings.TrimSpace(payload.Message) != "" {
		message := strings.TrimSpace(payload.Message)
		return &ConfigToolError{
			StatusCode: statusCode,
			Path:       path,
			Message:    humanizeConfigToolMessage(path, message),
			RawBody:    trimmed,
			ErrorType:  inferConfigToolErrorType(message),
		}
	}
	return &ConfigToolError{
		StatusCode: statusCode,
		Path:       path,
		Message:    fmt.Sprintf("sync: config tool request failed: status=%d body=%s", statusCode, trimmed),
		RawBody:    trimmed,
		ErrorType:  "config_tool_request_failed",
	}
}

func inferConfigToolErrorType(message string) string {
	normalized := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(normalized, "config parse failed"):
		return "config_parse_error"
	case strings.Contains(normalized, "expecting close brace"),
		strings.Contains(normalized, "json does not allow unescaped newline"),
		strings.Contains(normalized, "configexception"):
		return "config_parse_error"
	default:
		return "config_tool_request_failed"
	}
}

func humanizeConfigToolMessage(path string, message string) string {
	trimmed := strings.TrimSpace(message)
	if inferConfigToolErrorType(trimmed) == "config_parse_error" {
		detail := strings.TrimSpace(strings.TrimPrefix(trimmed, "Config parse failed:"))
		detail = strings.TrimSpace(strings.TrimPrefix(detail, "Config dag inspection failed:"))
		if detail == "" {
			return "sync: 配置解析失败，请检查括号、引号、逗号和换行"
		}
		return fmt.Sprintf("sync: 配置解析失败，请检查括号、引号、逗号和换行：%s", detail)
	}
	switch path {
	case "/api/v1/config/webui-dag":
		return fmt.Sprintf("sync: DAG 解析失败：%s", trimmed)
	case "/api/v1/config/validate":
		return fmt.Sprintf("sync: 配置校验失败：%s", trimmed)
	default:
		return fmt.Sprintf("sync: config tool request failed: status? message=%s", trimmed)
	}
}

// DefaultConfigToolResolver resolves java-proxy endpoint using cluster managed status or task override.
type DefaultConfigToolResolver struct {
	clusterService *clusterapp.Service
}

// NewDefaultConfigToolResolver creates a config tool resolver.
func NewDefaultConfigToolResolver(clusterService *clusterapp.Service) *DefaultConfigToolResolver {
	return &DefaultConfigToolResolver{clusterService: clusterService}
}

// ResolveConfigToolEndpoint resolves java-proxy endpoint.
func (r *DefaultConfigToolResolver) ResolveConfigToolEndpoint(ctx context.Context, clusterID uint, taskDefinition JSONMap) (string, error) {
	if endpoint := strings.TrimSpace(stringValue(taskDefinition, "proxy_base_url", "java_proxy_base_url", "config_tool_base_url")); endpoint != "" {
		return endpoint, nil
	}
	if r == nil || r.clusterService == nil {
		return "", fmt.Errorf("sync: config tool resolver is not configured")
	}
	status, err := r.clusterService.GetSeatunnelXJavaProxyStatus(ctx, clusterID)
	if err != nil {
		return "", err
	}
	if status == nil || strings.TrimSpace(status.Endpoint) == "" {
		return "", fmt.Errorf("sync: seatunnelx-java-proxy endpoint is empty for cluster %d", clusterID)
	}
	return strings.TrimSpace(status.Endpoint), nil
}
