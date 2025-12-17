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
package cluster

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// DeploymentMode represents the deployment mode of a SeaTunnel cluster.
type DeploymentMode string

const (
	// DeploymentModeHybrid indicates a hybrid deployment where master and worker run on the same nodes.
	DeploymentModeHybrid DeploymentMode = "hybrid"
	// DeploymentModeSeparated indicates a separated deployment where master and worker run on different nodes.
	DeploymentModeSeparated DeploymentMode = "separated"
)

// ClusterStatus represents the current status of a cluster.
type ClusterStatus string

const (
	// ClusterStatusCreated indicates the cluster has been created but not deployed.
	ClusterStatusCreated ClusterStatus = "created"
	// ClusterStatusDeploying indicates the cluster is being deployed.
	ClusterStatusDeploying ClusterStatus = "deploying"
	// ClusterStatusRunning indicates the cluster is running normally.
	ClusterStatusRunning ClusterStatus = "running"
	// ClusterStatusStopped indicates the cluster has been stopped.
	ClusterStatusStopped ClusterStatus = "stopped"
	// ClusterStatusError indicates the cluster is in an error state.
	ClusterStatusError ClusterStatus = "error"
)

// NodeRole represents the role of a node in a cluster.
type NodeRole string

const (
	// NodeRoleMaster indicates the node is a master node.
	NodeRoleMaster NodeRole = "master"
	// NodeRoleWorker indicates the node is a worker node.
	NodeRoleWorker NodeRole = "worker"
)

// NodeStatus represents the current status of a cluster node.
type NodeStatus string

const (
	// NodeStatusPending indicates the node is pending deployment.
	NodeStatusPending NodeStatus = "pending"
	// NodeStatusInstalling indicates the node is being installed.
	NodeStatusInstalling NodeStatus = "installing"
	// NodeStatusRunning indicates the node is running normally.
	NodeStatusRunning NodeStatus = "running"
	// NodeStatusStopped indicates the node has been stopped.
	NodeStatusStopped NodeStatus = "stopped"
	// NodeStatusError indicates the node is in an error state.
	NodeStatusError NodeStatus = "error"
)

// ClusterConfig represents the JSON configuration for a cluster.
type ClusterConfig map[string]interface{}

// Value implements the driver.Valuer interface for database storage.
func (c ClusterConfig) Value() (driver.Value, error) {
	if c == nil {
		return nil, nil
	}
	return json.Marshal(c)
}

// Scan implements the sql.Scanner interface for database retrieval.
func (c *ClusterConfig) Scan(value interface{}) error {
	if value == nil {
		*c = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("cluster: failed to scan ClusterConfig - expected []byte")
	}
	return json.Unmarshal(bytes, c)
}

// Cluster represents a SeaTunnel cluster consisting of multiple nodes.
type Cluster struct {
	ID             uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	Name           string         `json:"name" gorm:"size:100;uniqueIndex;not null"`
	Description    string         `json:"description" gorm:"type:text"`
	DeploymentMode DeploymentMode `json:"deployment_mode" gorm:"size:20;not null"`
	Version        string         `json:"version" gorm:"size:20"`
	Status         ClusterStatus  `json:"status" gorm:"size:20;default:created;index"`
	InstallDir     string         `json:"install_dir" gorm:"size:255"`
	Config         ClusterConfig  `json:"config" gorm:"type:json"`
	CreatedAt      time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedBy      uint           `json:"created_by"`
	Nodes          []ClusterNode  `json:"nodes" gorm:"foreignKey:ClusterID"`
}

// TableName specifies the table name for the Cluster model.
func (Cluster) TableName() string {
	return "clusters"
}

// ClusterNode represents a node within a SeaTunnel cluster.
// 集群节点，每个节点可以有独立的安装目录
type ClusterNode struct {
	ID            uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	ClusterID     uint       `json:"cluster_id" gorm:"index;not null"`
	HostID        uint       `json:"host_id" gorm:"index;not null"`
	Role          NodeRole   `json:"role" gorm:"size:20;not null"`
	InstallDir    string     `json:"install_dir" gorm:"size:255"` // SeaTunnel installation directory on this node / 此节点上的 SeaTunnel 安装目录
	Status        NodeStatus `json:"status" gorm:"size:20;default:pending"`
	ProcessPID    int        `json:"process_pid"`
	ProcessStatus string     `json:"process_status" gorm:"size:20"`
	CreatedAt     time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName specifies the table name for the ClusterNode model.
func (ClusterNode) TableName() string {
	return "cluster_nodes"
}

// ClusterFilter represents filter criteria for querying clusters.
type ClusterFilter struct {
	Name           string         `json:"name"`
	Status         ClusterStatus  `json:"status"`
	DeploymentMode DeploymentMode `json:"deployment_mode"`
	Page           int            `json:"page"`
	PageSize       int            `json:"page_size"`
}

// ClusterInfo represents cluster information for API responses.
type ClusterInfo struct {
	ID             uint           `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	DeploymentMode DeploymentMode `json:"deployment_mode"`
	Version        string         `json:"version"`
	Status         ClusterStatus  `json:"status"`
	InstallDir     string         `json:"install_dir"`
	Config         ClusterConfig  `json:"config"`
	NodeCount      int            `json:"node_count"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// ToClusterInfo converts a Cluster to ClusterInfo.
func (c *Cluster) ToClusterInfo() *ClusterInfo {
	return &ClusterInfo{
		ID:             c.ID,
		Name:           c.Name,
		Description:    c.Description,
		DeploymentMode: c.DeploymentMode,
		Version:        c.Version,
		Status:         c.Status,
		InstallDir:     c.InstallDir,
		Config:         c.Config,
		NodeCount:      len(c.Nodes),
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
}

// CreateClusterRequest represents a request to create a new cluster.
type CreateClusterRequest struct {
	Name           string         `json:"name" binding:"required,max=100"`
	Description    string         `json:"description"`
	DeploymentMode DeploymentMode `json:"deployment_mode" binding:"required"`
	Version        string         `json:"version"`
	InstallDir     string         `json:"install_dir"`
	Config         ClusterConfig  `json:"config"`
}

// UpdateClusterRequest represents a request to update an existing cluster.
type UpdateClusterRequest struct {
	Name        *string        `json:"name"`
	Description *string        `json:"description"`
	Version     *string        `json:"version"`
	InstallDir  *string        `json:"install_dir"`
	Config      *ClusterConfig `json:"config"`
}

// AddNodeRequest represents a request to add a node to a cluster.
// 添加节点请求，包含安装目录配置
type AddNodeRequest struct {
	HostID     uint     `json:"host_id" binding:"required"`
	Role       NodeRole `json:"role" binding:"required"`
	InstallDir string   `json:"install_dir"` // SeaTunnel installation directory / SeaTunnel 安装目录
}

// NodeInfo represents node information for API responses.
// 节点信息，用于 API 响应
type NodeInfo struct {
	ID            uint       `json:"id"`
	ClusterID     uint       `json:"cluster_id"`
	HostID        uint       `json:"host_id"`
	HostName      string     `json:"host_name"`
	HostIP        string     `json:"host_ip"`
	Role          NodeRole   `json:"role"`
	InstallDir    string     `json:"install_dir"` // SeaTunnel installation directory / SeaTunnel 安装目录
	Status        NodeStatus `json:"status"`
	ProcessPID    int        `json:"process_pid"`
	ProcessStatus string     `json:"process_status"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
