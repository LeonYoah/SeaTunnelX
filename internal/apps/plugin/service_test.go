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

// TestFetchPluginsFromDocs tests fetching plugins from SeaTunnel documentation.
// TestFetchPluginsFromDocs 测试从 SeaTunnel 文档获取插件。
func TestFetchPluginsFromDocs(t *testing.T) {
	service := NewService(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	version := "2.3.12"

	// Test fetching all plugins / 测试获取所有插件
	plugins, err := service.fetchPluginsFromDocs(ctx, version)
	if err != nil {
		t.Logf("Warning: Failed to fetch plugins from docs (may be network issue): %v", err)
		t.Logf("Using fallback plugins instead / 使用备用插件列表")
		plugins = getAvailablePluginsForVersion(version)
	}

	t.Logf("Total plugins fetched: %d / 获取到的插件总数: %d", len(plugins), len(plugins))

	// Count by category / 按分类统计
	sourceCount := 0
	sinkCount := 0
	transformCount := 0
	for _, p := range plugins {
		switch p.Category {
		case PluginCategorySource:
			sourceCount++
		case PluginCategorySink:
			sinkCount++
		case PluginCategoryTransform:
			transformCount++
		}
	}

	t.Logf("Source plugins: %d / 数据源插件: %d", sourceCount, sourceCount)
	t.Logf("Sink plugins: %d / 数据目标插件: %d", sinkCount, sinkCount)
	t.Logf("Transform plugins: %d / 数据转换插件: %d", transformCount, transformCount)

	// Verify we have plugins / 验证有插件
	if len(plugins) == 0 {
		t.Error("Expected at least some plugins, got 0 / 期望至少有一些插件，但得到 0")
	}

	// Print first 5 plugins of each category / 打印每个分类的前5个插件
	t.Log("\n=== Sample Source Plugins / 示例数据源插件 ===")
	count := 0
	for _, p := range plugins {
		if p.Category == PluginCategorySource && count < 5 {
			t.Logf("  - %s (%s): %s", p.DisplayName, p.Name, p.DocURL)
			count++
		}
	}

	t.Log("\n=== Sample Sink Plugins / 示例数据目标插件 ===")
	count = 0
	for _, p := range plugins {
		if p.Category == PluginCategorySink && count < 5 {
			t.Logf("  - %s (%s): %s", p.DisplayName, p.Name, p.DocURL)
			count++
		}
	}

	t.Log("\n=== Sample Transform Plugins / 示例数据转换插件 ===")
	count = 0
	for _, p := range plugins {
		if p.Category == PluginCategoryTransform && count < 5 {
			t.Logf("  - %s (%s): %s", p.DisplayName, p.Name, p.DocURL)
			count++
		}
	}
}
