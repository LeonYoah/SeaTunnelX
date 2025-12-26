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

package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	pb "github.com/seatunnel/seatunnelX/agent"
	"github.com/seatunnel/seatunnelX/agent/internal/installer"
)

// PrecheckSubCommand defines the sub-command types for precheck
// PrecheckSubCommand 定义预检查的子命令类型
type PrecheckSubCommand string

const (
	// PrecheckSubCommandCheckPort checks if a port is available or listening
	// PrecheckSubCommandCheckPort 检查端口是否可用或正在监听
	PrecheckSubCommandCheckPort PrecheckSubCommand = "check_port"

	// PrecheckSubCommandCheckDirectory checks if a directory exists and is writable
	// PrecheckSubCommandCheckDirectory 检查目录是否存在且可写
	PrecheckSubCommandCheckDirectory PrecheckSubCommand = "check_directory"

	// PrecheckSubCommandCheckHTTP checks if an HTTP endpoint is accessible
	// PrecheckSubCommandCheckHTTP 检查 HTTP 端点是否可访问
	PrecheckSubCommandCheckHTTP PrecheckSubCommand = "check_http"

	// PrecheckSubCommandCheckProcess checks if a SeaTunnel process is running
	// PrecheckSubCommandCheckProcess 检查 SeaTunnel 进程是否正在运行
	PrecheckSubCommandCheckProcess PrecheckSubCommand = "check_process"

	// PrecheckSubCommandCheckJava checks if Java is installed and its version
	// PrecheckSubCommandCheckJava 检查 Java 是否已安装及其版本
	PrecheckSubCommandCheckJava PrecheckSubCommand = "check_java"

	// PrecheckSubCommandFull runs all precheck items
	// PrecheckSubCommandFull 运行所有预检查项
	PrecheckSubCommandFull PrecheckSubCommand = "full"
)

// PrecheckResult represents the result of a precheck operation
// PrecheckResult 表示预检查操作的结果
type PrecheckResult struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// RegisterPrecheckHandlers registers all precheck-related command handlers
// RegisterPrecheckHandlers 注册所有预检查相关的命令处理器
func RegisterPrecheckHandlers(executor *CommandExecutor) {
	executor.RegisterHandler(pb.CommandType_PRECHECK, HandlePrecheckCommand)
}

// HandlePrecheckCommand handles the PRECHECK command type
// HandlePrecheckCommand 处理 PRECHECK 命令类型
func HandlePrecheckCommand(ctx context.Context, cmd *pb.CommandRequest, reporter ProgressReporter) (*pb.CommandResponse, error) {
	subCommand := PrecheckSubCommand(cmd.Parameters["sub_command"])
	if subCommand == "" {
		subCommand = PrecheckSubCommandFull
	}

	var result *PrecheckResult
	var err error

	switch subCommand {
	case PrecheckSubCommandCheckPort:
		result, err = handleCheckPort(ctx, cmd.Parameters)
	case PrecheckSubCommandCheckDirectory:
		result, err = handleCheckDirectory(ctx, cmd.Parameters)
	case PrecheckSubCommandCheckHTTP:
		result, err = handleCheckHTTP(ctx, cmd.Parameters)
	case PrecheckSubCommandCheckProcess:
		result, err = handleCheckProcess(ctx, cmd.Parameters)
	case PrecheckSubCommandCheckJava:
		result, err = handleCheckJava(ctx, cmd.Parameters)
	case PrecheckSubCommandFull:
		result, err = handleFullPrecheck(ctx, cmd.Parameters, reporter)
	default:
		return CreateErrorResponse(cmd.CommandId, fmt.Sprintf("unknown precheck sub-command: %s", subCommand)), nil
	}

	if err != nil {
		return CreateErrorResponse(cmd.CommandId, err.Error()), nil
	}

	output, err := json.Marshal(result)
	if err != nil {
		return CreateErrorResponse(cmd.CommandId, fmt.Sprintf("failed to serialize result: %v", err)), nil
	}

	if result.Success {
		return CreateSuccessResponse(cmd.CommandId, string(output)), nil
	}
	return CreateErrorResponse(cmd.CommandId, string(output)), nil
}

// handleCheckPort handles the check_port sub-command
// handleCheckPort 处理 check_port 子命令
func handleCheckPort(ctx context.Context, params map[string]string) (*PrecheckResult, error) {
	portStr := params["port"]
	if portStr == "" {
		return &PrecheckResult{
			Success: false,
			Message: "port parameter is required",
		}, nil
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return &PrecheckResult{
			Success: false,
			Message: fmt.Sprintf("invalid port number: %s", portStr),
		}, nil
	}

	checkResult := installer.CheckPortListening(port)

	return &PrecheckResult{
		Success: checkResult.Success,
		Message: checkResult.Message,
		Details: map[string]string{
			"port": portStr,
		},
	}, nil
}

// handleCheckDirectory handles the check_directory sub-command
// handleCheckDirectory 处理 check_directory 子命令
func handleCheckDirectory(ctx context.Context, params map[string]string) (*PrecheckResult, error) {
	path := params["path"]
	if path == "" {
		return &PrecheckResult{
			Success: false,
			Message: "path parameter is required",
		}, nil
	}

	checkResult := installer.CheckDirectoryExists(path)

	return &PrecheckResult{
		Success: checkResult.Success,
		Message: checkResult.Message,
		Details: map[string]string{
			"path": path,
		},
	}, nil
}

// handleCheckHTTP handles the check_http sub-command
// handleCheckHTTP 处理 check_http 子命令
func handleCheckHTTP(ctx context.Context, params map[string]string) (*PrecheckResult, error) {
	url := params["url"]
	if url == "" {
		return &PrecheckResult{
			Success: false,
			Message: "url parameter is required",
		}, nil
	}

	checkResult := installer.CheckHTTPEndpoint(url)

	return &PrecheckResult{
		Success: checkResult.Success,
		Message: checkResult.Message,
		Details: map[string]string{
			"url": url,
		},
	}, nil
}

// handleCheckProcess handles the check_process sub-command
// handleCheckProcess 处理 check_process 子命令
func handleCheckProcess(ctx context.Context, params map[string]string) (*PrecheckResult, error) {
	role := params["role"]
	if role == "" {
		role = "hybrid"
	}

	checkResult := installer.CheckSeaTunnelRunning(ctx, role)

	details := map[string]string{
		"role": role,
	}

	if checkResult.Success {
		processInfo, err := installer.CheckSeaTunnelProcess(ctx, role)
		if err == nil && processInfo != nil {
			details["pid"] = strconv.Itoa(processInfo.PID)
			details["actual_role"] = processInfo.Role
			details["start_time"] = processInfo.StartTime
		}
	}

	return &PrecheckResult{
		Success: checkResult.Success,
		Message: checkResult.Message,
		Details: details,
	}, nil
}

// handleCheckJava handles the check_java sub-command
// handleCheckJava 处理 check_java 子命令
func handleCheckJava(ctx context.Context, params map[string]string) (*PrecheckResult, error) {
	prechecker := installer.NewPrechecker(nil)
	item := prechecker.CheckJava(ctx)

	details := make(map[string]string)
	for k, v := range item.Details {
		details[k] = fmt.Sprintf("%v", v)
	}

	return &PrecheckResult{
		Success: item.Status == installer.CheckStatusPassed || item.Status == installer.CheckStatusWarning,
		Message: item.Message,
		Details: details,
	}, nil
}

// handleFullPrecheck handles the full precheck sub-command
// handleFullPrecheck 处理完整预检查子命令
func handleFullPrecheck(ctx context.Context, params map[string]string, reporter ProgressReporter) (*PrecheckResult, error) {
	results := make(map[string]string)
	allPassed := true

	if reporter != nil {
		reporter.Report(0, "Starting full precheck...")
	}

	// 1. Check install directory if provided
	if path := params["install_dir"]; path != "" {
		if reporter != nil {
			reporter.Report(20, fmt.Sprintf("Checking directory: %s", path))
		}
		dirResult := installer.CheckDirectoryExists(path)
		results["directory_check"] = dirResult.Message
		if !dirResult.Success {
			allPassed = false
		}
	}

	// 2. Check Hazelcast port if provided
	if portStr := params["hazelcast_port"]; portStr != "" {
		if reporter != nil {
			reporter.Report(40, fmt.Sprintf("Checking Hazelcast port: %s", portStr))
		}
		port, err := strconv.Atoi(portStr)
		if err == nil {
			portResult := installer.CheckPortListening(port)
			if portResult.Success {
				results["hazelcast_port_check"] = fmt.Sprintf("Port %d is already in use", port)
				allPassed = false
			} else {
				results["hazelcast_port_check"] = fmt.Sprintf("Port %d is available", port)
			}
		}
	}

	// 3. Check API port if provided
	if portStr := params["api_port"]; portStr != "" {
		if reporter != nil {
			reporter.Report(60, fmt.Sprintf("Checking API port: %s", portStr))
		}
		port, err := strconv.Atoi(portStr)
		if err == nil && port > 0 {
			portResult := installer.CheckPortListening(port)
			if portResult.Success {
				results["api_port_check"] = fmt.Sprintf("Port %d is already in use", port)
				allPassed = false
			} else {
				results["api_port_check"] = fmt.Sprintf("Port %d is available", port)
			}
		}
	}

	// 4. Check if SeaTunnel process is already running
	if reporter != nil {
		reporter.Report(80, "Checking for existing SeaTunnel processes...")
	}
	role := params["role"]
	if role == "" {
		role = "hybrid"
	}
	processResult := installer.CheckSeaTunnelRunning(ctx, role)
	if processResult.Success {
		results["process_check"] = fmt.Sprintf("SeaTunnel process is already running: %s", processResult.Message)
	} else {
		results["process_check"] = "No existing SeaTunnel process found"
	}

	if reporter != nil {
		reporter.Report(100, "Full precheck completed")
	}

	message := "All precheck items passed"
	if !allPassed {
		message = "Some precheck items failed"
	}

	return &PrecheckResult{
		Success: allPassed,
		Message: message,
		Details: results,
	}, nil
}
