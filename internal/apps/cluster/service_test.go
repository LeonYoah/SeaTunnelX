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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MockHostProvider implements HostProvider for testing
// MockHostProvider 实现用于测试的 HostProvider 接口
type MockHostProvider struct {
	hosts map[uint]*HostInfo
}

// NewMockHostProvider creates a new MockHostProvider
// NewMockHostProvider 创建一个新的 MockHostProvider
func NewMockHostProvider() *MockHostProvider {
	return &MockHostProvider{
		hosts: make(map[uint]*HostInfo),
	}
}

// AddHost adds a host to the mock provider
// AddHost 向模拟提供者添加主机
func (m *MockHostProvider) AddHost(host *HostInfo) {
	m.hosts[host.ID] = host
}

// GetHostByID retrieves host information by ID
// GetHostByID 根据 ID 获取主机信息
func (m *MockHostProvider) GetHostByID(ctx context.Context, id uint) (*HostInfo, error) {
	host, ok := m.hosts[id]
	if !ok {
		return nil, fmt.Errorf("host not found: %d", id)
	}
	return host, nil
}

// setupServiceTestDB creates an in-memory SQLite database for service testing
// setupServiceTestDB 创建用于服务测试的内存 SQLite 数据库
func setupServiceTestDB(t *testing.T) (*gorm.DB, func()) {
	tempDir, err := os.MkdirTemp("", "cluster_service_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to open database: %v", err)
	}

	// Auto-migrate models
	// 自动迁移模型
	if err := db.AutoMigrate(&Cluster{}, &ClusterNode{}); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to migrate: %v", err)
	}

	cleanup := func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
		os.RemoveAll(tempDir)
	}

	return db, cleanup
}

// genValidHostID generates valid host IDs for tests
// genValidHostID 为测试生成有效的主机 ID
func genValidHostID() gopter.Gen {
	return gen.UIntRange(1, 10000).Map(func(v uint) uint {
		return v
	})
}

// genNodeRole generates valid node roles
// genNodeRole 生成有效的节点角色
func genNodeRole() gopter.Gen {
	return gen.OneConstOf(NodeRoleMaster, NodeRoleWorker)
}

// **Feature: seatunnel-agent, Property 11: Node Association Validation**
// **Validates: Requirements 7.2**
// For any request to add a node to a cluster, if the host's Agent status is not "installed",
// the system SHALL reject the association and return an error.

func TestProperty_NodeAssociationValidation(t *testing.T) {
	// **Feature: seatunnel-agent, Property 11: Node Association Validation**
	// **Validates: Requirements 7.2**

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(42)

	properties := gopter.NewProperties(parameters)

	properties.Property("node association is rejected when agent is not installed", prop.ForAll(
		func(clusterName string, hostID uint, role NodeRole) bool {
			db, cleanup := setupServiceTestDB(t)
			defer cleanup()

			repo := NewRepository(db)
			mockHostProvider := NewMockHostProvider()

			// Add a host with agent NOT installed
			// 添加一个未安装 Agent 的主机
			mockHostProvider.AddHost(&HostInfo{
				ID:          hostID,
				Name:        fmt.Sprintf("host-%d", hostID),
				HostType:    "bare_metal",
				IPAddress:   fmt.Sprintf("192.168.1.%d", hostID%255),
				AgentStatus: "not_installed", // Agent not installed
			})

			svc := NewService(repo, mockHostProvider, nil)
			ctx := context.Background()

			// Create a cluster first
			// 首先创建一个集群
			cluster, err := svc.Create(ctx, &CreateClusterRequest{
				Name:           clusterName,
				DeploymentMode: DeploymentModeHybrid,
			})
			if err != nil {
				t.Logf("Failed to create cluster: %v", err)
				return false
			}

			// Try to add node with agent not installed
			// 尝试添加未安装 Agent 的节点
			_, err = svc.AddNode(ctx, cluster.ID, &AddNodeRequest{
				HostID: hostID,
				Role:   role,
			})

			// Should fail with ErrNodeAgentNotInstalled
			// 应该返回 ErrNodeAgentNotInstalled 错误
			if err != ErrNodeAgentNotInstalled {
				t.Logf("Expected ErrNodeAgentNotInstalled, got: %v", err)
				return false
			}

			return true
		},
		genValidClusterName(),
		genValidHostID(),
		genNodeRole(),
	))

	properties.Property("node association succeeds when agent is installed", prop.ForAll(
		func(clusterName string, hostID uint, role NodeRole) bool {
			db, cleanup := setupServiceTestDB(t)
			defer cleanup()

			repo := NewRepository(db)
			mockHostProvider := NewMockHostProvider()

			// Add a host with agent installed
			// 添加一个已安装 Agent 的主机
			now := time.Now()
			mockHostProvider.AddHost(&HostInfo{
				ID:            hostID,
				Name:          fmt.Sprintf("host-%d", hostID),
				HostType:      "bare_metal",
				IPAddress:     fmt.Sprintf("192.168.1.%d", hostID%255),
				AgentStatus:   "installed", // Agent installed
				LastHeartbeat: &now,
			})

			svc := NewService(repo, mockHostProvider, nil)
			ctx := context.Background()

			// Create a cluster first
			// 首先创建一个集群
			cluster, err := svc.Create(ctx, &CreateClusterRequest{
				Name:           clusterName,
				DeploymentMode: DeploymentModeHybrid,
			})
			if err != nil {
				t.Logf("Failed to create cluster: %v", err)
				return false
			}

			// Add node with agent installed
			// 添加已安装 Agent 的节点
			node, err := svc.AddNode(ctx, cluster.ID, &AddNodeRequest{
				HostID: hostID,
				Role:   role,
			})
			if err != nil {
				t.Logf("Failed to add node: %v", err)
				return false
			}

			// Verify node was created
			// 验证节点已创建
			if node.ClusterID != cluster.ID {
				t.Logf("Node cluster ID mismatch: expected %d, got %d", cluster.ID, node.ClusterID)
				return false
			}

			if node.HostID != hostID {
				t.Logf("Node host ID mismatch: expected %d, got %d", hostID, node.HostID)
				return false
			}

			if node.Role != role {
				t.Logf("Node role mismatch: expected %s, got %s", role, node.Role)
				return false
			}

			return true
		},
		genValidClusterName(),
		genValidHostID(),
		genNodeRole(),
	))

	properties.Property("node association succeeds for docker hosts without agent check", prop.ForAll(
		func(clusterName string, hostID uint, role NodeRole) bool {
			db, cleanup := setupServiceTestDB(t)
			defer cleanup()

			repo := NewRepository(db)
			mockHostProvider := NewMockHostProvider()

			// Add a Docker host (no agent required)
			// 添加一个 Docker 主机（不需要 Agent）
			mockHostProvider.AddHost(&HostInfo{
				ID:          hostID,
				Name:        fmt.Sprintf("docker-host-%d", hostID),
				HostType:    "docker",
				AgentStatus: "", // No agent status for Docker hosts
			})

			svc := NewService(repo, mockHostProvider, nil)
			ctx := context.Background()

			// Create a cluster first
			// 首先创建一个集群
			cluster, err := svc.Create(ctx, &CreateClusterRequest{
				Name:           clusterName,
				DeploymentMode: DeploymentModeHybrid,
			})
			if err != nil {
				t.Logf("Failed to create cluster: %v", err)
				return false
			}

			// Add Docker host as node (should succeed without agent check)
			// 添加 Docker 主机作为节点（应该成功，无需 Agent 检查）
			node, err := svc.AddNode(ctx, cluster.ID, &AddNodeRequest{
				HostID: hostID,
				Role:   role,
			})
			if err != nil {
				t.Logf("Failed to add Docker node: %v", err)
				return false
			}

			// Verify node was created
			// 验证节点已创建
			if node == nil {
				t.Logf("Node should not be nil")
				return false
			}

			return true
		},
		genValidClusterName(),
		genValidHostID(),
		genNodeRole(),
	))

	properties.TestingRun(t)
}

// **Feature: seatunnel-agent, Property 12: Cluster Health Status Propagation**
// **Validates: Requirements 7.6**
// For any cluster with at least one node whose host status is "offline",
// the cluster's health status SHALL be marked as "unhealthy".

func TestProperty_ClusterHealthStatusPropagation(t *testing.T) {
	// **Feature: seatunnel-agent, Property 12: Cluster Health Status Propagation**
	// **Validates: Requirements 7.6**

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(42)

	properties := gopter.NewProperties(parameters)

	properties.Property("cluster is unhealthy when any node is offline", prop.ForAll(
		func(clusterName string, onlineHostID uint, offlineHostID uint) bool {
			// Ensure different host IDs
			// 确保不同的主机 ID
			if onlineHostID == offlineHostID {
				offlineHostID = onlineHostID + 1
			}

			db, cleanup := setupServiceTestDB(t)
			defer cleanup()

			repo := NewRepository(db)
			mockHostProvider := NewMockHostProvider()

			// Add an online host (recent heartbeat)
			// 添加一个在线主机（最近有心跳）
			recentTime := time.Now()
			mockHostProvider.AddHost(&HostInfo{
				ID:            onlineHostID,
				Name:          fmt.Sprintf("online-host-%d", onlineHostID),
				HostType:      "bare_metal",
				IPAddress:     fmt.Sprintf("192.168.1.%d", onlineHostID%255),
				AgentStatus:   "installed",
				LastHeartbeat: &recentTime,
			})

			// Add an offline host (old heartbeat)
			// 添加一个离线主机（旧心跳）
			oldTime := time.Now().Add(-60 * time.Second) // 60 seconds ago, exceeds 30s timeout
			mockHostProvider.AddHost(&HostInfo{
				ID:            offlineHostID,
				Name:          fmt.Sprintf("offline-host-%d", offlineHostID),
				HostType:      "bare_metal",
				IPAddress:     fmt.Sprintf("192.168.2.%d", offlineHostID%255),
				AgentStatus:   "installed",
				LastHeartbeat: &oldTime,
			})

			svc := NewService(repo, mockHostProvider, &ServiceConfig{
				HeartbeatTimeout: 30 * time.Second,
			})
			ctx := context.Background()

			// Create a cluster
			// 创建一个集群
			cluster, err := svc.Create(ctx, &CreateClusterRequest{
				Name:           clusterName,
				DeploymentMode: DeploymentModeHybrid,
			})
			if err != nil {
				t.Logf("Failed to create cluster: %v", err)
				return false
			}

			// Add online host as master node
			// 添加在线主机作为 master 节点
			_, err = svc.AddNode(ctx, cluster.ID, &AddNodeRequest{
				HostID: onlineHostID,
				Role:   NodeRoleMaster,
			})
			if err != nil {
				t.Logf("Failed to add online node: %v", err)
				return false
			}

			// Add offline host as worker node
			// 添加离线主机作为 worker 节点
			_, err = svc.AddNode(ctx, cluster.ID, &AddNodeRequest{
				HostID: offlineHostID,
				Role:   NodeRoleWorker,
			})
			if err != nil {
				t.Logf("Failed to add offline node: %v", err)
				return false
			}

			// Get cluster status
			// 获取集群状态
			status, err := svc.GetStatus(ctx, cluster.ID)
			if err != nil {
				t.Logf("Failed to get cluster status: %v", err)
				return false
			}

			// Verify cluster is unhealthy because one node is offline
			// 验证集群不健康，因为有一个节点离线
			if status.HealthStatus != HealthStatusUnhealthy {
				t.Logf("Cluster should be unhealthy when any node is offline, got: %s", status.HealthStatus)
				return false
			}

			// Verify node counts
			// 验证节点计数
			if status.TotalNodes != 2 {
				t.Logf("Expected 2 total nodes, got: %d", status.TotalNodes)
				return false
			}

			if status.OnlineNodes != 1 {
				t.Logf("Expected 1 online node, got: %d", status.OnlineNodes)
				return false
			}

			if status.OfflineNodes != 1 {
				t.Logf("Expected 1 offline node, got: %d", status.OfflineNodes)
				return false
			}

			return true
		},
		genValidClusterName(),
		genValidHostID(),
		genValidHostID(),
	))

	properties.Property("cluster is healthy when all nodes are online", prop.ForAll(
		func(clusterName string, hostID1 uint, hostID2 uint) bool {
			// Ensure different host IDs
			// 确保不同的主机 ID
			if hostID1 == hostID2 {
				hostID2 = hostID1 + 1
			}

			db, cleanup := setupServiceTestDB(t)
			defer cleanup()

			repo := NewRepository(db)
			mockHostProvider := NewMockHostProvider()

			// Add two online hosts (recent heartbeats)
			// 添加两个在线主机（最近有心跳）
			recentTime := time.Now()
			mockHostProvider.AddHost(&HostInfo{
				ID:            hostID1,
				Name:          fmt.Sprintf("host-%d", hostID1),
				HostType:      "bare_metal",
				IPAddress:     fmt.Sprintf("192.168.1.%d", hostID1%255),
				AgentStatus:   "installed",
				LastHeartbeat: &recentTime,
			})

			mockHostProvider.AddHost(&HostInfo{
				ID:            hostID2,
				Name:          fmt.Sprintf("host-%d", hostID2),
				HostType:      "bare_metal",
				IPAddress:     fmt.Sprintf("192.168.2.%d", hostID2%255),
				AgentStatus:   "installed",
				LastHeartbeat: &recentTime,
			})

			svc := NewService(repo, mockHostProvider, &ServiceConfig{
				HeartbeatTimeout: 30 * time.Second,
			})
			ctx := context.Background()

			// Create a cluster
			// 创建一个集群
			cluster, err := svc.Create(ctx, &CreateClusterRequest{
				Name:           clusterName,
				DeploymentMode: DeploymentModeHybrid,
			})
			if err != nil {
				t.Logf("Failed to create cluster: %v", err)
				return false
			}

			// Add both hosts as nodes
			// 添加两个主机作为节点
			_, err = svc.AddNode(ctx, cluster.ID, &AddNodeRequest{
				HostID: hostID1,
				Role:   NodeRoleMaster,
			})
			if err != nil {
				t.Logf("Failed to add node 1: %v", err)
				return false
			}

			_, err = svc.AddNode(ctx, cluster.ID, &AddNodeRequest{
				HostID: hostID2,
				Role:   NodeRoleWorker,
			})
			if err != nil {
				t.Logf("Failed to add node 2: %v", err)
				return false
			}

			// Get cluster status
			// 获取集群状态
			status, err := svc.GetStatus(ctx, cluster.ID)
			if err != nil {
				t.Logf("Failed to get cluster status: %v", err)
				return false
			}

			// Verify cluster is healthy because all nodes are online
			// 验证集群健康，因为所有节点都在线
			if status.HealthStatus != HealthStatusHealthy {
				t.Logf("Cluster should be healthy when all nodes are online, got: %s", status.HealthStatus)
				return false
			}

			// Verify node counts
			// 验证节点计数
			if status.TotalNodes != 2 {
				t.Logf("Expected 2 total nodes, got: %d", status.TotalNodes)
				return false
			}

			if status.OnlineNodes != 2 {
				t.Logf("Expected 2 online nodes, got: %d", status.OnlineNodes)
				return false
			}

			if status.OfflineNodes != 0 {
				t.Logf("Expected 0 offline nodes, got: %d", status.OfflineNodes)
				return false
			}

			return true
		},
		genValidClusterName(),
		genValidHostID(),
		genValidHostID(),
	))

	properties.Property("cluster health is unknown when no nodes exist", prop.ForAll(
		func(clusterName string) bool {
			db, cleanup := setupServiceTestDB(t)
			defer cleanup()

			repo := NewRepository(db)
			mockHostProvider := NewMockHostProvider()

			svc := NewService(repo, mockHostProvider, nil)
			ctx := context.Background()

			// Create a cluster without nodes
			// 创建一个没有节点的集群
			cluster, err := svc.Create(ctx, &CreateClusterRequest{
				Name:           clusterName,
				DeploymentMode: DeploymentModeHybrid,
			})
			if err != nil {
				t.Logf("Failed to create cluster: %v", err)
				return false
			}

			// Get cluster status
			// 获取集群状态
			status, err := svc.GetStatus(ctx, cluster.ID)
			if err != nil {
				t.Logf("Failed to get cluster status: %v", err)
				return false
			}

			// Verify cluster health is unknown when no nodes
			// 验证没有节点时集群健康状态为未知
			if status.HealthStatus != HealthStatusUnknown {
				t.Logf("Cluster health should be unknown when no nodes, got: %s", status.HealthStatus)
				return false
			}

			if status.TotalNodes != 0 {
				t.Logf("Expected 0 total nodes, got: %d", status.TotalNodes)
				return false
			}

			return true
		},
		genValidClusterName(),
	))

	properties.TestingRun(t)
}

// TestClusterServiceBasicOperations tests basic CRUD operations
// TestClusterServiceBasicOperations 测试基本的 CRUD 操作
func TestClusterServiceBasicOperations(t *testing.T) {
	db, cleanup := setupServiceTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	mockHostProvider := NewMockHostProvider()
	svc := NewService(repo, mockHostProvider, nil)
	ctx := context.Background()

	// Test Create
	// 测试创建
	cluster, err := svc.Create(ctx, &CreateClusterRequest{
		Name:           "test-cluster",
		Description:    "Test cluster description",
		DeploymentMode: DeploymentModeHybrid,
		Version:        "2.3.0",
	})
	if err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}
	if cluster.Name != "test-cluster" {
		t.Errorf("Expected name 'test-cluster', got '%s'", cluster.Name)
	}
	if cluster.Status != ClusterStatusCreated {
		t.Errorf("Expected status 'created', got '%s'", cluster.Status)
	}

	// Test Get
	// 测试获取
	retrieved, err := svc.Get(ctx, cluster.ID)
	if err != nil {
		t.Fatalf("Failed to get cluster: %v", err)
	}
	if retrieved.Name != cluster.Name {
		t.Errorf("Retrieved cluster name mismatch")
	}

	// Test Update
	// 测试更新
	newName := "updated-cluster"
	updated, err := svc.Update(ctx, cluster.ID, &UpdateClusterRequest{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("Failed to update cluster: %v", err)
	}
	if updated.Name != newName {
		t.Errorf("Expected updated name '%s', got '%s'", newName, updated.Name)
	}

	// Test List
	// 测试列表
	clusters, total, err := svc.List(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list clusters: %v", err)
	}
	if total != 1 {
		t.Errorf("Expected 1 cluster, got %d", total)
	}
	if len(clusters) != 1 {
		t.Errorf("Expected 1 cluster in list, got %d", len(clusters))
	}

	// Test Delete
	// 测试删除
	err = svc.Delete(ctx, cluster.ID)
	if err != nil {
		t.Fatalf("Failed to delete cluster: %v", err)
	}

	// Verify deletion
	// 验证删除
	_, err = svc.Get(ctx, cluster.ID)
	if err != ErrClusterNotFound {
		t.Errorf("Expected ErrClusterNotFound after deletion, got: %v", err)
	}
}

// TestClusterServiceDeleteConstraint tests that running clusters cannot be deleted
// TestClusterServiceDeleteConstraint 测试运行中的集群不能被删除
func TestClusterServiceDeleteConstraint(t *testing.T) {
	db, cleanup := setupServiceTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	mockHostProvider := NewMockHostProvider()
	svc := NewService(repo, mockHostProvider, nil)
	ctx := context.Background()

	// Create a cluster
	// 创建一个集群
	cluster, err := svc.Create(ctx, &CreateClusterRequest{
		Name:           "running-cluster",
		DeploymentMode: DeploymentModeHybrid,
	})
	if err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}

	// Set cluster status to running
	// 设置集群状态为运行中
	err = svc.UpdateStatus(ctx, cluster.ID, ClusterStatusRunning)
	if err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	// Try to delete running cluster
	// 尝试删除运行中的集群
	err = svc.Delete(ctx, cluster.ID)
	if err != ErrClusterHasRunningTask {
		t.Errorf("Expected ErrClusterHasRunningTask, got: %v", err)
	}

	// Verify cluster still exists
	// 验证集群仍然存在
	_, err = svc.Get(ctx, cluster.ID)
	if err != nil {
		t.Errorf("Cluster should still exist after failed deletion")
	}
}
