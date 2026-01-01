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
	"os"
	"path/filepath"
	"testing"
)

// TestVersionDetector_DetectVersion tests version detection from install directory
// TestVersionDetector_DetectVersion 测试从安装目录检测版本
func TestVersionDetector_DetectVersion(t *testing.T) {
	detector := NewVersionDetector()

	// Test with non-existent directory / 测试不存在的目录
	version := detector.DetectVersion("/non/existent/path")
	if version != "unknown" {
		t.Errorf("Expected 'unknown' for non-existent path, got %s", version)
	}
}

// TestVersionDetector_DetectFromConnectors tests version detection from connectors directory
// TestVersionDetector_DetectFromConnectors 测试从 connectors 目录检测版本
func TestVersionDetector_DetectFromConnectors(t *testing.T) {
	// Create temp directory / 创建临时目录
	tempDir, err := os.MkdirTemp("", "seatunnel-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create connectors directory with fake connector jar
	// 创建 connectors 目录并放入假的 connector jar
	connectorsDir := filepath.Join(tempDir, "connectors")
	if err := os.MkdirAll(connectorsDir, 0755); err != nil {
		t.Fatalf("Failed to create connectors dir: %v", err)
	}

	// Create fake connector jar / 创建假的 connector jar
	jarPath := filepath.Join(connectorsDir, "connector-fake-2.3.12.jar")
	if err := os.WriteFile(jarPath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create jar file: %v", err)
	}

	// Detect version / 检测版本
	detector := NewVersionDetector()
	version := detector.DetectVersion(tempDir)

	if version != "2.3.12" {
		t.Errorf("Expected version '2.3.12', got '%s'", version)
	}
}

// TestVersionDetector_DetectFromLib tests version detection from lib directory
// TestVersionDetector_DetectFromLib 测试从 lib 目录检测版本
func TestVersionDetector_DetectFromLib(t *testing.T) {
	// Create temp directory / 创建临时目录
	tempDir, err := os.MkdirTemp("", "seatunnel-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create lib directory with engine core jar
	// 创建 lib 目录并放入 engine core jar
	libDir := filepath.Join(tempDir, "lib")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatalf("Failed to create lib dir: %v", err)
	}

	// Create engine core jar / 创建 engine core jar
	jarPath := filepath.Join(libDir, "seatunnel-engine-core-2.3.10.jar")
	if err := os.WriteFile(jarPath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create jar file: %v", err)
	}

	// Detect version / 检测版本
	detector := NewVersionDetector()
	version := detector.DetectVersion(tempDir)

	if version != "2.3.10" {
		t.Errorf("Expected version '2.3.10', got '%s'", version)
	}
}

// TestConfigParser_ParseClusterConfig tests the legacy ParseClusterConfig method
// TestConfigParser_ParseClusterConfig 测试遗留的 ParseClusterConfig 方法
func TestConfigParser_ParseClusterConfig(t *testing.T) {
	parser := NewConfigParser()

	// Test with non-existent directory - should return defaults
	// 测试不存在的目录 - 应返回默认值
	config, err := parser.ParseClusterConfig("/non/existent/path", "hybrid")
	if err != nil {
		t.Fatalf("ParseClusterConfig should not return error: %v", err)
	}

	// Verify defaults / 验证默认值
	if config.ClusterName != "seatunnel" {
		t.Errorf("Expected default cluster name 'seatunnel', got '%s'", config.ClusterName)
	}

	if config.HazelcastPort != 5801 {
		t.Errorf("Expected default port 5801, got %d", config.HazelcastPort)
	}

	if config.Version != "unknown" {
		t.Errorf("Expected version 'unknown', got '%s'", config.Version)
	}
}
