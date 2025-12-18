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

package grpc

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	"github.com/seatunnel/seatunnelX/internal/apps/agent"
	"github.com/seatunnel/seatunnelX/internal/apps/audit"
	"github.com/seatunnel/seatunnelX/internal/apps/host"
	pb "github.com/seatunnel/seatunnelX/internal/proto/agent"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Register handles Agent registration requests.
// Register 处理 Agent 注册请求。
// Requirements: 1.1, 3.2 - Handles Agent registration, matches host IP, updates Agent status.
func (s *Server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	s.logger.Info("Agent registration request received",
		zap.String("agent_id", req.AgentId),
		zap.String("hostname", req.Hostname),
		zap.String("ip_address", req.IpAddress),
		zap.String("version", req.AgentVersion),
	)

	// Validate request
	// 验证请求
	if req.IpAddress == "" {
		return &pb.RegisterResponse{
			Success: false,
			Message: "ip_address is required",
		}, nil
	}

	// Generate agent_id if not provided (first-time registration)
	// 如果未提供 agent_id，则生成一个（首次注册）
	if req.AgentId == "" {
		req.AgentId = generateAgentID(req.Hostname, req.IpAddress)
		s.logger.Info("Generated agent_id for new Agent",
			zap.String("agent_id", req.AgentId),
			zap.String("hostname", req.Hostname),
			zap.String("ip_address", req.IpAddress),
		)
	}

	// Register Agent with manager
	// 向管理器注册 Agent
	conn, err := s.agentManager.RegisterAgent(ctx, req)
	if err != nil {
		s.logger.Error("Failed to register Agent",
			zap.String("agent_id", req.AgentId),
			zap.Error(err),
		)
		return &pb.RegisterResponse{
			Success: false,
			Message: "failed to register agent: " + err.Error(),
		}, nil
	}

	// Try to match with existing host by IP address
	// 尝试通过 IP 地址匹配现有主机
	if s.hostService != nil {
		var sysInfo *host.SystemInfo
		if req.SystemInfo != nil {
			sysInfo = &host.SystemInfo{
				OSType:      req.OsType,
				Arch:        req.Arch,
				CPUCores:    int(req.SystemInfo.CpuCores),
				TotalMemory: req.SystemInfo.TotalMemory,
				TotalDisk:   req.SystemInfo.TotalDisk,
			}
		}

		updatedHost, err := s.hostService.UpdateAgentStatus(ctx, req.IpAddress, req.AgentId, req.AgentVersion, sysInfo)
		if err != nil {
			// Log warning but don't fail registration
			// 记录警告但不使注册失败
			s.logger.Warn("Failed to update host agent status",
				zap.String("ip_address", req.IpAddress),
				zap.Error(err),
			)
		} else if updatedHost != nil {
			conn.HostID = updatedHost.ID
			s.logger.Info("Agent matched with host",
				zap.String("agent_id", req.AgentId),
				zap.Uint("host_id", updatedHost.ID),
				zap.String("host_name", updatedHost.Name),
			)
		}
	}

	// Build response with configuration
	// 构建带配置的响应
	response := &pb.RegisterResponse{
		Success:    true,
		Message:    "registration successful",
		AssignedId: req.AgentId,
		Config: &pb.AgentConfig{
			HeartbeatInterval: int32(s.config.HeartbeatInterval),
			LogLevel:          int32(pb.LogLevel_INFO),
			Extra:             make(map[string]string),
		},
	}

	s.logger.Info("Agent registered successfully",
		zap.String("agent_id", req.AgentId),
		zap.Uint("host_id", conn.HostID),
	)

	return response, nil
}

// Heartbeat handles Agent heartbeat requests.
// Heartbeat 处理 Agent 心跳请求。
// Requirements: 1.3, 3.3 - Processes heartbeat, updates host resource usage.
func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	// Validate request
	// 验证请求
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	// Handle heartbeat through manager
	// 通过管理器处理心跳
	if err := s.agentManager.HandleHeartbeat(ctx, req); err != nil {
		if err == agent.ErrAgentNotFound {
			return nil, status.Error(codes.NotFound, "agent not found, please re-register")
		}
		s.logger.Error("Failed to handle heartbeat",
			zap.String("agent_id", req.AgentId),
			zap.Error(err),
		)
		return nil, status.Error(codes.Internal, "failed to process heartbeat")
	}

	// Update host heartbeat data if host service is available
	// 如果主机服务可用，更新主机心跳数据
	if s.hostService != nil && req.ResourceUsage != nil {
		if err := s.hostService.UpdateHeartbeat(
			ctx,
			req.AgentId,
			req.ResourceUsage.CpuUsage,
			req.ResourceUsage.MemoryUsage,
			req.ResourceUsage.DiskUsage,
		); err != nil {
			// Log warning but don't fail heartbeat
			// 记录警告但不使心跳失败
			s.logger.Warn("Failed to update host heartbeat",
				zap.String("agent_id", req.AgentId),
				zap.Error(err),
			)
		}
	}

	return &pb.HeartbeatResponse{
		Success:    true,
		ServerTime: time.Now().UnixMilli(),
	}, nil
}

// CommandStream handles bidirectional streaming for command dispatch and result reporting.
// CommandStream 处理用于命令分发和结果上报的双向流。
// Requirements: 1.5, 8.6 - Implements bidirectional stream for command dispatching.
func (s *Server) CommandStream(stream grpc.BidiStreamingServer[pb.CommandResponse, pb.CommandRequest]) error {
	// Get peer info for logging
	// 获取对端信息用于日志记录
	peerAddr := "unknown"
	if p, ok := peer.FromContext(stream.Context()); ok {
		peerAddr = p.Addr.String()
	}

	s.logger.Info("CommandStream started", zap.String("peer", peerAddr))

	// First message should identify the Agent
	// 第一条消息应该标识 Agent
	firstMsg, err := stream.Recv()
	if err != nil {
		s.logger.Error("Failed to receive first message in CommandStream",
			zap.String("peer", peerAddr),
			zap.Error(err),
		)
		return status.Error(codes.InvalidArgument, "failed to receive agent identification")
	}

	// Extract agent_id from the first response (Agent sends its ID)
	// 从第一个响应中提取 agent_id（Agent 发送其 ID）
	agentID := extractAgentIDFromResponse(firstMsg)
	if agentID == "" {
		return status.Error(codes.InvalidArgument, "agent_id not provided in first message")
	}

	// Verify Agent is registered
	// 验证 Agent 已注册
	conn, ok := s.agentManager.GetAgent(agentID)
	if !ok {
		return status.Error(codes.NotFound, "agent not registered, please register first")
	}

	// Set the stream for this Agent
	// 为此 Agent 设置流
	if err := s.agentManager.SetAgentStream(agentID, stream); err != nil {
		s.logger.Error("Failed to set agent stream",
			zap.String("agent_id", agentID),
			zap.Error(err),
		)
		return status.Error(codes.Internal, "failed to set agent stream")
	}

	s.logger.Info("CommandStream established for Agent",
		zap.String("agent_id", agentID),
		zap.Uint("host_id", conn.HostID),
	)

	// Process the first message if it contains command response data
	// 如果第一条消息包含命令响应数据，则处理它
	if firstMsg.CommandId != "" {
		s.handleCommandResponse(agentID, firstMsg)
	}

	// Main loop: receive command responses from Agent
	// 主循环：从 Agent 接收命令响应
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				s.logger.Info("CommandStream closed by Agent",
					zap.String("agent_id", agentID),
				)
			} else {
				s.logger.Error("CommandStream receive error",
					zap.String("agent_id", agentID),
					zap.Error(err),
				)
			}

			// Handle Agent disconnect
			// 处理 Agent 断开连接
			s.agentManager.HandleDisconnect(agentID)
			return err
		}

		// Process command response
		// 处理命令响应
		s.handleCommandResponse(agentID, resp)
	}
}

// handleCommandResponse processes a command response from an Agent.
// handleCommandResponse 处理来自 Agent 的命令响应。
func (s *Server) handleCommandResponse(agentID string, resp *pb.CommandResponse) {
	if resp.CommandId == "" {
		return
	}

	s.logger.Debug("Received command response",
		zap.String("agent_id", agentID),
		zap.String("command_id", resp.CommandId),
		zap.String("status", resp.Status.String()),
		zap.Int32("progress", resp.Progress),
	)

	// Forward to agent manager
	// 转发给 Agent 管理器
	s.agentManager.HandleCommandResponse(resp)

	// Update command log in audit repository
	// 在审计仓库中更新命令日志
	if s.auditRepo != nil {
		s.updateCommandLog(resp)
	}
}

// updateCommandLog updates the command log with the response data.
// updateCommandLog 使用响应数据更新命令日志。
func (s *Server) updateCommandLog(resp *pb.CommandResponse) {
	ctx := context.Background()

	// Get existing command log
	// 获取现有命令日志
	cmdLog, err := s.auditRepo.GetCommandLogByCommandID(ctx, resp.CommandId)
	if err != nil {
		// Command log might not exist yet, which is fine
		// 命令日志可能还不存在，这是正常的
		return
	}

	// Map protobuf status to audit status
	// 将 protobuf 状态映射到审计状态
	var auditStatus audit.CommandStatus
	switch resp.Status {
	case pb.CommandStatus_PENDING:
		auditStatus = audit.CommandStatusPending
	case pb.CommandStatus_RUNNING:
		auditStatus = audit.CommandStatusRunning
	case pb.CommandStatus_SUCCESS:
		auditStatus = audit.CommandStatusSuccess
	case pb.CommandStatus_FAILED:
		auditStatus = audit.CommandStatusFailed
	case pb.CommandStatus_CANCELLED:
		auditStatus = audit.CommandStatusCancelled
	default:
		auditStatus = audit.CommandStatusPending
	}

	// Update command log
	// 更新命令日志
	updates := map[string]interface{}{
		"status":   auditStatus,
		"progress": int(resp.Progress),
	}

	if resp.Output != "" {
		updates["output"] = cmdLog.Output + resp.Output
	}

	if resp.Error != "" {
		updates["error"] = resp.Error
	}

	// Set started_at if transitioning to running
	// 如果转换为运行状态，设置 started_at
	if auditStatus == audit.CommandStatusRunning && cmdLog.StartedAt == nil {
		now := time.Now()
		updates["started_at"] = now
	}

	// Set finished_at if terminal status
	// 如果是终止状态，设置 finished_at
	if auditStatus == audit.CommandStatusSuccess ||
		auditStatus == audit.CommandStatusFailed ||
		auditStatus == audit.CommandStatusCancelled {
		now := time.Now()
		updates["finished_at"] = now
	}

	if err := s.auditRepo.UpdateCommandLogStatus(ctx, cmdLog.ID, updates); err != nil {
		s.logger.Warn("Failed to update command log",
			zap.String("command_id", resp.CommandId),
			zap.Error(err),
		)
	}
}

// LogStream handles log streaming from Agents.
// LogStream 处理来自 Agent 的日志流。
// Requirements: 10.2, 10.3 - Receives Agent logs and stores to audit log.
func (s *Server) LogStream(stream grpc.ClientStreamingServer[pb.LogEntry, pb.LogStreamResponse]) error {
	// Get peer info for logging
	// 获取对端信息用于日志记录
	peerAddr := "unknown"
	if p, ok := peer.FromContext(stream.Context()); ok {
		peerAddr = p.Addr.String()
	}

	s.logger.Debug("LogStream started", zap.String("peer", peerAddr))

	var receivedCount int64
	var agentID string

	// Receive log entries
	// 接收日志条目
	for {
		entry, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				// Client finished sending
				// 客户端完成发送
				s.logger.Debug("LogStream completed",
					zap.String("agent_id", agentID),
					zap.Int64("received_count", receivedCount),
				)
				return stream.SendAndClose(&pb.LogStreamResponse{
					Success:       true,
					ReceivedCount: receivedCount,
				})
			}

			s.logger.Error("LogStream receive error",
				zap.String("agent_id", agentID),
				zap.Error(err),
			)
			return err
		}

		// Track agent ID from first entry
		// 从第一个条目跟踪 Agent ID
		if agentID == "" {
			agentID = entry.AgentId
		}

		// Store log entry to audit log
		// 将日志条目存储到审计日志
		if s.auditRepo != nil {
			s.storeLogEntry(entry)
		}

		receivedCount++
	}
}

// storeLogEntry stores a log entry to the audit log.
// storeLogEntry 将日志条目存储到审计日志。
func (s *Server) storeLogEntry(entry *pb.LogEntry) {
	ctx := context.Background()

	// Map log level to action
	// 将日志级别映射到操作
	action := "agent_log"
	switch entry.Level {
	case pb.LogLevel_ERROR:
		action = "agent_error"
	case pb.LogLevel_WARN:
		action = "agent_warning"
	}

	// Build details from log entry
	// 从日志条目构建详情
	details := audit.AuditDetails{
		"message":    entry.Message,
		"level":      entry.Level.String(),
		"timestamp":  entry.Timestamp,
		"command_id": entry.CommandId,
	}

	// Add extra fields
	// 添加额外字段
	for k, v := range entry.Fields {
		details[k] = v
	}

	// Create audit log entry
	// 创建审计日志条目
	auditLog := &audit.AuditLog{
		Action:       action,
		ResourceType: "agent",
		ResourceID:   entry.AgentId,
		Details:      details,
	}

	if err := s.auditRepo.CreateAuditLog(ctx, auditLog); err != nil {
		s.logger.Warn("Failed to store log entry",
			zap.String("agent_id", entry.AgentId),
			zap.Error(err),
		)
	}
}

// extractAgentIDFromResponse extracts the agent ID from a command response.
// extractAgentIDFromResponse 从命令响应中提取 Agent ID。
// The Agent sends its ID in the output field of the first message.
// Agent 在第一条消息的 output 字段中发送其 ID。
func extractAgentIDFromResponse(resp *pb.CommandResponse) string {
	// Convention: Agent sends its ID in the output field of the first message
	// 约定：Agent 在第一条消息的 output 字段中发送其 ID
	// The command_id will be empty or a special value like "AGENT_INIT"
	// command_id 将为空或特殊值如 "AGENT_INIT"
	if resp.CommandId == "" || resp.CommandId == "AGENT_INIT" {
		return resp.Output
	}
	return ""
}

// generateAgentID generates a unique agent ID based on hostname and IP address.
// generateAgentID 根据主机名和 IP 地址生成唯一的 Agent ID。
func generateAgentID(hostname, ipAddress string) string {
	// Create a deterministic ID based on hostname and IP
	// 根据主机名和 IP 创建确定性 ID
	data := fmt.Sprintf("%s-%s-%d", hostname, ipAddress, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	// Use first 16 characters of hex hash as agent ID
	// 使用十六进制哈希的前 16 个字符作为 Agent ID
	return fmt.Sprintf("agent-%x", hash[:8])
}
