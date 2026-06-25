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

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/seatunnel/seatunnelX/internal/config"
	seatunnelmeta "github.com/seatunnel/seatunnelX/internal/seatunnel"
)

const seatunnelXJavaProxyDefaultPort = 18080

// SeatunnelXJavaProxyStatus represents the managed seatunnelx-java-proxy state for a cluster.
type SeatunnelXJavaProxyStatus struct {
	ClusterID      uint   `json:"cluster_id"`
	ClusterName    string `json:"cluster_name,omitempty"`
	NodeID         uint   `json:"node_id,omitempty"`
	HostID         uint   `json:"host_id,omitempty"`
	HostName       string `json:"host_name,omitempty"`
	HostIP         string `json:"host_ip,omitempty"`
	Role           string `json:"role,omitempty"`
	InstallDir     string `json:"install_dir,omitempty"`
	Version        string `json:"version,omitempty"`
	Service        string `json:"service,omitempty"`
	Managed        bool   `json:"managed"`
	Running        bool   `json:"running"`
	Healthy        bool   `json:"healthy"`
	Endpoint       string `json:"endpoint,omitempty"`
	LocalEndpoint  string `json:"local_endpoint,omitempty"`
	DirectEndpoint string `json:"direct_endpoint,omitempty"`
	Port           int    `json:"port,omitempty"`
	PID            int    `json:"pid,omitempty"`
	LogPath        string `json:"log_path,omitempty"`
	Message        string `json:"message,omitempty"`
}

// SeatunnelXJavaProxyLogPreviewResult represents a service.log preview result.
// SeatunnelXJavaProxyLogPreviewResult 表示 service.log 预览结果。
type SeatunnelXJavaProxyLogPreviewResult struct {
	ClusterID uint   `json:"cluster_id"`
	LogPath   string `json:"log_path,omitempty"`
	Lines     int    `json:"lines,omitempty"`
	Logs      string `json:"logs,omitempty"`
}

func (s *Service) GetSeatunnelXJavaProxyStatus(ctx context.Context, clusterID uint, nodeID ...uint) (*SeatunnelXJavaProxyStatus, error) {
	return s.executeSeatunnelXJavaProxyCommand(ctx, clusterID, "status", optionalNodeID(nodeID...))
}

func (s *Service) StartSeatunnelXJavaProxy(ctx context.Context, clusterID uint, nodeID ...uint) (*SeatunnelXJavaProxyStatus, error) {
	return s.executeSeatunnelXJavaProxyCommand(ctx, clusterID, "start", optionalNodeID(nodeID...))
}

func (s *Service) StopSeatunnelXJavaProxy(ctx context.Context, clusterID uint, nodeID ...uint) (*SeatunnelXJavaProxyStatus, error) {
	return s.executeSeatunnelXJavaProxyCommand(ctx, clusterID, "stop", optionalNodeID(nodeID...))
}

func (s *Service) RestartSeatunnelXJavaProxy(ctx context.Context, clusterID uint, nodeID ...uint) (*SeatunnelXJavaProxyStatus, error) {
	return s.executeSeatunnelXJavaProxyCommand(ctx, clusterID, "restart", optionalNodeID(nodeID...))
}

func (s *Service) InstallOrRepairSeatunnelXJavaProxy(ctx context.Context, clusterID uint, nodeID ...uint) (*SeatunnelXJavaProxyStatus, error) {
	if s.agentSender == nil {
		return nil, fmt.Errorf("agent sender is not configured")
	}
	if s.hostProvider == nil {
		return nil, fmt.Errorf("host provider is not configured")
	}
	clusterInfo, err := s.repo.GetByID(ctx, clusterID, false)
	if err != nil {
		return nil, err
	}
	node, hostInfo, err := s.pickSeatunnelXJavaProxyNode(ctx, clusterID, optionalNodeID(nodeID...))
	if err != nil {
		return nil, err
	}
	version := seatunnelmeta.ResolveSeatunnelXJavaProxyVersion(clusterInfo.Version)
	assetBaseURL := strings.TrimRight(config.GetExternalURL(), "/")
	if assetBaseURL == "" {
		return decodeSeatunnelXJavaProxyStatus(clusterInfo, node, hostInfo, ""), fmt.Errorf("app.external_url is required to install seatunnelx-java-proxy assets")
	}
	params := map[string]string{
		"sub_command": "seatunnelx_java_proxy_install",
		"service":     "seatunnelx_java_proxy",
		"cluster_id":  fmt.Sprintf("%d", clusterID),
		"node_id":     fmt.Sprintf("%d", node.ID),
		"version":     version,
		"jar_url":     fmt.Sprintf("%s/api/v1/agent/assets/seatunnelx-java-proxy.jar?version=%s", assetBaseURL, url.QueryEscape(version)),
		"script_url":  fmt.Sprintf("%s/api/v1/agent/assets/seatunnelx-java-proxy.sh", assetBaseURL),
	}
	success, message, sendErr := s.agentSender.SendCommand(ctx, hostInfo.AgentID, "seatunnelx_java_proxy_install", params)
	status := decodeSeatunnelXJavaProxyStatus(clusterInfo, node, hostInfo, "")
	if sendErr != nil {
		status.Message = firstNonEmpty(errorString(sendErr), status.Message)
		return status, sendErr
	}
	if !success {
		status.Message = firstNonEmpty(parseCommandMessage(message), message, "seatunnelx-java-proxy asset install failed")
		return status, fmt.Errorf("%s", status.Message)
	}
	refreshed, statusErr := s.GetSeatunnelXJavaProxyStatus(ctx, clusterID, node.ID)
	if statusErr == nil && refreshed != nil {
		status = refreshed
	}
	status.Message = firstNonEmpty(parseCommandMessage(message), "seatunnelx-java-proxy assets installed or repaired")
	return status, nil
}

func (s *Service) GetSeatunnelXJavaProxyServiceLog(
	ctx context.Context,
	clusterID uint,
	lines int,
	nodeID ...uint,
) (*SeatunnelXJavaProxyLogPreviewResult, error) {
	if lines <= 0 {
		lines = 200
	}
	clusterInfo, err := s.repo.GetByID(ctx, clusterID, false)
	if err != nil {
		return nil, err
	}
	node, hostInfo, err := s.pickSeatunnelXJavaProxyNode(ctx, clusterID, optionalNodeID(nodeID...))
	if err != nil {
		return nil, err
	}
	status, err := s.GetSeatunnelXJavaProxyStatus(ctx, clusterID, optionalNodeID(nodeID...))
	if err != nil && status == nil {
		return nil, err
	}
	logPath := strings.TrimSpace(status.LogPath)
	if logPath == "" {
		installDir := strings.TrimSpace(node.InstallDir)
		if installDir == "" {
			installDir = strings.TrimSpace(clusterInfo.InstallDir)
		}
		if installDir == "" {
			installDir = "/opt/seatunnel"
		}
		logPath = fmt.Sprintf("%s/.seatunnelx/seatunnelx-java-proxy/service.log", installDir)
	}
	success, message, sendErr := s.agentSender.SendCommand(ctx, hostInfo.AgentID, "get_logs", map[string]string{
		"log_file": logPath,
		"lines":    fmt.Sprintf("%d", lines),
		"mode":     "tail",
	})
	result := &SeatunnelXJavaProxyLogPreviewResult{
		ClusterID: clusterID,
		LogPath:   logPath,
		Lines:     lines,
		Logs:      message,
	}
	if sendErr != nil {
		return result, fmt.Errorf("failed to preview seatunnelx-java-proxy service log: %w", sendErr)
	}
	if !success {
		return result, fmt.Errorf("%s", firstNonEmpty(message, "failed to preview seatunnelx-java-proxy service log"))
	}
	return result, nil
}

func (s *Service) executeSeatunnelXJavaProxyCommand(ctx context.Context, clusterID uint, commandType string, nodeID uint) (*SeatunnelXJavaProxyStatus, error) {
	if s.agentSender == nil {
		return nil, fmt.Errorf("agent sender is not configured")
	}
	if s.hostProvider == nil {
		return nil, fmt.Errorf("host provider is not configured")
	}

	clusterInfo, err := s.repo.GetByID(ctx, clusterID, false)
	if err != nil {
		return nil, err
	}
	node, hostInfo, err := s.pickSeatunnelXJavaProxyNode(ctx, clusterID, nodeID)
	if err != nil {
		return nil, err
	}

	params := map[string]string{
		"service":     "seatunnelx_java_proxy",
		"cluster_id":  fmt.Sprintf("%d", clusterID),
		"node_id":     fmt.Sprintf("%d", node.ID),
		"install_dir": node.InstallDir,
		"version":     clusterInfo.Version,
	}
	success, message, sendErr := s.agentSender.SendCommand(ctx, hostInfo.AgentID, commandType, params)
	status := decodeSeatunnelXJavaProxyStatus(clusterInfo, node, hostInfo, firstNonEmpty(message, errorString(sendErr)))
	if sendErr != nil {
		return status, sendErr
	}
	if !success {
		return status, fmt.Errorf("%s", firstNonEmpty(status.Message, parseCommandMessage(message), "seatunnelx-java-proxy command failed"))
	}
	return status, nil
}

func (s *Service) pickSeatunnelXJavaProxyNode(ctx context.Context, clusterID uint, nodeID uint) (*NodeInfo, *HostInfo, error) {
	nodes, err := s.GetNodes(ctx, clusterID)
	if err != nil {
		return nil, nil, err
	}
	if nodeID > 0 {
		for _, node := range nodes {
			if node == nil || node.ID != nodeID {
				continue
			}
			hostInfo, err := s.hostProvider.GetHostByID(ctx, node.HostID)
			if err != nil {
				return nil, nil, err
			}
			if hostInfo == nil || !hostInfo.IsOnline(s.heartbeatTimeout) || strings.TrimSpace(hostInfo.AgentID) == "" {
				return nil, nil, fmt.Errorf("selected java-proxy deployment node has no online agent")
			}
			return node, hostInfo, nil
		}
		return nil, nil, fmt.Errorf("selected java-proxy deployment node %d was not found", nodeID)
	}
	for _, node := range nodes {
		if node == nil || !node.IsOnline {
			continue
		}
		if node.Role != NodeRoleMaster && node.Role != NodeRoleMasterWorker {
			continue
		}
		hostInfo, err := s.hostProvider.GetHostByID(ctx, node.HostID)
		if err != nil {
			continue
		}
		if hostInfo == nil || !hostInfo.IsOnline(s.heartbeatTimeout) || strings.TrimSpace(hostInfo.AgentID) == "" {
			continue
		}
		return node, hostInfo, nil
	}
	for _, node := range nodes {
		if node == nil || !node.IsOnline {
			continue
		}
		hostInfo, err := s.hostProvider.GetHostByID(ctx, node.HostID)
		if err != nil {
			continue
		}
		if hostInfo == nil || !hostInfo.IsOnline(s.heartbeatTimeout) || strings.TrimSpace(hostInfo.AgentID) == "" {
			continue
		}
		return node, hostInfo, nil
	}
	return nil, nil, fmt.Errorf("no online node with agent available for seatunnelx-java-proxy management")
}

func decodeSeatunnelXJavaProxyStatus(clusterInfo *Cluster, node *NodeInfo, hostInfo *HostInfo, message string) *SeatunnelXJavaProxyStatus {
	status := &SeatunnelXJavaProxyStatus{
		ClusterID:   clusterInfo.ID,
		ClusterName: clusterInfo.Name,
		Version:     clusterInfo.Version,
		Service:     "seatunnelx_java_proxy",
		Message:     parseCommandMessage(message),
	}
	if node != nil {
		status.NodeID = node.ID
		status.HostID = node.HostID
		status.HostName = node.HostName
		status.HostIP = node.HostIP
		status.Role = string(node.Role)
		status.InstallDir = node.InstallDir
	}
	if hostInfo != nil {
		status.HostName = firstNonEmpty(status.HostName, hostInfo.Name)
		status.HostIP = firstNonEmpty(status.HostIP, hostInfo.IPAddress)
	}

	var payload SeatunnelXJavaProxyStatus
	if err := json.Unmarshal([]byte(strings.TrimSpace(message)), &payload); err == nil {
		if payload.Service != "" {
			status.Service = payload.Service
		}
		status.Managed = payload.Managed
		status.Running = payload.Running
		status.Healthy = payload.Healthy
		status.Endpoint = payload.Endpoint
		status.LocalEndpoint = payload.LocalEndpoint
		status.DirectEndpoint = payload.DirectEndpoint
		status.Port = payload.Port
		status.PID = payload.PID
		status.LogPath = payload.LogPath
		status.Message = firstNonEmpty(payload.Message, status.Message)
	}
	if status.LocalEndpoint == "" {
		status.LocalEndpoint = status.Endpoint
	}
	if status.Port <= 0 {
		status.Port = seatunnelXJavaProxyEndpointPort(firstNonEmpty(status.LocalEndpoint, status.Endpoint))
	}
	if status.Port <= 0 {
		status.Port = seatunnelXJavaProxyDefaultPort
	}
	if status.Managed || status.Endpoint == "" || seatunnelXJavaProxyIsLocalEndpoint(status.Endpoint) {
		status.DirectEndpoint = firstNonEmpty(
			status.DirectEndpoint,
			seatunnelXJavaProxyDirectEndpoint(status.HostIP, status.Port),
		)
		status.Endpoint = firstNonEmpty(status.DirectEndpoint, status.Endpoint)
	}
	return status
}

func optionalNodeID(nodeID ...uint) uint {
	if len(nodeID) == 0 {
		return 0
	}
	return nodeID[0]
}

func seatunnelXJavaProxyEndpointPort(endpoint string) int {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return 0
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return 0
	}
	port := strings.TrimSpace(parsed.Port())
	if port == "" {
		return 0
	}
	value, err := strconv.Atoi(port)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func seatunnelXJavaProxyIsLocalEndpoint(endpoint string) bool {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return false
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	host := strings.TrimSpace(parsed.Hostname())
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func seatunnelXJavaProxyDirectEndpoint(host string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" || port <= 0 {
		return ""
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(port))
}
