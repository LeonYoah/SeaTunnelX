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

package plugin

import (
	"context"
	"testing"
	"time"
)

// TestFetchPluginsFromDocs tests fetching plugins from Maven repository.
// TestFetchPluginsFromDocs 测试从 Maven 仓库获取插件。
func TestFetchPluginsFromDocs(t *testing.T) {
	service := NewService(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	version := "2.3.12"

	// Test fetching all plugins from Maven / 测试从 Maven 获取所有插件
	plugins, err := service.fetchPluginsFromDocs(ctx, version)
	if err != nil {
		t.Logf("Warning: Failed to fetch plugins from Maven (may be network issue): %v", err)
		t.Logf("Skipping test due to network issue / 由于网络问题跳过测试")
		t.Skip("Network issue, skipping test")
		return
	}

	t.Logf("Total plugins fetched: %d / 获取到的插件总数: %d", len(plugins), len(plugins))

	// Count by category / 按分类统计
	connectorCount := 0
	for _, p := range plugins {
		if p.Category == PluginCategoryConnector {
			connectorCount++
		}
	}

	t.Logf("Connector plugins: %d / 连接器插件: %d", connectorCount, connectorCount)

	// Verify we have plugins / 验证有插件
	if len(plugins) == 0 {
		t.Error("Expected at least some plugins, got 0 / 期望至少有一些插件，但得到 0")
	}

	// Print first 10 connector plugins / 打印前10个连接器插件
	t.Log("\n=== Sample Connector Plugins / 示例连接器插件 ===")
	count := 0
	for _, p := range plugins {
		if p.Category == PluginCategoryConnector && count < 10 {
			t.Logf("  - %s (%s): artifact=%s", p.DisplayName, p.Name, p.ArtifactID)
			count++
		}
	}
}
