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

	"github.com/seatunnel/seatunnelX/internal/config"
)

type seatunnelxJavaProxyCommandRecord struct {
	agentID     string
	commandType string
	params      map[string]string
}

type seatunnelxJavaProxyAgentSender struct {
	lastAgentID        string
	lastCommand        string
	lastParams         map[string]string
	commands           []seatunnelxJavaProxyCommandRecord
	response           string
	responses          []string
	responsesByCommand map[string]string
	success            bool
	successes          []bool
	err                error
}

func (m *seatunnelxJavaProxyAgentSender) SendCommand(ctx context.Context, agentID string, commandType string, params map[string]string) (bool, string, error) {
	m.lastAgentID = agentID
	m.lastCommand = commandType
	m.lastParams = map[string]string{}
	for k, v := range params {
		m.lastParams[k] = v
	}
	m.commands = append(m.commands, seatunnelxJavaProxyCommandRecord{
		agentID:     agentID,
		commandType: commandType,
		params:      m.lastParams,
	})
	if len(m.responsesByCommand) > 0 {
		response, ok := m.responsesByCommand[commandType]
		if !ok {
			response = ""
		}
		success := m.success
		if len(m.successes) > 0 {
			success = m.successes[0]
			m.successes = m.successes[1:]
		}
		return success, response, m.err
	}
	if len(m.responses) > 0 {
		response := m.responses[0]
		m.responses = m.responses[1:]
		success := m.success
		if len(m.successes) > 0 {
			success = m.successes[0]
			m.successes = m.successes[1:]
		}
		return success, response, m.err
	}
	return m.success, m.response, m.err
}

func TestGetSeatunnelXJavaProxyStatusUsesOnlineMasterNode(t *testing.T) {
	db, cleanup := setupServiceTestDB(t)
	defer cleanup()
	oldDefaultPort := config.Config.JavaProxy.DefaultPort
	config.Config.JavaProxy.DefaultPort = 19080
	defer func() {
		config.Config.JavaProxy.DefaultPort = oldDefaultPort
	}()

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
	if agentSender.lastParams[seatunnelXJavaProxyDefaultPortParam] != "19080" {
		t.Fatalf("expected configured java proxy default port param, got %#v", agentSender.lastParams)
	}
	if status.Endpoint != "http://10.0.0.1:18080" || status.DirectEndpoint != "http://10.0.0.1:18080" {
		t.Fatalf("expected Control Plane direct endpoint to use node host IP, got %#v", status)
	}
	if status.LocalEndpoint != "http://127.0.0.1:18080" {
		t.Fatalf("expected Agent local endpoint to preserve reported localhost endpoint, got %#v", status)
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

func TestGetSeatunnelXJavaProxyServiceLogUsesReportedLogPath(t *testing.T) {
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
		success: true,
		responsesByCommand: map[string]string{
			"status":   `{"service":"seatunnelx_java_proxy","managed":true,"running":true,"healthy":true,"endpoint":"http://127.0.0.1:18080","port":18080,"pid":4567,"log_path":"/opt/seatunnel/.seatunnelx/seatunnelx-java-proxy/service.log","message":"ok"}`,
			"get_logs": "line-1\nline-2",
		},
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

	result, err := service.GetSeatunnelXJavaProxyServiceLog(context.Background(), clusterInfo.ID, 120)
	if err != nil {
		t.Fatalf("GetSeatunnelXJavaProxyServiceLog returned error: %v", err)
	}
	if result.LogPath != "/opt/seatunnel/.seatunnelx/seatunnelx-java-proxy/service.log" {
		t.Fatalf("unexpected log path: %#v", result)
	}
	if result.Logs != "line-1\nline-2" {
		t.Fatalf("unexpected log payload: %#v", result)
	}
	if agentSender.lastCommand != "get_logs" {
		t.Fatalf("expected get_logs command, got %s", agentSender.lastCommand)
	}
	if agentSender.lastParams["log_file"] != result.LogPath {
		t.Fatalf("expected get_logs to target reported log path, got %#v", agentSender.lastParams)
	}
}

func TestInstallOrRepairSeatunnelXJavaProxyTargetsSelectedNode(t *testing.T) {
	db, cleanup := setupServiceTestDB(t)
	defer cleanup()

	oldExternalURL := config.Config.App.ExternalURL
	config.Config.App.ExternalURL = "http://control-plane.example"
	defer func() {
		config.Config.App.ExternalURL = oldExternalURL
	}()

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
	hostProvider.AddHost(&HostInfo{
		ID:            2,
		Name:          "worker-1",
		IPAddress:     "10.0.0.2",
		AgentID:       "agent-worker-1",
		AgentStatus:   "installed",
		LastHeartbeat: &lastHeartbeat,
	})

	agentSender := &seatunnelxJavaProxyAgentSender{
		success: true,
		responsesByCommand: map[string]string{
			"seatunnelx_java_proxy_install": `{"message":"installed"}`,
			"status":                        `{"service":"seatunnelx_java_proxy","managed":true,"running":true,"healthy":true,"endpoint":"http://127.0.0.1:18080","port":18080,"pid":4567,"message":"ok"}`,
		},
	}
	service.SetAgentCommandSender(agentSender)

	clusterInfo, err := service.Create(context.Background(), &CreateClusterRequest{
		Name:           "cluster-a",
		Version:        "2.3.13",
		InstallDir:     "/opt/seatunnel",
		DeploymentMode: DeploymentModeSeparated,
	})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if _, err := service.AddNode(context.Background(), clusterInfo.ID, &AddNodeRequest{
		HostID:     1,
		Role:       NodeRoleMaster,
		InstallDir: "/opt/seatunnel",
	}); err != nil {
		t.Fatalf("add master node: %v", err)
	}
	worker, err := service.AddNode(context.Background(), clusterInfo.ID, &AddNodeRequest{
		HostID:     2,
		Role:       NodeRoleWorker,
		InstallDir: "/opt/seatunnel",
	})
	if err != nil {
		t.Fatalf("add worker node: %v", err)
	}

	agentSender.commands = nil
	status, err := service.InstallOrRepairSeatunnelXJavaProxy(context.Background(), clusterInfo.ID, worker.ID)
	if err != nil {
		t.Fatalf("InstallOrRepairSeatunnelXJavaProxy returned error: %v", err)
	}
	if len(agentSender.commands) < 2 {
		t.Fatalf("expected install and status commands, got %#v", agentSender.commands)
	}
	installCommand := agentSender.commands[0]
	if installCommand.agentID != "agent-worker-1" || installCommand.commandType != "seatunnelx_java_proxy_install" {
		t.Fatalf("expected install command to selected worker agent, got %#v", installCommand)
	}
	if installCommand.params["node_id"] != "2" {
		t.Fatalf("expected selected node id in install command, got %#v", installCommand.params)
	}
	if installCommand.params["jar_url"] == "" || installCommand.params["script_url"] == "" {
		t.Fatalf("expected support asset URLs in install command, got %#v", installCommand.params)
	}
	if installCommand.params[seatunnelXJavaProxyDefaultPortParam] == "" {
		t.Fatalf("expected java proxy default port in install command, got %#v", installCommand.params)
	}
	if status.Endpoint != "http://10.0.0.2:18080" || status.NodeID != worker.ID {
		t.Fatalf("expected refreshed direct status for selected worker, got %#v", status)
	}
}
