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

package monitor

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

// AgentConfigSender defines the interface for sending config to agents.
// AgentConfigSender 定义向 Agent 发送配置的接口。
type AgentConfigSender interface {
	// SendMonitorConfig sends monitor config and tracked processes to agent.
	// SendMonitorConfig 向 Agent 发送监控配置和跟踪的进程列表。
	SendMonitorConfig(ctx context.Context, agentID string, config *MonitorConfig, processes []*TrackedProcessInfo) error
}

// TrackedProcessInfo represents a process to be tracked by the agent.
// TrackedProcessInfo 表示需要被 Agent 跟踪的进程信息。
type TrackedProcessInfo struct {
	PID        int    `json:"pid"`         // 进程 PID / Process PID
	Name       string `json:"name"`        // 进程名称 / Process name (e.g., "seatunnel-master")
	InstallDir string `json:"install_dir"` // 安装目录 / Install directory
	Role       string `json:"role"`        // 节点角色 / Node role
}

// ClusterNodeProvider defines the interface for getting cluster node information.
// ClusterNodeProvider 定义获取集群节点信息的接口。
type ClusterNodeProvider interface {
	// GetNodesByClusterID returns all nodes for a cluster.
	// GetNodesByClusterID 返回集群的所有节点。
	GetNodesByClusterID(ctx context.Context, clusterID uint) ([]*NodeInfoForMonitor, error)
}

// NodeInfoForMonitor represents node info needed for monitoring.
// NodeInfoForMonitor 表示监控所需的节点信息。
type NodeInfoForMonitor struct {
	HostID     uint   `json:"host_id"`
	AgentID    string `json:"agent_id"`
	InstallDir string `json:"install_dir"`
	Role       string `json:"role"`
	ProcessPID int    `json:"process_pid"`
}

// Service provides monitor configuration and event management.
// Service 提供监控配置和事件管理。
type Service struct {
	repo             *Repository
	configSender     AgentConfigSender
	nodeProvider     ClusterNodeProvider
}

// NewService creates a new monitor service.
// NewService 创建新的监控服务。
func NewService(repo *Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// SetConfigSender sets the agent config sender.
// SetConfigSender 设置 Agent 配置发送器。
func (s *Service) SetConfigSender(sender AgentConfigSender) {
	s.configSender = sender
}

// SetNodeProvider sets the cluster node provider.
// SetNodeProvider 设置集群节点提供者。
func (s *Service) SetNodeProvider(provider ClusterNodeProvider) {
	s.nodeProvider = provider
}

// ==================== MonitorConfig Operations 监控配置操作 ====================

// GetConfig retrieves monitor config for a cluster.
// GetConfig 获取集群的监控配置。
// Requirements: 5.2 - Get monitor config
func (s *Service) GetConfig(ctx context.Context, clusterID uint) (*MonitorConfig, error) {
	return s.repo.GetConfigByClusterID(ctx, clusterID)
}

// GetOrCreateConfig retrieves or creates default monitor config for a cluster.
// GetOrCreateConfig 获取或创建集群的默认监控配置。
// Requirements: 5.2, 5.7 - Get or create default config
// **Feature: seatunnel-process-monitor, Property 14: 新集群默认配置**
// **Validates: Requirements 5.2**
func (s *Service) GetOrCreateConfig(ctx context.Context, clusterID uint) (*MonitorConfig, error) {
	config, err := s.repo.GetConfigByClusterID(ctx, clusterID)
	if err == ErrConfigNotFound {
		// Create default config / 创建默认配置
		config = DefaultMonitorConfig(clusterID)
		if err := s.repo.CreateConfig(ctx, config); err != nil {
			return nil, err
		}
		log.Printf("[Monitor] Created default config for cluster %d / 为集群 %d 创建默认配置", clusterID, clusterID)
		return config, nil
	}
	if err != nil {
		return nil, err
	}

	// Fix legacy records with zero values / 修复旧记录的零值
	// 如果配置存在但关键字段为零值，则应用默认值
	needsUpdate := false
	defaults := DefaultMonitorConfig(clusterID)

	if config.MonitorInterval <= 0 {
		config.MonitorInterval = defaults.MonitorInterval
		needsUpdate = true
	}
	if config.RestartDelay <= 0 {
		config.RestartDelay = defaults.RestartDelay
		needsUpdate = true
	}
	if config.MaxRestarts <= 0 {
		config.MaxRestarts = defaults.MaxRestarts
		needsUpdate = true
	}
	if config.TimeWindow <= 0 {
		config.TimeWindow = defaults.TimeWindow
		needsUpdate = true
	}
	if config.CooldownPeriod <= 0 {
		config.CooldownPeriod = defaults.CooldownPeriod
		needsUpdate = true
	}

	// Update database if defaults were applied / 如果应用了默认值则更新数据库
	if needsUpdate {
		if err := s.repo.UpdateConfig(ctx, config); err != nil {
			log.Printf("[Monitor] Failed to update legacy config for cluster %d: %v / 更新集群 %d 旧配置失败: %v",
				clusterID, err, clusterID, err)
		} else {
			log.Printf("[Monitor] Applied default values to legacy config for cluster %d / 为集群 %d 旧配置应用默认值",
				clusterID, clusterID)
		}
	}

	return config, nil
}

// UpdateConfig updates monitor config for a cluster.
// UpdateConfig 更新集群的监控配置。
// Requirements: 5.4 - Update monitor config
// **Feature: seatunnel-process-monitor, Property 13: 配置热更新**
// **Validates: Requirements 5.5**
func (s *Service) UpdateConfig(ctx context.Context, clusterID uint, req *UpdateMonitorConfigRequest) (*MonitorConfig, error) {
	// Validate request / 验证请求
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Get existing config or create default / 获取现有配置或创建默认配置
	config, err := s.GetOrCreateConfig(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	// Apply updates / 应用更新
	if req.AutoMonitor != nil {
		config.AutoMonitor = *req.AutoMonitor
	}
	if req.AutoRestart != nil {
		config.AutoRestart = *req.AutoRestart
	}
	if req.MonitorInterval != nil {
		config.MonitorInterval = *req.MonitorInterval
	}
	if req.RestartDelay != nil {
		config.RestartDelay = *req.RestartDelay
	}
	if req.MaxRestarts != nil {
		config.MaxRestarts = *req.MaxRestarts
	}
	if req.TimeWindow != nil {
		config.TimeWindow = *req.TimeWindow
	}
	if req.CooldownPeriod != nil {
		config.CooldownPeriod = *req.CooldownPeriod
	}

	// Increment version / 递增版本号
	config.ConfigVersion++

	// Save config / 保存配置
	if err := s.repo.UpdateConfig(ctx, config); err != nil {
		return nil, err
	}

	log.Printf("[Monitor] Updated config for cluster %d, version %d / 更新集群 %d 配置，版本 %d",
		clusterID, config.ConfigVersion, clusterID, config.ConfigVersion)

	// Push config to agents / 推送配置到 Agent
	// This is done asynchronously to not block the API response
	// Use a background context since the HTTP request context will be canceled after response
	// 异步执行以不阻塞 API 响应
	// 使用后台 context，因为 HTTP 请求 context 在响应后会被取消
	go s.pushConfigToAgents(context.Background(), clusterID, config)

	return config, nil
}

// pushConfigToAgents pushes monitor config to all agents in the cluster.
// pushConfigToAgents 将监控配置推送到集群中的所有 Agent。
func (s *Service) pushConfigToAgents(ctx context.Context, clusterID uint, config *MonitorConfig) {
	if s.configSender == nil || s.nodeProvider == nil {
		log.Printf("[Monitor] Config sender or node provider not configured, skipping push / 配置发送器或节点提供者未配置，跳过推送")
		return
	}

	// Get all nodes in the cluster / 获取集群中的所有节点
	nodes, err := s.nodeProvider.GetNodesByClusterID(ctx, clusterID)
	if err != nil {
		log.Printf("[Monitor] Failed to get nodes for cluster %d: %v / 获取集群 %d 节点失败：%v", clusterID, err, clusterID, err)
		return
	}

	// Group nodes by agent ID / 按 Agent ID 分组节点
	agentProcesses := make(map[string][]*TrackedProcessInfo)
	for _, node := range nodes {
		if node.AgentID == "" {
			continue
		}
		// When auto-restart is enabled, track all processes (including PID=0) so agent can restart them
		// When auto-restart is disabled, only track running processes (PID > 0)
		// 当启用自动重启时，跟踪所有进程（包括 PID=0），以便 Agent 可以重启它们
		// 当禁用自动重启时，只跟踪运行中的进程（PID > 0）
		if config.AutoRestart || node.ProcessPID > 0 {
			processName := "seatunnel"
			if node.Role != "" && node.Role != "hybrid" && node.Role != "master/worker" {
				processName = "seatunnel-" + node.Role
			}
			agentProcesses[node.AgentID] = append(agentProcesses[node.AgentID], &TrackedProcessInfo{
				PID:        node.ProcessPID,
				Name:       processName,
				InstallDir: node.InstallDir,
				Role:       node.Role,
			})
		}
	}

	// Send config to each agent / 向每个 Agent 发送配置
	for agentID, processes := range agentProcesses {
		if err := s.configSender.SendMonitorConfig(ctx, agentID, config, processes); err != nil {
			log.Printf("[Monitor] Failed to send config to agent %s: %v / 向 Agent %s 发送配置失败：%v", agentID, err, agentID, err)
		} else {
			log.Printf("[Monitor] Sent config to agent %s with %d processes / 向 Agent %s 发送配置，包含 %d 个进程",
				agentID, len(processes), agentID, len(processes))
		}
	}

	// Mark config as synced / 标记配置已同步
	s.MarkConfigSynced(ctx, clusterID)
}

// DeleteConfig deletes monitor config for a cluster.
// DeleteConfig 删除集群的监控配置。
func (s *Service) DeleteConfig(ctx context.Context, clusterID uint) error {
	return s.repo.DeleteConfigByClusterID(ctx, clusterID)
}

// MarkConfigSynced marks the config as synced to agents.
// MarkConfigSynced 标记配置已同步到 Agent。
func (s *Service) MarkConfigSynced(ctx context.Context, clusterID uint) error {
	config, err := s.repo.GetConfigByClusterID(ctx, clusterID)
	if err != nil {
		return err
	}
	now := time.Now()
	config.LastSyncAt = &now
	return s.repo.UpdateConfig(ctx, config)
}

// ==================== ProcessEvent Operations 进程事件操作 ====================

// RecordEvent records a new process event.
// RecordEvent 记录新的进程事件。
// Requirements: 6.1 - Record process events
func (s *Service) RecordEvent(ctx context.Context, event *ProcessEvent) error {
	if err := s.repo.CreateEvent(ctx, event); err != nil {
		return err
	}
	log.Printf("[Monitor] Recorded event: type=%s, cluster=%d, node=%d, pid=%d / 记录事件：类型=%s，集群=%d，节点=%d，PID=%d",
		event.EventType, event.ClusterID, event.NodeID, event.PID, event.EventType, event.ClusterID, event.NodeID, event.PID)
	return nil
}

// RecordEventFromReport records an event from agent report.
// RecordEventFromReport 从 Agent 上报记录事件。
// Requirements: 3.4, 3.5 - Process event from agent
func (s *Service) RecordEventFromReport(ctx context.Context, clusterID, nodeID, hostID uint, eventType ProcessEventType, pid int, processName, installDir, role string, details map[string]string) error {
	detailsJSON, _ := json.Marshal(details)
	event := &ProcessEvent{
		ClusterID:   clusterID,
		NodeID:      nodeID,
		HostID:      hostID,
		EventType:   eventType,
		PID:         pid,
		ProcessName: processName,
		InstallDir:  installDir,
		Role:        role,
		Details:     string(detailsJSON),
	}
	return s.RecordEvent(ctx, event)
}

// GetEvent retrieves a process event by ID.
// GetEvent 根据 ID 获取进程事件。
func (s *Service) GetEvent(ctx context.Context, id uint) (*ProcessEvent, error) {
	return s.repo.GetEventByID(ctx, id)
}

// ListEvents retrieves process events with filtering.
// ListEvents 获取带过滤的进程事件列表。
// Requirements: 6.4 - List process events
func (s *Service) ListEvents(ctx context.Context, filter *ProcessEventFilter) ([]*ProcessEventWithHost, int64, error) {
	// Set default pagination / 设置默认分页
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	return s.repo.ListEventsWithHost(ctx, filter)
}

// ListClusterEvents retrieves recent events for a cluster.
// ListClusterEvents 获取集群的最近事件。
func (s *Service) ListClusterEvents(ctx context.Context, clusterID uint, limit int) ([]*ProcessEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.ListEventsByClusterID(ctx, clusterID, limit)
}

// DeleteClusterEvents deletes all events for a cluster.
// DeleteClusterEvents 删除集群的所有事件。
func (s *Service) DeleteClusterEvents(ctx context.Context, clusterID uint) error {
	return s.repo.DeleteEventsByClusterID(ctx, clusterID)
}

// GetLatestNodeEvent retrieves the latest event for a node.
// GetLatestNodeEvent 获取节点的最新事件。
func (s *Service) GetLatestNodeEvent(ctx context.Context, nodeID uint) (*ProcessEvent, error) {
	return s.repo.GetLatestEventByNodeID(ctx, nodeID)
}

// GetEventStats retrieves event statistics for a cluster.
// GetEventStats 获取集群的事件统计。
func (s *Service) GetEventStats(ctx context.Context, clusterID uint, since *time.Time) (map[ProcessEventType]int64, error) {
	stats := make(map[ProcessEventType]int64)
	eventTypes := []ProcessEventType{
		EventTypeStarted,
		EventTypeStopped,
		EventTypeCrashed,
		EventTypeRestarted,
		EventTypeRestartFailed,
		EventTypeRestartLimitReached,
	}
	for _, eventType := range eventTypes {
		count, err := s.repo.CountEventsByType(ctx, clusterID, eventType, since)
		if err != nil {
			return nil, err
		}
		stats[eventType] = count
	}
	return stats, nil
}
