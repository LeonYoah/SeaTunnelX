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
	"strings"
)

// SeatunnelXJavaProxyStatus represents the managed seatunnelx-java-proxy state for a cluster.
type SeatunnelXJavaProxyStatus struct {
	ClusterID   uint   `json:"cluster_id"`
	ClusterName string `json:"cluster_name,omitempty"`
	NodeID      uint   `json:"node_id,omitempty"`
	HostID      uint   `json:"host_id,omitempty"`
	HostName    string `json:"host_name,omitempty"`
	HostIP      string `json:"host_ip,omitempty"`
	Role        string `json:"role,omitempty"`
	InstallDir  string `json:"install_dir,omitempty"`
	Version     string `json:"version,omitempty"`
	Service     string `json:"service,omitempty"`
	Managed     bool   `json:"managed"`
	Running     bool   `json:"running"`
	Healthy     bool   `json:"healthy"`
	Endpoint    string `json:"endpoint,omitempty"`
	Port        int    `json:"port,omitempty"`
	PID         int    `json:"pid,omitempty"`
	LogPath     string `json:"log_path,omitempty"`
	Message     string `json:"message,omitempty"`
}

func (s *Service) GetSeatunnelXJavaProxyStatus(ctx context.Context, clusterID uint) (*SeatunnelXJavaProxyStatus, error) {
	return s.executeSeatunnelXJavaProxyCommand(ctx, clusterID, "status")
}

func (s *Service) StartSeatunnelXJavaProxy(ctx context.Context, clusterID uint) (*SeatunnelXJavaProxyStatus, error) {
	return s.executeSeatunnelXJavaProxyCommand(ctx, clusterID, "start")
}

func (s *Service) StopSeatunnelXJavaProxy(ctx context.Context, clusterID uint) (*SeatunnelXJavaProxyStatus, error) {
	return s.executeSeatunnelXJavaProxyCommand(ctx, clusterID, "stop")
}

func (s *Service) RestartSeatunnelXJavaProxy(ctx context.Context, clusterID uint) (*SeatunnelXJavaProxyStatus, error) {
	return s.executeSeatunnelXJavaProxyCommand(ctx, clusterID, "restart")
}

func (s *Service) executeSeatunnelXJavaProxyCommand(ctx context.Context, clusterID uint, commandType string) (*SeatunnelXJavaProxyStatus, error) {
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
	node, hostInfo, err := s.pickSeatunnelXJavaProxyNode(ctx, clusterID)
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

func (s *Service) pickSeatunnelXJavaProxyNode(ctx context.Context, clusterID uint) (*NodeInfo, *HostInfo, error) {
	nodes, err := s.GetNodes(ctx, clusterID)
	if err != nil {
		return nil, nil, err
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
	return nil, nil, fmt.Errorf("no online master node with agent available for seatunnelx-java-proxy management")
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
		status.Port = payload.Port
		status.PID = payload.PID
		status.LogPath = payload.LogPath
		status.Message = firstNonEmpty(payload.Message, status.Message)
	}
	return status
}
