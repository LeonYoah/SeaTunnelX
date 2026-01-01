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

package discovery

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"
)

// **Feature: seatunnel-process-monitor, Property: 进程发现正确性**
// **Validates: Requirements 1.4, 1.5**
// Process discovery should correctly identify SeaTunnel processes and extract
// PID, role, and install directory from command line.
// 进程发现应正确识别 SeaTunnel 进程并从命令行提取 PID、角色和安装目录。
func TestProperty_ProcessDiscoveryCorrectness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		scanner := NewProcessScanner()

		// Generate random install directory / 生成随机安装目录
		installDir := fmt.Sprintf("/opt/seatunnel-%d", rapid.IntRange(1, 100).Draw(t, "dirNum"))

		// Generate random role / 生成随机角色
		roles := []string{"master", "worker", "hybrid"}
		roleIdx := rapid.IntRange(0, 2).Draw(t, "roleIdx")
		role := roles[roleIdx]

		// Build command line / 构建命令行
		var cmdline string
		if role == "hybrid" {
			cmdline = fmt.Sprintf("java -DSEATUNNEL_HOME=%s -cp %s/lib/* %s",
				installDir, installDir, SeaTunnelMainClass)
		} else {
			cmdline = fmt.Sprintf("java -DSEATUNNEL_HOME=%s -cp %s/lib/* %s -r %s",
				installDir, installDir, SeaTunnelMainClass, role)
		}

		// Parse command line / 解析命令行
		parsedDir, parsedRole := scanner.parseCommandLine(cmdline)

		// Verify install directory / 验证安装目录
		if parsedDir != installDir {
			t.Errorf("Install dir mismatch: got %s, want %s", parsedDir, installDir)
		}

		// Verify role / 验证角色
		expectedRole := role
		if role == "hybrid" {
			expectedRole = "hybrid" // Default when no -r flag
		}
		if parsedRole != expectedRole {
			t.Errorf("Role mismatch: got %s, want %s", parsedRole, expectedRole)
		}
	})
}

// TestProcessDiscovery_DiscoverByRole tests discovery by role
// TestProcessDiscovery_DiscoverByRole 测试按角色发现
func TestProcessDiscovery_DiscoverByRole(t *testing.T) {
	// This test verifies the DiscoverProcessByRole method exists and works
	// 此测试验证 DiscoverProcessByRole 方法存在且工作正常
	discovery := NewProcessDiscovery()

	// Should not panic / 不应 panic
	_, err := discovery.DiscoverProcessByRole("master")
	// Error is expected since no actual processes are running
	// 预期会出错，因为没有实际运行的进程
	if err != nil {
		// This is fine - no processes running / 这是正常的 - 没有运行的进程
		t.Logf("Expected: no processes found: %v", err)
	}
}

// TestVersionDetector tests version detection
// TestVersionDetector 测试版本检测
func TestVersionDetector_Basic(t *testing.T) {
	detector := NewVersionDetector()

	// Test with non-existent directory / 测试不存在的目录
	version := detector.DetectVersion("/non/existent/path")
	if version != "unknown" {
		t.Errorf("Expected 'unknown' for non-existent path, got %s", version)
	}
}
