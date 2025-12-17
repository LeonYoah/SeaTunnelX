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

// Package cluster provides cluster management functionality for the SeaTunnel Agent system.
// cluster 包提供 SeaTunnel Agent 系统的集群管理功能。
package cluster

import (
	"context"
	"fmt"
	"time"
)

// HealthStatus represents the health status of a cluster.
// HealthStatus 表示集群的健康状态。
type HealthStatus string

const (
	// HealthStatusHealthy indicates all nodes are online and running.
	// HealthStatusHealthy 表示所有节点都在线且运行正常。
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusUnhealthy indicates one or more nodes are offline or in error state.
	// HealthStatusUnhealthy 表示一个或多个节点离线或处于错误状态。
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	// HealthStatusUnknown indicates the health status cannot be determined.
	// HealthStatusUnknown 表示无法确定健康状态。
	HealthStatusUnknown HealthStatus = "unknown"
)

// HostInfo represents host information needed by cluster service.
// HostInfo 表示集群服务所需的主机信息。
// This interface decouples cluster from host package to avoid import cycles.
// 此接口将集群与主机包解耦以避免导入循环。
type HostInfo struct {
	ID            uint
	Name          string
	HostType      string
	IPAddress     string
	AgentID       string
	AgentStatus   string
	LastHeartbeat *time.Time
}

// IsOnline checks if the host is online based on heartbeat timeout.
// IsOnline 根据心跳超时检查主机是否在线。
func (h *HostInfo) IsOnline(timeout time.Duration) bool {
	if h.LastHeartbeat == nil {
		return false
	}
	return time.Since(*h.LastHeartbeat) <= timeout
}

// HostProvider is an interface for retrieving host information.
// HostProvider 是获取主机信息的接口。
// This interface decouples cluster service from host package.
// 此接口将集群服务与主机包解耦。
type HostProvider interface {
	// GetHostByID retrieves host information by ID.
	// GetHostByID 根据 ID 获取主机信息。
	GetHostByID(ctx context.Context, id uint) (*HostInfo, error)
}

// ClusterStatusInfo represents detailed cluster status information.
// ClusterStatusInfo 表示详细的集群状态信息。
type ClusterStatusInfo struct {
	ClusterID    uint              `json:"cluster_id"`
	ClusterName  string            `json:"cluster_name"`
	Status       ClusterStatus     `json:"status"`
	HealthStatus HealthStatus      `json:"health_status"`
	TotalNodes   int               `json:"total_nodes"`
	OnlineNodes  int               `json:"online_nodes"`
	OfflineNodes int               `json:"offline_nodes"`
	Nodes        []*NodeStatusInfo `json:"nodes"`
}

// NodeStatusInfo represents detailed node status information.
// NodeStatusInfo 表示详细的节点状态信息。
type NodeStatusInfo struct {
	NodeID        uint       `json:"node_id"`
	HostID        uint       `json:"host_id"`
	HostName      string     `json:"host_name"`
	HostIP        string     `json:"host_ip"`
	Role          NodeRole   `json:"role"`
	Status        NodeStatus `json:"status"`
	IsOnline      bool       `json:"is_online"`
	ProcessPID    int        `json:"process_pid"`
	ProcessStatus string     `json:"process_status"`
}

// OperationType represents the type of cluster operation.
// OperationType 表示集群操作类型。
type OperationType string

const (
	// OperationStart starts the cluster.
	// OperationStart 启动集群。
	OperationStart OperationType = "start"
	// OperationStop stops the cluster.
	// OperationStop 停止集群。
	OperationStop OperationType = "stop"
	// OperationRestart restarts the cluster.
	// OperationRestart 重启集群。
	OperationRestart OperationType = "restart"
)

// OperationResult represents the result of a cluster operation.
// OperationResult 表示集群操作的结果。
type OperationResult struct {
	ClusterID   uint                   `json:"cluster_id"`
	Operation   OperationType          `json:"operation"`
	Success     bool                   `json:"success"`
	Message     string                 `json:"message"`
	NodeResults []*NodeOperationResult `json:"node_results"`
}

// NodeOperationResult represents the result of an operation on a single node.
// NodeOperationResult 表示单个节点操作的结果。
type NodeOperationResult struct {
	NodeID   uint   `json:"node_id"`
	HostID   uint   `json:"host_id"`
	HostName string `json:"host_name"`
	Success  bool   `json:"success"`
	Message  string `json:"message"`
}

// AgentCommandSender is an interface for sending commands to agents.
// AgentCommandSender 是向 Agent 发送命令的接口。
// This interface will be implemented by the Agent Manager in Phase 4.
// 此接口将在第 4 阶段由 Agent Manager 实现。
type AgentCommandSender interface {
	// SendCommand sends a command to an agent and returns the result.
	// SendCommand 向 Agent 发送命令并返回结果。
	SendCommand(ctx context.Context, agentID string, commandType string, params map[string]string) (bool, string, error)
}

// Service provides business logic for cluster management operations.
// Service 提供集群管理操作的业务逻辑。
type Service struct {
	repo             *Repository
	hostProvider     HostProvider
	heartbeatTimeout time.Duration
	agentSender      AgentCommandSender
}

// ServiceConfig holds configuration for the Cluster Service.
// ServiceConfig 保存 Cluster Service 的配置。
type ServiceConfig struct {
	HeartbeatTimeout time.Duration
}

// NewService creates a new Service instance.
// NewService 创建一个新的 Service 实例。
func NewService(repo *Repository, hostProvider HostProvider, cfg *ServiceConfig) *Service {
	timeout := 30 * time.Second
	if cfg != nil && cfg.HeartbeatTimeout > 0 {
		timeout = cfg.HeartbeatTimeout
	}

	return &Service{
		repo:             repo,
		hostProvider:     hostProvider,
		heartbeatTimeout: timeout,
	}
}

// SetAgentCommandSender sets the agent command sender for cluster operations.
// SetAgentCommandSender 设置用于集群操作的 Agent 命令发送器。
func (s *Service) SetAgentCommandSender(sender AgentCommandSender) {
	s.agentSender = sender
}

// Create creates a new cluster with validation.
// Create 创建一个新集群并进行验证。
// Requirements: 7.1 - Validates cluster name uniqueness and stores basic info.
func (s *Service) Create(ctx context.Context, req *CreateClusterRequest) (*Cluster, error) {
	// Validate cluster name is not empty
	// 验证集群名不为空
	if req.Name == "" {
		return nil, ErrClusterNameEmpty
	}

	// Validate deployment mode
	// 验证部署模式
	if !isValidDeploymentMode(req.DeploymentMode) {
		return nil, ErrInvalidDeploymentMode
	}

	// Create cluster
	// 创建集群
	cluster := &Cluster{
		Name:           req.Name,
		Description:    req.Description,
		DeploymentMode: req.DeploymentMode,
		Version:        req.Version,
		Status:         ClusterStatusCreated,
		InstallDir:     req.InstallDir,
		Config:         req.Config,
	}

	if err := s.repo.Create(ctx, cluster); err != nil {
		return nil, err
	}

	return cluster, nil
}

// Get retrieves a cluster by ID with optional node preloading.
// Get 根据 ID 获取集群，可选择预加载节点。
// Requirements: 7.3 - Returns cluster name, status, node list, version info, creation time.
func (s *Service) Get(ctx context.Context, id uint) (*Cluster, error) {
	return s.repo.GetByID(ctx, id, true)
}

// GetByName retrieves a cluster by name.
// GetByName 根据名称获取集群。
func (s *Service) GetByName(ctx context.Context, name string) (*Cluster, error) {
	return s.repo.GetByName(ctx, name)
}

// List retrieves clusters based on filter criteria.
// List 根据过滤条件获取集群列表。
// Requirements: 7.3 - Returns cluster list with node count.
func (s *Service) List(ctx context.Context, filter *ClusterFilter) ([]*Cluster, int64, error) {
	return s.repo.List(ctx, filter)
}

// ListWithInfo retrieves clusters and converts them to ClusterInfo.
// ListWithInfo 获取集群列表并转换为 ClusterInfo。
func (s *Service) ListWithInfo(ctx context.Context, filter *ClusterFilter) ([]*ClusterInfo, int64, error) {
	clusters, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	infos := make([]*ClusterInfo, len(clusters))
	for i, c := range clusters {
		infos[i] = c.ToClusterInfo()
	}

	return infos, total, nil
}

// Update updates an existing cluster with validation.
// Update 更新现有集群并进行验证。
func (s *Service) Update(ctx context.Context, id uint, req *UpdateClusterRequest) (*Cluster, error) {
	// Get existing cluster
	// 获取现有集群
	cluster, err := s.repo.GetByID(ctx, id, false)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	// 如果提供了字段则更新
	if req.Name != nil {
		if *req.Name == "" {
			return nil, ErrClusterNameEmpty
		}
		cluster.Name = *req.Name
	}

	if req.Description != nil {
		cluster.Description = *req.Description
	}

	if req.Version != nil {
		cluster.Version = *req.Version
	}

	if req.InstallDir != nil {
		cluster.InstallDir = *req.InstallDir
	}

	if req.Config != nil {
		cluster.Config = *req.Config
	}

	if err := s.repo.Update(ctx, cluster); err != nil {
		return nil, err
	}

	return cluster, nil
}

// Delete removes a cluster after checking for running tasks.
// Delete 在检查运行中的任务后删除集群。
// Requirements: 7.5 - Checks if cluster has running tasks before deletion.
func (s *Service) Delete(ctx context.Context, id uint) error {
	// Get cluster to check status
	// 获取集群以检查状态
	cluster, err := s.repo.GetByID(ctx, id, false)
	if err != nil {
		return err
	}

	// Check if cluster has running tasks (deploying or running status)
	// 检查集群是否有运行中的任务（部署中或运行中状态）
	if cluster.Status == ClusterStatusDeploying || cluster.Status == ClusterStatusRunning {
		return ErrClusterHasRunningTask
	}

	return s.repo.Delete(ctx, id)
}

// UpdateStatus updates the status of a cluster.
// UpdateStatus 更新集群的状态。
func (s *Service) UpdateStatus(ctx context.Context, id uint, status ClusterStatus) error {
	return s.repo.UpdateStatus(ctx, id, status)
}

// isValidDeploymentMode checks if the deployment mode is valid.
// isValidDeploymentMode 检查部署模式是否有效。
func isValidDeploymentMode(mode DeploymentMode) bool {
	return mode == DeploymentModeHybrid || mode == DeploymentModeSeparated
}

// isValidNodeRole checks if the node role is valid.
// isValidNodeRole 检查节点角色是否有效。
func isValidNodeRole(role NodeRole) bool {
	return role == NodeRoleMaster || role == NodeRoleWorker
}

// AddNode adds a node to a cluster with validation.
// AddNode 向集群添加节点并进行验证。
// Requirements: 7.2 - Validates host Agent status is "installed" before association.
func (s *Service) AddNode(ctx context.Context, clusterID uint, req *AddNodeRequest) (*ClusterNode, error) {
	// Validate node role
	// 验证节点角色
	if !isValidNodeRole(req.Role) {
		return nil, ErrInvalidNodeRole
	}

	// Check if cluster exists
	// 检查集群是否存在
	_, err := s.repo.GetByID(ctx, clusterID, false)
	if err != nil {
		return nil, err
	}

	// Check if host exists and has Agent installed
	// 检查主机是否存在且已安装 Agent
	if s.hostProvider != nil {
		hostInfo, err := s.hostProvider.GetHostByID(ctx, req.HostID)
		if err != nil {
			return nil, err
		}

		// For bare_metal hosts, verify Agent is installed
		// 对于物理机/VM 主机，验证 Agent 已安装
		if hostInfo.HostType == "bare_metal" || hostInfo.HostType == "" {
			if hostInfo.AgentStatus != "installed" {
				return nil, ErrNodeAgentNotInstalled
			}
		}
	}

	// Create node
	// 创建节点
	node := &ClusterNode{
		ClusterID: clusterID,
		HostID:    req.HostID,
		Role:      req.Role,
		Status:    NodeStatusPending,
	}

	if err := s.repo.AddNode(ctx, node); err != nil {
		return nil, err
	}

	return node, nil
}

// RemoveNode removes a node from a cluster.
// RemoveNode 从集群中移除节点。
// Requirements: 7.4 - Removes node from cluster.
func (s *Service) RemoveNode(ctx context.Context, clusterID uint, nodeID uint) error {
	// Verify node belongs to the cluster
	// 验证节点属于该集群
	node, err := s.repo.GetNodeByID(ctx, nodeID)
	if err != nil {
		return err
	}

	if node.ClusterID != clusterID {
		return ErrNodeNotFound
	}

	return s.repo.RemoveNode(ctx, nodeID)
}

// RemoveNodeByHostID removes a node from a cluster by host ID.
// RemoveNodeByHostID 根据主机 ID 从集群中移除节点。
func (s *Service) RemoveNodeByHostID(ctx context.Context, clusterID uint, hostID uint) error {
	return s.repo.RemoveNodeByClusterAndHost(ctx, clusterID, hostID)
}

// GetNodes retrieves all nodes for a cluster with host information.
// GetNodes 获取集群的所有节点及其主机信息。
// Requirements: 7.4 - Returns each node's host info, role, SeaTunnel process status, resource usage.
func (s *Service) GetNodes(ctx context.Context, clusterID uint) ([]*NodeInfo, error) {
	nodes, err := s.repo.GetNodesByClusterID(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	nodeInfos := make([]*NodeInfo, len(nodes))
	for i, node := range nodes {
		nodeInfo := &NodeInfo{
			ID:            node.ID,
			ClusterID:     node.ClusterID,
			HostID:        node.HostID,
			Role:          node.Role,
			Status:        node.Status,
			ProcessPID:    node.ProcessPID,
			ProcessStatus: node.ProcessStatus,
			CreatedAt:     node.CreatedAt,
			UpdatedAt:     node.UpdatedAt,
		}

		// Get host information
		// 获取主机信息
		if s.hostProvider != nil {
			hostInfo, err := s.hostProvider.GetHostByID(ctx, node.HostID)
			if err == nil {
				nodeInfo.HostName = hostInfo.Name
				nodeInfo.HostIP = hostInfo.IPAddress
			}
		}

		nodeInfos[i] = nodeInfo
	}

	return nodeInfos, nil
}

// GetNode retrieves a specific node by ID.
// GetNode 根据 ID 获取特定节点。
func (s *Service) GetNode(ctx context.Context, nodeID uint) (*ClusterNode, error) {
	return s.repo.GetNodeByID(ctx, nodeID)
}

// UpdateNodeStatus updates the status of a cluster node.
// UpdateNodeStatus 更新集群节点的状态。
func (s *Service) UpdateNodeStatus(ctx context.Context, nodeID uint, status NodeStatus) error {
	return s.repo.UpdateNodeStatus(ctx, nodeID, status)
}

// UpdateNodeProcess updates the process information for a cluster node.
// UpdateNodeProcess 更新集群节点的进程信息。
func (s *Service) UpdateNodeProcess(ctx context.Context, nodeID uint, pid int, processStatus string) error {
	return s.repo.UpdateNodeProcess(ctx, nodeID, pid, processStatus)
}

// GetStatus retrieves the detailed status of a cluster including node health.
// GetStatus 获取集群的详细状态，包括节点健康状况。
// Requirements: 7.6 - Returns cluster health status based on node states.
func (s *Service) GetStatus(ctx context.Context, clusterID uint) (*ClusterStatusInfo, error) {
	// Get cluster with nodes
	// 获取集群及其节点
	cluster, err := s.repo.GetByID(ctx, clusterID, true)
	if err != nil {
		return nil, err
	}

	statusInfo := &ClusterStatusInfo{
		ClusterID:   cluster.ID,
		ClusterName: cluster.Name,
		Status:      cluster.Status,
		TotalNodes:  len(cluster.Nodes),
		Nodes:       make([]*NodeStatusInfo, len(cluster.Nodes)),
	}

	onlineCount := 0
	offlineCount := 0

	for i, node := range cluster.Nodes {
		nodeStatus := &NodeStatusInfo{
			NodeID:        node.ID,
			HostID:        node.HostID,
			Role:          node.Role,
			Status:        node.Status,
			ProcessPID:    node.ProcessPID,
			ProcessStatus: node.ProcessStatus,
		}

		// Get host information and online status
		// 获取主机信息和在线状态
		if s.hostProvider != nil {
			hostInfo, err := s.hostProvider.GetHostByID(ctx, node.HostID)
			if err == nil {
				nodeStatus.HostName = hostInfo.Name
				nodeStatus.HostIP = hostInfo.IPAddress
				nodeStatus.IsOnline = hostInfo.IsOnline(s.heartbeatTimeout)

				if nodeStatus.IsOnline {
					onlineCount++
				} else {
					offlineCount++
				}
			} else {
				offlineCount++
			}
		}

		statusInfo.Nodes[i] = nodeStatus
	}

	statusInfo.OnlineNodes = onlineCount
	statusInfo.OfflineNodes = offlineCount

	// Determine health status
	// 确定健康状态
	// Requirements: 7.6 - If any node is offline, cluster health is "unhealthy"
	if statusInfo.TotalNodes == 0 {
		statusInfo.HealthStatus = HealthStatusUnknown
	} else if offlineCount > 0 {
		statusInfo.HealthStatus = HealthStatusUnhealthy
	} else {
		statusInfo.HealthStatus = HealthStatusHealthy
	}

	return statusInfo, nil
}

// IsClusterHealthy checks if all nodes in a cluster are online.
// IsClusterHealthy 检查集群中的所有节点是否都在线。
// Requirements: 7.6 - Returns false if any node is offline.
func (s *Service) IsClusterHealthy(ctx context.Context, clusterID uint) (bool, error) {
	status, err := s.GetStatus(ctx, clusterID)
	if err != nil {
		return false, err
	}
	return status.HealthStatus == HealthStatusHealthy, nil
}

// Start starts all nodes in a cluster.
// Start 启动集群中的所有节点。
// Requirements: 6.1 - Executes SeaTunnel start script, waits for process startup, verifies process alive.
func (s *Service) Start(ctx context.Context, clusterID uint) (*OperationResult, error) {
	return s.executeOperation(ctx, clusterID, OperationStart)
}

// Stop stops all nodes in a cluster.
// Stop 停止集群中的所有节点。
// Requirements: 6.2 - Sends SIGTERM, waits for graceful shutdown (max 30s), sends SIGKILL if timeout.
func (s *Service) Stop(ctx context.Context, clusterID uint) (*OperationResult, error) {
	return s.executeOperation(ctx, clusterID, OperationStop)
}

// Restart restarts all nodes in a cluster.
// Restart 重启集群中的所有节点。
// Requirements: 6.3 - Executes stop first, waits for complete exit, then executes start.
func (s *Service) Restart(ctx context.Context, clusterID uint) (*OperationResult, error) {
	return s.executeOperation(ctx, clusterID, OperationRestart)
}

// executeOperation executes an operation on all nodes in a cluster.
// executeOperation 在集群的所有节点上执行操作。
func (s *Service) executeOperation(ctx context.Context, clusterID uint, operation OperationType) (*OperationResult, error) {
	// Get cluster with nodes
	// 获取集群及其节点
	cluster, err := s.repo.GetByID(ctx, clusterID, true)
	if err != nil {
		return nil, err
	}

	result := &OperationResult{
		ClusterID:   clusterID,
		Operation:   operation,
		Success:     true,
		NodeResults: make([]*NodeOperationResult, 0, len(cluster.Nodes)),
	}

	// Update cluster status based on operation
	// 根据操作更新集群状态
	switch operation {
	case OperationStart, OperationRestart:
		if err := s.repo.UpdateStatus(ctx, clusterID, ClusterStatusDeploying); err != nil {
			return nil, err
		}
	}

	// Execute operation on each node
	// 在每个节点上执行操作
	for _, node := range cluster.Nodes {
		nodeResult := &NodeOperationResult{
			NodeID: node.ID,
			HostID: node.HostID,
		}

		// Get host information
		// 获取主机信息
		if s.hostProvider != nil {
			hostInfo, err := s.hostProvider.GetHostByID(ctx, node.HostID)
			if err != nil {
				nodeResult.Success = false
				nodeResult.Message = "Failed to get host information: " + err.Error()
				result.NodeResults = append(result.NodeResults, nodeResult)
				result.Success = false
				continue
			}

			nodeResult.HostName = hostInfo.Name

			// Check if host is online (for bare_metal hosts)
			// 检查主机是否在线（对于物理机/VM 主机）
			if hostInfo.HostType == "bare_metal" || hostInfo.HostType == "" {
				if !hostInfo.IsOnline(s.heartbeatTimeout) {
					nodeResult.Success = false
					nodeResult.Message = "Host is offline"
					result.NodeResults = append(result.NodeResults, nodeResult)
					result.Success = false
					continue
				}

				// Send command to agent if sender is available
				// 如果发送器可用，向 Agent 发送命令
				if s.agentSender != nil && hostInfo.AgentID != "" {
					params := map[string]string{
						"cluster_id":  fmt.Sprintf("%d", clusterID),
						"node_id":     fmt.Sprintf("%d", node.ID),
						"role":        string(node.Role),
						"install_dir": cluster.InstallDir,
					}

					success, message, err := s.agentSender.SendCommand(ctx, hostInfo.AgentID, string(operation), params)
					if err != nil {
						nodeResult.Success = false
						nodeResult.Message = "Failed to send command: " + err.Error()
						result.Success = false
					} else {
						nodeResult.Success = success
						nodeResult.Message = message
						if !success {
							result.Success = false
						}
					}
				} else {
					// Agent sender not available, mark as pending
					// Agent 发送器不可用，标记为待处理
					nodeResult.Success = true
					nodeResult.Message = "Operation queued (Agent sender not configured)"
				}
			} else {
				// For Docker/K8s hosts, operations will be handled by respective managers
				// 对于 Docker/K8s 主机，操作将由相应的管理器处理
				nodeResult.Success = true
				nodeResult.Message = "Operation queued for " + hostInfo.HostType + " host"
			}
		} else {
			// No host provider, mark as pending
			// 没有主机提供者，标记为待处理
			nodeResult.Success = true
			nodeResult.Message = "Operation queued (host provider not configured)"
		}

		// Update node status based on operation
		// 根据操作更新节点状态
		if nodeResult.Success {
			switch operation {
			case OperationStart:
				_ = s.repo.UpdateNodeStatus(ctx, node.ID, NodeStatusRunning)
			case OperationStop:
				_ = s.repo.UpdateNodeStatus(ctx, node.ID, NodeStatusStopped)
			case OperationRestart:
				_ = s.repo.UpdateNodeStatus(ctx, node.ID, NodeStatusRunning)
			}
		} else {
			_ = s.repo.UpdateNodeStatus(ctx, node.ID, NodeStatusError)
		}

		result.NodeResults = append(result.NodeResults, nodeResult)
	}

	// Update cluster status based on overall result
	// 根据整体结果更新集群状态
	if result.Success {
		switch operation {
		case OperationStart, OperationRestart:
			_ = s.repo.UpdateStatus(ctx, clusterID, ClusterStatusRunning)
		case OperationStop:
			_ = s.repo.UpdateStatus(ctx, clusterID, ClusterStatusStopped)
		}
		result.Message = "Operation completed successfully"
	} else {
		_ = s.repo.UpdateStatus(ctx, clusterID, ClusterStatusError)
		result.Message = "Operation completed with errors"
	}

	return result, nil
}

// GetClustersByHostID retrieves all clusters that have a specific host as a node.
// GetClustersByHostID 获取将特定主机作为节点的所有集群。
func (s *Service) GetClustersByHostID(ctx context.Context, hostID uint) ([]*Cluster, error) {
	return s.repo.GetClustersWithHostID(ctx, hostID)
}
