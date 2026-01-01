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

// **Feature: seatunnel-process-monitor, Property 1: SeaTunnel 进程识别正确性**
// **Validates: Requirements 1.4, 2.1**
// For any Java process command line, if it contains the SeaTunnel main class
// org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer,
// the process scanner should identify it as a SeaTunnel process.
// 对于任何 Java 进程命令行，如果包含 SeaTunnel 主类，
// 进程扫描器应将其识别为 SeaTunnel 进程。
func TestProperty_SeaTunnelProcessIdentification(t *testing.T) {
	scanner := NewProcessScanner()

	rapid.Check(t, func(t *rapid.T) {
		// Generate random prefix and suffix / 生成随机前缀和后缀
		prefix := rapid.StringMatching(`[a-zA-Z0-9/\-_\s]{0,50}`).Draw(t, "prefix")
		suffix := rapid.StringMatching(`[a-zA-Z0-9/\-_\s]{0,50}`).Draw(t, "suffix")

		// Test with SeaTunnel main class / 测试包含 SeaTunnel 主类
		cmdlineWithMainClass := fmt.Sprintf("%s %s %s", prefix, SeaTunnelMainClass, suffix)
		if !scanner.IsSeaTunnelProcess(cmdlineWithMainClass) {
			t.Errorf("Should identify as SeaTunnel process: %s", cmdlineWithMainClass)
		}

		// Test without SeaTunnel main class / 测试不包含 SeaTunnel 主类
		cmdlineWithoutMainClass := fmt.Sprintf("%s some.other.MainClass %s", prefix, suffix)
		if scanner.IsSeaTunnelProcess(cmdlineWithoutMainClass) {
			t.Errorf("Should NOT identify as SeaTunnel process: %s", cmdlineWithoutMainClass)
		}
	})
}

// **Feature: seatunnel-process-monitor, Property 2: 进程参数解析完整性**
// **Validates: Requirements 1.5**
// For any SeaTunnel process command line arguments,
// parsing should correctly extract install directory and node role.
// 对于任何 SeaTunnel 进程命令行参数，
// 解析后应能正确提取安装目录和节点角色信息。
func TestProperty_ProcessArgsParsingCompleteness(t *testing.T) {
	scanner := NewProcessScanner()

	rapid.Check(t, func(t *rapid.T) {
		// Generate random install directory / 生成随机安装目录
		installDir := rapid.StringMatching(`/[a-z]+(/[a-z]+){0,3}`).Draw(t, "installDir")
		if installDir == "" {
			installDir = "/opt/seatunnel"
		}

		// Generate random role / 生成随机角色
		roleOptions := []string{"", "master", "worker"}
		roleIdx := rapid.IntRange(0, len(roleOptions)-1).Draw(t, "roleIdx")
		role := roleOptions[roleIdx]

		// Build command line / 构建命令行
		cmdline := fmt.Sprintf("java -DSEATUNNEL_HOME=%s %s", installDir, SeaTunnelMainClass)
		if role != "" {
			cmdline += fmt.Sprintf(" -r %s", role)
		}

		// Parse using parseCommandLine / 使用 parseCommandLine 解析
		parsedDir, parsedRole := scanner.parseCommandLine(cmdline)

		// Verify install directory / 验证安装目录
		if parsedDir != installDir {
			t.Errorf("Install dir mismatch: got %s, want %s", parsedDir, installDir)
		}

		// Verify role / 验证角色
		expectedRole := role
		if expectedRole == "" {
			expectedRole = "hybrid"
		}
		if parsedRole != expectedRole {
			t.Errorf("Role mismatch: got %s, want %s", parsedRole, expectedRole)
		}
	})
}

// TestProcessScanner_ParseCommandLine tests parsing command line
// TestProcessScanner_ParseCommandLine 测试解析命令行
func TestProcessScanner_ParseCommandLine(t *testing.T) {
	scanner := NewProcessScanner()

	testCases := []struct {
		name     string
		cmdline  string
		wantDir  string
		wantRole string
	}{
		{
			name:     "hybrid mode with SEATUNNEL_HOME",
			cmdline:  "java -DSEATUNNEL_HOME=/opt/seatunnel org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer",
			wantDir:  "/opt/seatunnel",
			wantRole: "hybrid",
		},
		{
			name:     "master mode",
			cmdline:  "java -DSEATUNNEL_HOME=/opt/seatunnel org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer -r master",
			wantDir:  "/opt/seatunnel",
			wantRole: "master",
		},
		{
			name:     "worker mode",
			cmdline:  "java -DSEATUNNEL_HOME=/opt/seatunnel org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer -r worker",
			wantDir:  "/opt/seatunnel",
			wantRole: "worker",
		},
		{
			name:     "extract from classpath",
			cmdline:  "java -classpath /opt/seatunnel/lib/seatunnel.jar org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer",
			wantDir:  "/opt/seatunnel",
			wantRole: "hybrid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir, role := scanner.parseCommandLine(tc.cmdline)

			if dir != tc.wantDir {
				t.Errorf("InstallDir: got %s, want %s", dir, tc.wantDir)
			}

			if role != tc.wantRole {
				t.Errorf("Role: got %s, want %s", role, tc.wantRole)
			}
		})
	}
}

// TestProcessScanner_IsSeaTunnelProcess tests SeaTunnel process identification
// TestProcessScanner_IsSeaTunnelProcess 测试 SeaTunnel 进程识别
func TestProcessScanner_IsSeaTunnelProcess(t *testing.T) {
	scanner := NewProcessScanner()

	testCases := []struct {
		name    string
		cmdline string
		want    bool
	}{
		{
			name:    "SeaTunnel process",
			cmdline: "java -DSEATUNNEL_HOME=/opt/seatunnel org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer",
			want:    true,
		},
		{
			name:    "Other Java process",
			cmdline: "java -jar /opt/app/myapp.jar",
			want:    false,
		},
		{
			name:    "Non-Java process",
			cmdline: "/usr/bin/python3 script.py",
			want:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := scanner.IsSeaTunnelProcess(tc.cmdline)
			if got != tc.want {
				t.Errorf("IsSeaTunnelProcess: got %v, want %v", got, tc.want)
			}
		})
	}
}
