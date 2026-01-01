package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReproduction_VersionDetection verifies version detection from connectors directory
// TestReproduction_VersionDetection 验证从 connectors 目录检测版本
// Note: Config parsing has been simplified - we no longer parse hazelcast.yaml
// 注意：配置解析已简化 - 我们不再解析 hazelcast.yaml
func TestReproduction_VersionDetection(t *testing.T) {
	// Setup test environment
	tempDir, err := os.MkdirTemp("", "seatunnel-repro-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create connectors directory
	connectorsDir := filepath.Join(tempDir, "connectors")
	if err := os.MkdirAll(connectorsDir, 0755); err != nil {
		t.Fatalf("Failed to create connectors dir: %v", err)
	}

	// Create fake connector jar
	fakeJar := filepath.Join(connectorsDir, "connector-fake-2.3.12.jar")
	if err := os.WriteFile(fakeJar, []byte("fake jar content"), 0644); err != nil {
		t.Fatalf("Failed to create fake jar: %v", err)
	}

	// Test version detection
	detector := NewVersionDetector()
	version := detector.DetectVersion(tempDir)

	if version != "2.3.12" {
		t.Errorf("Expected version 2.3.12, got %s", version)
	}
}

// TestReproduction_SimplifiedConfigParser verifies the simplified ConfigParser
// TestReproduction_SimplifiedConfigParser 验证简化的 ConfigParser
// The simplified parser only returns defaults and detected version
// 简化的解析器只返回默认值和检测到的版本
func TestReproduction_SimplifiedConfigParser(t *testing.T) {
	// Setup test environment
	tempDir, err := os.MkdirTemp("", "seatunnel-repro-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create connectors directory with version jar
	connectorsDir := filepath.Join(tempDir, "connectors")
	if err := os.MkdirAll(connectorsDir, 0755); err != nil {
		t.Fatalf("Failed to create connectors dir: %v", err)
	}

	fakeJar := filepath.Join(connectorsDir, "connector-fake-2.3.12.jar")
	if err := os.WriteFile(fakeJar, []byte("fake jar content"), 0644); err != nil {
		t.Fatalf("Failed to create fake jar: %v", err)
	}

	parser := NewConfigParser()

	// Test that ParseClusterConfig returns defaults with detected version
	// 测试 ParseClusterConfig 返回带有检测版本的默认值
	config, err := parser.ParseClusterConfig(tempDir, "hybrid")
	if err != nil {
		t.Fatalf("ParseClusterConfig failed: %v", err)
	}

	// Should return default cluster name
	// 应返回默认集群名称
	if config.ClusterName != "seatunnel" {
		t.Errorf("Expected default cluster name 'seatunnel', got %s", config.ClusterName)
	}

	// Should return default port
	// 应返回默认端口
	if config.HazelcastPort != 5801 {
		t.Errorf("Expected default port 5801, got %d", config.HazelcastPort)
	}

	// Should detect version from connectors
	// 应从 connectors 检测版本
	if config.Version != "2.3.12" {
		t.Errorf("Expected version 2.3.12, got %s", config.Version)
	}
}
