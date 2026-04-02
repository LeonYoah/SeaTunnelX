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
	"testing"
	"time"
)

type seatunnelxJavaProxyAgentSender struct {
	lastAgentID string
	lastCommand string
	lastParams  map[string]string
	response    string
	success     bool
	err         error
}

func (m *seatunnelxJavaProxyAgentSender) SendCommand(ctx context.Context, agentID string, commandType string, params map[string]string) (bool, string, error) {
	m.lastAgentID = agentID
	m.lastCommand = commandType
	m.lastParams = map[string]string{}
	for k, v := range params {
		m.lastParams[k] = v
	}
	return m.success, m.response, m.err
}

func TestGetSeatunnelXJavaProxyStatusUsesOnlineMasterNode(t *testing.T) {
	db, cleanup := setupServiceTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	hostProvider := NewMockHostProvider()
	service := NewService(repo, hostProvider, nil)
	lastHeartbeat := time.Now()
	hostProvider.AddHost(&HostInfo{
		ID:            1,
		Name:          "master-1",
		IPAddress:     "10.0.0.1",
		AgentID:       "agent-master-1",
		AgentStatus:   "installed",
		LastHeartbeat: &lastHeartbeat,
	})

	agentSender := &seatunnelxJavaProxyAgentSender{
		success:  true,
		response: `{"service":"seatunnelx_java_proxy","managed":true,"running":true,"healthy":true,"endpoint":"http://127.0.0.1:18080","port":18080,"pid":4567,"log_path":"/opt/seatunnel/.seatunnelx/seatunnelx-java-proxy/service.log","message":"ok"}`,
	}
	service.SetAgentCommandSender(agentSender)

	clusterInfo, err := service.Create(context.Background(), &CreateClusterRequest{
		Name:           "cluster-a",
		Version:        "2.3.13",
		InstallDir:     "/opt/seatunnel",
		DeploymentMode: DeploymentModeHybrid,
	})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if _, err := service.AddNode(context.Background(), clusterInfo.ID, &AddNodeRequest{
		HostID:     1,
		Role:       NodeRoleMasterWorker,
		InstallDir: "/opt/seatunnel",
	}); err != nil {
		t.Fatalf("add node: %v", err)
	}

	status, err := service.GetSeatunnelXJavaProxyStatus(context.Background(), clusterInfo.ID)
	if err != nil {
		t.Fatalf("GetSeatunnelXJavaProxyStatus returned error: %v", err)
	}
	if !status.Healthy || !status.Running {
		t.Fatalf("expected running healthy status, got %#v", status)
	}
	if agentSender.lastCommand != "status" {
		t.Fatalf("expected status command, got %s", agentSender.lastCommand)
	}
	if agentSender.lastParams["service"] != "seatunnelx_java_proxy" {
		t.Fatalf("expected service param seatunnelx_java_proxy, got %#v", agentSender.lastParams)
	}
}

func TestStartSeatunnelXJavaProxyPropagatesAgentFailure(t *testing.T) {
	db, cleanup := setupServiceTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	hostProvider := NewMockHostProvider()
	service := NewService(repo, hostProvider, nil)
	lastHeartbeat := time.Now()
	hostProvider.AddHost(&HostInfo{
		ID:            1,
		Name:          "master-1",
		IPAddress:     "10.0.0.1",
		AgentID:       "agent-master-1",
		AgentStatus:   "installed",
		LastHeartbeat: &lastHeartbeat,
	})
	service.SetAgentCommandSender(&seatunnelxJavaProxyAgentSender{
		success:  false,
		response: `{"service":"seatunnelx_java_proxy","managed":true,"running":false,"healthy":false,"message":"start failed"}`,
	})

	clusterInfo, err := service.Create(context.Background(), &CreateClusterRequest{
		Name:           "cluster-a",
		Version:        "2.3.13",
		InstallDir:     "/opt/seatunnel",
		DeploymentMode: DeploymentModeHybrid,
	})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if _, err := service.AddNode(context.Background(), clusterInfo.ID, &AddNodeRequest{
		HostID:     1,
		Role:       NodeRoleMasterWorker,
		InstallDir: "/opt/seatunnel",
	}); err != nil {
		t.Fatalf("add node: %v", err)
	}

	status, err := service.StartSeatunnelXJavaProxy(context.Background(), clusterInfo.ID)
	if err == nil {
		t.Fatal("expected error from failed start command")
	}
	if status == nil || status.Message != "start failed" {
		t.Fatalf("expected decoded failure status, got %#v", status)
	}
}
