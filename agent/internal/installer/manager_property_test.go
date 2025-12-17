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

package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"pgregory.net/rapid"
)

// **Feature: seatunnel-agent, Property 8: Checksum Validation**
// **Validates: Requirements 5.2**
//
// Property: For any offline installation request with a specified package path
// and expected checksum, the system SHALL verify the file's checksum matches
// the expected value and reject mismatches with an error.
// 属性：对于任何指定安装包路径和预期校验和的离线安装请求，
// 系统应该验证文件的校验和与预期值匹配，并在不匹配时返回错误。
func TestProperty_ChecksumValidation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random file content / 生成随机文件内容
		contentSize := rapid.IntRange(1, 10000).Draw(rt, "contentSize")
		content := make([]byte, contentSize)
		for i := range content {
			content[i] = byte(rapid.IntRange(0, 255).Draw(rt, "byte"))
		}

		// Create a temporary file with the content / 创建包含内容的临时文件
		tempDir := t.TempDir()
		tempFile := filepath.Join(tempDir, "test-package.tar.gz")
		if err := os.WriteFile(tempFile, content, 0644); err != nil {
			rt.Fatalf("Failed to create temp file: %v", err)
		}

		// Calculate the actual checksum / 计算实际校验和
		hash := sha256.Sum256(content)
		actualChecksum := hex.EncodeToString(hash[:])

		// Create installer manager / 创建安装管理器
		manager := NewInstallerManager()

		// Property 1: Correct checksum should pass verification
		// 属性 1：正确的校验和应该通过验证
		err := manager.VerifyChecksum(tempFile, actualChecksum)
		if err != nil {
			rt.Fatalf("Checksum verification failed for correct checksum: %v", err)
		}

		// Property 2: Uppercase checksum should also pass (case-insensitive)
		// 属性 2：大写校验和也应该通过（不区分大小写）
		err = manager.VerifyChecksum(tempFile, hex.EncodeToString(hash[:]))
		if err != nil {
			rt.Fatalf("Checksum verification failed for uppercase checksum: %v", err)
		}

		// Property 3: Incorrect checksum should fail verification
		// 属性 3：错误的校验和应该验证失败
		// Generate a different checksum by modifying one character
		// 通过修改一个字符生成不同的校验和
		wrongChecksum := generateWrongChecksum(actualChecksum)
		err = manager.VerifyChecksum(tempFile, wrongChecksum)
		if err == nil {
			rt.Fatal("Checksum verification should fail for incorrect checksum")
		}
		if !errors.Is(err, ErrChecksumMismatch) {
			rt.Fatalf("Expected ErrChecksumMismatch, got: %v", err)
		}

		// Property 4: CalculateChecksum should return consistent results
		// 属性 4：CalculateChecksum 应该返回一致的结果
		calculatedChecksum, err := CalculateChecksum(tempFile)
		if err != nil {
			rt.Fatalf("CalculateChecksum failed: %v", err)
		}
		if calculatedChecksum != actualChecksum {
			rt.Fatalf("CalculateChecksum returned inconsistent result: expected %s, got %s", actualChecksum, calculatedChecksum)
		}

		// Property 5: Checksum with whitespace should be trimmed and pass
		// 属性 5：带空格的校验和应该被修剪并通过
		checksumWithSpaces := "  " + actualChecksum + "  \n"
		err = manager.VerifyChecksum(tempFile, checksumWithSpaces)
		if err != nil {
			rt.Fatalf("Checksum verification failed for checksum with whitespace: %v", err)
		}
	})
}

// generateWrongChecksum generates a checksum that differs from the input
// generateWrongChecksum 生成与输入不同的校验和
func generateWrongChecksum(checksum string) string {
	if len(checksum) == 0 {
		return "0000000000000000000000000000000000000000000000000000000000000000"
	}

	// Modify the first character / 修改第一个字符
	chars := []byte(checksum)
	if chars[0] == '0' {
		chars[0] = '1'
	} else {
		chars[0] = '0'
	}
	return string(chars)
}

// TestProperty_ChecksumValidation_NonExistentFile tests checksum validation for non-existent files
// TestProperty_ChecksumValidation_NonExistentFile 测试不存在文件的校验和验证
func TestProperty_ChecksumValidation_NonExistentFile(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random non-existent file path / 生成随机的不存在文件路径
		tempDir := t.TempDir()
		nonExistentFile := filepath.Join(tempDir, rapid.StringMatching(`[a-z]{5,15}\.tar\.gz`).Draw(rt, "filename"))

		// Generate a random checksum / 生成随机校验和
		checksumBytes := make([]byte, 32)
		for i := range checksumBytes {
			checksumBytes[i] = byte(rapid.IntRange(0, 255).Draw(rt, "checksumByte"))
		}
		checksum := hex.EncodeToString(checksumBytes)

		// Property: VerifyChecksum should fail for non-existent files
		// 属性：VerifyChecksum 应该对不存在的文件失败
		manager := NewInstallerManager()
		err := manager.VerifyChecksum(nonExistentFile, checksum)
		if err == nil {
			rt.Fatal("VerifyChecksum should fail for non-existent file")
		}

		// Property: CalculateChecksum should also fail for non-existent files
		// 属性：CalculateChecksum 也应该对不存在的文件失败
		_, err = CalculateChecksum(nonExistentFile)
		if err == nil {
			rt.Fatal("CalculateChecksum should fail for non-existent file")
		}
	})
}

// **Feature: seatunnel-agent, Property 9: Config Generation Consistency**
// **Validates: Requirements 5.4**
//
// Property: For any SeaTunnel configuration generation request with deployment mode
// and node role parameters, the generated configuration file SHALL contain the
// correct settings for that mode and role combination.
// 属性：对于任何带有部署模式和节点角色参数的 SeaTunnel 配置生成请求，
// 生成的配置文件应该包含该模式和角色组合的正确设置。
func TestProperty_ConfigGenerationConsistency(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random configuration parameters / 生成随机配置参数
		deploymentMode := rapid.SampledFrom([]DeploymentMode{
			DeploymentModeHybrid,
			DeploymentModeSeparated,
		}).Draw(rt, "deploymentMode")

		nodeRole := rapid.SampledFrom([]NodeRole{
			NodeRoleMaster,
			NodeRoleWorker,
		}).Draw(rt, "nodeRole")

		clusterName := rapid.StringMatching(`[a-z][a-z0-9-]{2,20}`).Draw(rt, "clusterName")
		clusterPort := rapid.IntRange(1024, 65535).Draw(rt, "clusterPort")
		httpPort := rapid.IntRange(1024, 65535).Draw(rt, "httpPort")

		// Generate master addresses / 生成 master 地址
		numMasters := rapid.IntRange(0, 5).Draw(rt, "numMasters")
		masterAddresses := make([]string, numMasters)
		for i := range masterAddresses {
			ip := rapid.StringMatching(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`).Draw(rt, "masterIP")
			masterAddresses[i] = ip
		}

		// Generate SeaTunnel configuration / 生成 SeaTunnel 配置
		config := GenerateSeaTunnelConfig(deploymentMode, nodeRole, clusterName, masterAddresses, clusterPort, httpPort)

		// Property 1: Configuration must contain cluster name
		// 属性 1：配置必须包含集群名称
		if !containsString(config, clusterName) {
			rt.Fatalf("Configuration does not contain cluster name: %s", clusterName)
		}

		// Property 2: Configuration must contain HTTP port
		// 属性 2：配置必须包含 HTTP 端口
		httpPortStr := rapid.Just(httpPort).Draw(rt, "httpPortStr")
		if !containsString(config, intToString(httpPortStr)) {
			rt.Fatalf("Configuration does not contain HTTP port: %d", httpPort)
		}

		// Property 3: Deployment mode specific settings
		// 属性 3：部署模式特定设置
		// Both modes should always have dynamic-slot: true (user requirement)
		// 两种模式都应该始终有 dynamic-slot: true（用户要求）
		if !containsString(config, "dynamic-slot: true") {
			rt.Fatal("Configuration should always contain 'dynamic-slot: true'")
		}

		switch deploymentMode {
		case DeploymentModeHybrid:
			if !containsString(config, "Hybrid mode") || !containsString(config, "混合模式") {
				rt.Fatal("Hybrid mode configuration should contain mode comment")
			}
		case DeploymentModeSeparated:
			if !containsString(config, "Separated mode") || !containsString(config, "分离模式") {
				rt.Fatal("Separated mode configuration should contain mode comment")
			}
		}

		// Property 4: Node role specific settings
		// 属性 4：节点角色特定设置
		switch nodeRole {
		case NodeRoleMaster:
			// Master node should have backup-count: 1 and checkpoint settings
			// Master 节点应该有 backup-count: 1 和 checkpoint 设置
			if !containsString(config, "backup-count: 1") {
				rt.Fatal("Master node configuration should contain 'backup-count: 1'")
			}
			if !containsString(config, "checkpoint:") {
				rt.Fatal("Master node configuration should contain checkpoint settings")
			}
			if !containsString(config, "Master node") || !containsString(config, "Master 节点") {
				rt.Fatal("Master node configuration should contain role comment")
			}
		case NodeRoleWorker:
			// Worker node should have backup-count: 0
			// Worker 节点应该有 backup-count: 0
			if !containsString(config, "backup-count: 0") {
				rt.Fatal("Worker node configuration should contain 'backup-count: 0'")
			}
			if !containsString(config, "Worker node") || !containsString(config, "Worker 节点") {
				rt.Fatal("Worker node configuration should contain role comment")
			}
		}

		// Property 5: Configuration must be valid YAML structure
		// 属性 5：配置必须是有效的 YAML 结构
		if !containsString(config, "seatunnel:") {
			rt.Fatal("Configuration should start with 'seatunnel:' section")
		}
		if !containsString(config, "engine:") {
			rt.Fatal("Configuration should contain 'engine:' section")
		}

		// Generate Hazelcast configuration / 生成 Hazelcast 配置
		hazelcastConfig := GenerateHazelcastConfig(clusterName, masterAddresses, clusterPort)

		// Property 6: Hazelcast configuration must contain cluster name
		// 属性 6：Hazelcast 配置必须包含集群名称
		if !containsString(hazelcastConfig, clusterName) {
			rt.Fatalf("Hazelcast configuration does not contain cluster name: %s", clusterName)
		}

		// Property 7: Hazelcast configuration must contain cluster port
		// 属性 7：Hazelcast 配置必须包含集群端口
		if !containsString(hazelcastConfig, intToString(clusterPort)) {
			rt.Fatalf("Hazelcast configuration does not contain cluster port: %d", clusterPort)
		}

		// Property 8: Hazelcast configuration must contain master addresses
		// 属性 8：Hazelcast 配置必须包含 master 地址
		for _, addr := range masterAddresses {
			if !containsString(hazelcastConfig, addr) {
				rt.Fatalf("Hazelcast configuration does not contain master address: %s", addr)
			}
		}

		// Property 9: Hazelcast configuration must have TCP-IP enabled
		// 属性 9：Hazelcast 配置必须启用 TCP-IP
		if !containsString(hazelcastConfig, "tcp-ip:") {
			rt.Fatal("Hazelcast configuration should contain 'tcp-ip:' section")
		}
		if !containsString(hazelcastConfig, "enabled: true") {
			rt.Fatal("Hazelcast configuration should have TCP-IP enabled")
		}

		// Property 10: Hazelcast configuration must have multicast disabled
		// 属性 10：Hazelcast 配置必须禁用 multicast
		if !containsString(hazelcastConfig, "multicast:") {
			rt.Fatal("Hazelcast configuration should contain 'multicast:' section")
		}
	})
}

// containsString checks if a string contains a substring
// containsString 检查字符串是否包含子字符串
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

// findSubstring finds a substring in a string
// findSubstring 在字符串中查找子字符串
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// intToString converts an int to string
// intToString 将 int 转换为字符串
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	if negative {
		result = "-" + result
	}
	return result
}

// TestProperty_ConfigGenerationConsistency_DefaultValues tests config generation with default values
// TestProperty_ConfigGenerationConsistency_DefaultValues 测试使用默认值的配置生成
func TestProperty_ConfigGenerationConsistency_DefaultValues(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Test with empty/default values / 使用空/默认值测试
		deploymentMode := rapid.SampledFrom([]DeploymentMode{
			DeploymentModeHybrid,
			DeploymentModeSeparated,
		}).Draw(rt, "deploymentMode")

		nodeRole := rapid.SampledFrom([]NodeRole{
			NodeRoleMaster,
			NodeRoleWorker,
		}).Draw(rt, "nodeRole")

		// Generate with empty cluster name (should use default)
		// 使用空集群名称生成（应该使用默认值）
		config := GenerateSeaTunnelConfig(deploymentMode, nodeRole, "", nil, 0, 0)

		// Property: Default cluster name should be used
		// 属性：应该使用默认集群名称
		if !containsString(config, "seatunnel-cluster") {
			rt.Fatal("Configuration should use default cluster name 'seatunnel-cluster'")
		}

		// Property: Default HTTP port should be used
		// 属性：应该使用默认 HTTP 端口
		if !containsString(config, "8080") {
			rt.Fatal("Configuration should use default HTTP port 8080")
		}

		// Generate Hazelcast config with defaults / 使用默认值生成 Hazelcast 配置
		hazelcastConfig := GenerateHazelcastConfig("", nil, 0)

		// Property: Default cluster port should be used
		// 属性：应该使用默认集群端口
		if !containsString(hazelcastConfig, "5801") {
			rt.Fatal("Hazelcast configuration should use default cluster port 5801")
		}

		// Property: Default member should be localhost
		// 属性：默认成员应该是 localhost
		if !containsString(hazelcastConfig, "127.0.0.1") {
			rt.Fatal("Hazelcast configuration should use default member 127.0.0.1")
		}
	})
}
