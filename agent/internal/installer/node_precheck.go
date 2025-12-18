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

// Package installer provides SeaTunnel installation management for the Agent.
// installer 包提供 Agent 的 SeaTunnel 安装管理功能。
package installer

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// NodePrecheckResult represents the result of a node precheck
// NodePrecheckResult 表示节点预检查的结果
type NodePrecheckResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// CheckPortListening checks if a port is listening (service is running)
// CheckPortListening 检查端口是否正在监听（服务正在运行）
func CheckPortListening(port int) *NodePrecheckResult {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return &NodePrecheckResult{
			Success: false,
			Message: fmt.Sprintf("Port %d is not listening: %v", port, err),
		}
	}
	conn.Close()
	return &NodePrecheckResult{
		Success: true,
		Message: fmt.Sprintf("Port %d is listening", port),
	}
}


// CheckDirectoryExists checks if a directory exists and is writable
// CheckDirectoryExists 检查目录是否存在且可写
func CheckDirectoryExists(path string) *NodePrecheckResult {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return &NodePrecheckResult{
			Success: false,
			Message: fmt.Sprintf("Directory %s does not exist", path),
		}
	}
	if err != nil {
		return &NodePrecheckResult{
			Success: false,
			Message: fmt.Sprintf("Failed to check directory %s: %v", path, err),
		}
	}

	if !info.IsDir() {
		return &NodePrecheckResult{
			Success: false,
			Message: fmt.Sprintf("Path %s is not a directory", path),
		}
	}

	// Check if writable by creating a temp file
	testFile := fmt.Sprintf("%s/.seatunnel_write_test_%d", path, time.Now().UnixNano())
	f, err := os.Create(testFile)
	if err != nil {
		return &NodePrecheckResult{
			Success: false,
			Message: fmt.Sprintf("Directory %s is not writable: %v", path, err),
		}
	}
	f.Close()
	os.Remove(testFile)

	return &NodePrecheckResult{
		Success: true,
		Message: fmt.Sprintf("Directory %s exists and is writable", path),
	}
}

// CheckHTTPEndpoint checks if an HTTP endpoint is accessible
// CheckHTTPEndpoint 检查 HTTP 端点是否可访问
func CheckHTTPEndpoint(url string) *NodePrecheckResult {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return &NodePrecheckResult{
			Success: false,
			Message: fmt.Sprintf("HTTP endpoint %s is not accessible: %v", url, err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return &NodePrecheckResult{
			Success: true,
			Message: fmt.Sprintf("HTTP endpoint %s is accessible (status: %d)", url, resp.StatusCode),
		}
	}

	return &NodePrecheckResult{
		Success: false,
		Message: fmt.Sprintf("HTTP endpoint %s returned error status: %d", url, resp.StatusCode),
	}
}


// SeaTunnelProcessInfo represents information about a SeaTunnel process
// SeaTunnelProcessInfo 表示 SeaTunnel 进程的信息
type SeaTunnelProcessInfo struct {
	PID       int    `json:"pid"`
	Role      string `json:"role"` // "hybrid", "master", "worker"
	CmdLine   string `json:"cmd_line"`
	StartTime string `json:"start_time"`
}

// CheckSeaTunnelProcess checks for running SeaTunnel processes
// CheckSeaTunnelProcess 检查正在运行的 SeaTunnel 进程
func CheckSeaTunnelProcess(ctx context.Context, role string) (*SeaTunnelProcessInfo, error) {
	var grepPattern string

	switch role {
	case "hybrid":
		grepPattern = `org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer`
	case "master":
		grepPattern = `org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer.*-r master`
	case "worker":
		grepPattern = `org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer.*-r worker`
	default:
		grepPattern = `org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer`
	}

	cmd := exec.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf(`ps -ef | grep "%s" | grep -v grep`, grepPattern))
	output, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return nil, nil
	}

	lines := strings.Split(outputStr, "\n")
	if len(lines) == 0 {
		return nil, nil
	}

	fields := strings.Fields(lines[0])
	if len(fields) < 8 {
		return nil, fmt.Errorf("unexpected ps output format")
	}

	pid, err := strconv.Atoi(fields[1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse PID: %w", err)
	}

	actualRole := "hybrid"
	cmdLine := strings.Join(fields[7:], " ")
	if strings.Contains(cmdLine, "-r master") {
		actualRole = "master"
	} else if strings.Contains(cmdLine, "-r worker") {
		actualRole = "worker"
	}

	return &SeaTunnelProcessInfo{
		PID:       pid,
		Role:      actualRole,
		CmdLine:   cmdLine,
		StartTime: fields[4],
	}, nil
}

// GetSeaTunnelProcessPID returns the PID of a running SeaTunnel process
// GetSeaTunnelProcessPID 返回正在运行的 SeaTunnel 进程的 PID
func GetSeaTunnelProcessPID(ctx context.Context, role string) (int, error) {
	info, err := CheckSeaTunnelProcess(ctx, role)
	if err != nil {
		return 0, err
	}
	if info == nil {
		return 0, nil
	}
	return info.PID, nil
}

// CheckSeaTunnelRunning checks if SeaTunnel is running
// CheckSeaTunnelRunning 检查 SeaTunnel 是否正在运行
func CheckSeaTunnelRunning(ctx context.Context, role string) *NodePrecheckResult {
	info, err := CheckSeaTunnelProcess(ctx, role)
	if err != nil {
		return &NodePrecheckResult{
			Success: false,
			Message: fmt.Sprintf("Failed to check SeaTunnel process: %v", err),
		}
	}

	if info == nil {
		return &NodePrecheckResult{
			Success: false,
			Message: fmt.Sprintf("SeaTunnel process (role: %s) is not running", role),
		}
	}

	return &NodePrecheckResult{
		Success: true,
		Message: fmt.Sprintf("SeaTunnel process found: PID=%d, role=%s", info.PID, info.Role),
	}
}

// GetAllSeaTunnelProcesses returns all running SeaTunnel processes
// GetAllSeaTunnelProcesses 返回所有正在运行的 SeaTunnel 进程
func GetAllSeaTunnelProcesses(ctx context.Context) ([]*SeaTunnelProcessInfo, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c",
		`ps -ef | grep "org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer" | grep -v grep`)
	output, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return nil, nil
	}

	lines := strings.Split(outputStr, "\n")
	processes := make([]*SeaTunnelProcessInfo, 0, len(lines))

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		cmdLine := strings.Join(fields[7:], " ")
		role := "hybrid"
		if strings.Contains(cmdLine, "-r master") {
			role = "master"
		} else if strings.Contains(cmdLine, "-r worker") {
			role = "worker"
		}

		processes = append(processes, &SeaTunnelProcessInfo{
			PID:       pid,
			Role:      role,
			CmdLine:   cmdLine,
			StartTime: fields[4],
		})
	}

	return processes, nil
}

// ExtractPortFromConfig extracts port configuration from SeaTunnel config file
// ExtractPortFromConfig 从 SeaTunnel 配置文件中提取端口配置
func ExtractPortFromConfig(configPath string, portName string) (int, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read config file: %w", err)
	}

	patterns := []string{
		fmt.Sprintf(`%s\s*[=:]\s*(\d+)`, regexp.QuoteMeta(portName)),
		fmt.Sprintf(`"%s"\s*[=:]\s*(\d+)`, regexp.QuoteMeta(portName)),
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(string(content))
		if len(matches) > 1 {
			port, err := strconv.Atoi(matches[1])
			if err == nil {
				return port, nil
			}
		}
	}

	return 0, fmt.Errorf("port %s not found in config", portName)
}
