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

// Package host provides host management functionality for the SeaTunnel Agent system.
package host

import (
	"net"
	"time"
)

// AgentStatus represents the installation status of an Agent on a host.
type AgentStatus string

const (
	// AgentStatusNotInstalled indicates the Agent has not been installed on the host.
	AgentStatusNotInstalled AgentStatus = "not_installed"
	// AgentStatusInstalled indicates the Agent is installed and connected.
	AgentStatusInstalled AgentStatus = "installed"
	// AgentStatusOffline indicates the Agent was installed but is currently offline.
	AgentStatusOffline AgentStatus = "offline"
)

// Host represents a physical or virtual machine node that runs the Agent and SeaTunnel services.
type Host struct {
	ID            uint        `json:"id" gorm:"primaryKey;autoIncrement"`
	Name          string      `json:"name" gorm:"size:100;uniqueIndex;not null"`
	IPAddress     string      `json:"ip_address" gorm:"size:45;not null"`
	SSHPort       int         `json:"ssh_port" gorm:"default:22"`
	Description   string      `json:"description" gorm:"type:text"`
	AgentID       string      `json:"agent_id" gorm:"size:100;index"`
	AgentStatus   AgentStatus `json:"agent_status" gorm:"size:20;default:not_installed;index"`
	AgentVersion  string      `json:"agent_version" gorm:"size:20"`
	OSType        string      `json:"os_type" gorm:"size:20"`
	Arch          string      `json:"arch" gorm:"size:20"`
	CPUCores      int         `json:"cpu_cores"`
	TotalMemory   int64       `json:"total_memory"`
	TotalDisk     int64       `json:"total_disk"`
	CPUUsage      float64     `json:"cpu_usage" gorm:"type:decimal(5,2)"`
	MemoryUsage   float64     `json:"memory_usage" gorm:"type:decimal(5,2)"`
	DiskUsage     float64     `json:"disk_usage" gorm:"type:decimal(5,2)"`
	LastHeartbeat *time.Time  `json:"last_heartbeat"`
	CreatedAt     time.Time   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time   `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedBy     uint        `json:"created_by"`
}

// TableName specifies the table name for the Host model.
func (Host) TableName() string {
	return "hosts"
}

// IsOnline returns true if the host has received a heartbeat within the timeout period.
// The default timeout is 30 seconds as per Requirements 3.4.
func (h *Host) IsOnline(timeout time.Duration) bool {
	if h.LastHeartbeat == nil {
		return false
	}
	return time.Since(*h.LastHeartbeat) <= timeout
}

// ValidateIPAddress validates that the IP address is a valid IPv4 or IPv6 address.
// Returns true if the IP address is valid, false otherwise.
func ValidateIPAddress(ip string) bool {
	if ip == "" {
		return false
	}
	return net.ParseIP(ip) != nil
}

// HostFilter represents filter criteria for querying hosts.
type HostFilter struct {
	Name        string      `json:"name"`
	IPAddress   string      `json:"ip_address"`
	AgentStatus AgentStatus `json:"agent_status"`
	IsOnline    *bool       `json:"is_online"`
	Page        int         `json:"page"`
	PageSize    int         `json:"page_size"`
}

// HostInfo represents host information for API responses.
type HostInfo struct {
	ID            uint        `json:"id"`
	Name          string      `json:"name"`
	IPAddress     string      `json:"ip_address"`
	SSHPort       int         `json:"ssh_port"`
	Description   string      `json:"description"`
	AgentID       string      `json:"agent_id"`
	AgentStatus   AgentStatus `json:"agent_status"`
	AgentVersion  string      `json:"agent_version"`
	OSType        string      `json:"os_type"`
	Arch          string      `json:"arch"`
	CPUCores      int         `json:"cpu_cores"`
	TotalMemory   int64       `json:"total_memory"`
	TotalDisk     int64       `json:"total_disk"`
	CPUUsage      float64     `json:"cpu_usage"`
	MemoryUsage   float64     `json:"memory_usage"`
	DiskUsage     float64     `json:"disk_usage"`
	IsOnline      bool        `json:"is_online"`
	LastHeartbeat *time.Time  `json:"last_heartbeat"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// ToHostInfo converts a Host to HostInfo with online status calculated.
func (h *Host) ToHostInfo(heartbeatTimeout time.Duration) *HostInfo {
	return &HostInfo{
		ID:            h.ID,
		Name:          h.Name,
		IPAddress:     h.IPAddress,
		SSHPort:       h.SSHPort,
		Description:   h.Description,
		AgentID:       h.AgentID,
		AgentStatus:   h.AgentStatus,
		AgentVersion:  h.AgentVersion,
		OSType:        h.OSType,
		Arch:          h.Arch,
		CPUCores:      h.CPUCores,
		TotalMemory:   h.TotalMemory,
		TotalDisk:     h.TotalDisk,
		CPUUsage:      h.CPUUsage,
		MemoryUsage:   h.MemoryUsage,
		DiskUsage:     h.DiskUsage,
		IsOnline:      h.IsOnline(heartbeatTimeout),
		LastHeartbeat: h.LastHeartbeat,
		CreatedAt:     h.CreatedAt,
		UpdatedAt:     h.UpdatedAt,
	}
}

// CreateHostRequest represents a request to create a new host.
type CreateHostRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	IPAddress   string `json:"ip_address" binding:"required"`
	SSHPort     int    `json:"ssh_port"`
	Description string `json:"description"`
}

// UpdateHostRequest represents a request to update an existing host.
type UpdateHostRequest struct {
	Name        *string `json:"name"`
	IPAddress   *string `json:"ip_address"`
	SSHPort     *int    `json:"ssh_port"`
	Description *string `json:"description"`
}
