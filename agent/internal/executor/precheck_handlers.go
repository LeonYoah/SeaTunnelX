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
// PrecheckSubCommand 瀹氫箟棰勬鏌ョ殑瀛愬懡浠ょ被鍨?
type PrecheckSubCommand string

const (
	// PrecheckSubCommandCheckPort checks if a port is available or listening
	// PrecheckSubCommandCheckPort 妫€鏌ョ鍙ｆ槸鍚﹀彲鐢ㄦ垨姝ｅ湪鐩戝惉
	PrecheckSubCommandCheckPort PrecheckSubCommand = "check_port"

	// PrecheckSubCommandCheckDirectory checks if a directory exists and is writable
	// PrecheckSubCommandCheckDirectory 妫€鏌ョ洰褰曟槸鍚﹀瓨鍦ㄤ笖鍙啓
	PrecheckSubCommandCheckDirectory PrecheckSubCommand = "check_directory"

	// PrecheckSubCommandCheckHTTP checks if an HTTP endpoint is accessible
	// PrecheckSubCommandCheckHTTP 妫€鏌?HTTP 绔偣鏄惁鍙闂?
	PrecheckSubCommandCheckHTTP PrecheckSubCommand = "check_http"

	// PrecheckSubCommandCheckProcess checks if a SeaTunnel process is running
	// PrecheckSubCommandCheckProcess 妫€鏌?SeaTunnel 杩涚▼鏄惁姝ｅ湪杩愯
	PrecheckSubCommandCheckProcess PrecheckSubCommand = "check_process"

	// PrecheckSubCommandFull runs all precheck items
	// PrecheckSubCommandFull 杩愯鎵€鏈夐妫€鏌ラ」
	PrecheckSubCommandFull PrecheckSubCommand = "full"
)

// PrecheckResult represents the result of a precheck operation
// PrecheckResult 琛ㄧず棰勬鏌ユ搷浣滅殑缁撴灉
type PrecheckResult struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// RegisterPrecheckHandlers registers all precheck-related command handlers
// RegisterPrecheckHandlers 娉ㄥ唽鎵€鏈夐妫€鏌ョ浉鍏崇殑鍛戒护澶勭悊鍣?
func RegisterPrecheckHandlers(executor *CommandExecutor) {
	executor.RegisterHandler(pb.CommandType_PRECHECK, HandlePrecheckCommand)
}

// HandlePrecheckCommand handles the PRECHECK command type
// HandlePrecheckCommand 澶勭悊 PRECHECK 鍛戒护绫诲瀷
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
// handleCheckPort 澶勭悊 check_port 瀛愬懡浠?
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
// handleCheckDirectory 澶勭悊 check_directory 瀛愬懡浠?
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
// handleCheckHTTP 澶勭悊 check_http 瀛愬懡浠?
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
// handleCheckProcess 澶勭悊 check_process 瀛愬懡浠?
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

// handleFullPrecheck handles the full precheck sub-command
// handleFullPrecheck 澶勭悊瀹屾暣棰勬鏌ュ瓙鍛戒护
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
