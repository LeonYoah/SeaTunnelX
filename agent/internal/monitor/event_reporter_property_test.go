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

package monitor

import (
	"sync"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// **Feature: seatunnel-process-monitor, Property 16: 事件缓存与重传**
// **Validates: Requirements 7.6**
// For any process events generated during communication failure,
// Agent should cache these events and batch report when connection is restored.
// 对于通信异常期间产生的进程事件，Agent 应缓存这些事件，待连接恢复后批量上报。
func TestProperty_EventCacheAndRetransmit(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random event count / 生成随机事件数量
		eventCount := rapid.IntRange(1, 20).Draw(t, "eventCount")

		var reportedEvents []*ProcessEvent
		var mu sync.Mutex

		reporter := NewEventReporter(func(events []*ProcessEvent) error {
			mu.Lock()
			reportedEvents = append(reportedEvents, events...)
			mu.Unlock()
			return nil
		})
		reporter.SetBatchSize(100) // Large batch size to report all at once / 大批量大小以一次性上报

		// Simulate disconnected state / 模拟断开连接状态
		reporter.SetConnected(false)

		// Generate and cache events / 生成并缓存事件
		for i := 0; i < eventCount; i++ {
			event := &ProcessEvent{
				Type:      EventCrashed,
				PID:       rapid.IntRange(1000, 65535).Draw(t, "pid"),
				Name:      rapid.StringMatching(`seatunnel-[a-z0-9]+`).Draw(t, "name"),
				Timestamp: time.Now(),
			}
			reporter.ReportEvent(event)
		}

		// Verify events are cached / 验证事件已缓存
		cachedCount := reporter.GetCachedEventCount()
		if cachedCount != eventCount {
			t.Errorf("Expected %d cached events, got %d", eventCount, cachedCount)
		}

		// Simulate reconnection / 模拟重新连接
		reporter.SetConnected(true)

		// Wait for flush / 等待刷新
		time.Sleep(100 * time.Millisecond)

		// Verify all events were reported / 验证所有事件已上报
		mu.Lock()
		reportedCount := len(reportedEvents)
		mu.Unlock()

		if reportedCount != eventCount {
			t.Errorf("Expected %d reported events, got %d", eventCount, reportedCount)
		}

		// Verify cache is empty / 验证缓存为空
		if reporter.GetCachedEventCount() != 0 {
			t.Errorf("Cache should be empty after flush")
		}
	})
}

// TestEventReporter_CacheLimit tests cache size limit
// TestEventReporter_CacheLimit 测试缓存大小限制
func TestEventReporter_CacheLimit(t *testing.T) {
	reporter := NewEventReporter(nil)
	reporter.SetCacheSize(5)
	reporter.SetConnected(false) // Prevent immediate reporting / 防止立即上报

	// Add more events than cache size / 添加超过缓存大小的事件
	for i := 0; i < 10; i++ {
		event := &ProcessEvent{
			Type:      EventCrashed,
			PID:       i,
			Name:      "test",
			Timestamp: time.Now(),
		}
		reporter.ReportEvent(event)
	}

	// Verify cache is limited / 验证缓存受限
	if reporter.GetCachedEventCount() > 5 {
		t.Errorf("Cache should be limited to 5, got %d", reporter.GetCachedEventCount())
	}
}

// TestEventReporter_BatchReporting tests batch reporting
// TestEventReporter_BatchReporting 测试批量上报
func TestEventReporter_BatchReporting(t *testing.T) {
	var batches []int
	var mu sync.Mutex

	reporter := NewEventReporter(func(events []*ProcessEvent) error {
		mu.Lock()
		batches = append(batches, len(events))
		mu.Unlock()
		return nil
	})
	reporter.SetBatchSize(3)
	reporter.SetConnected(true)

	// Add events / 添加事件
	for i := 0; i < 10; i++ {
		event := &ProcessEvent{
			Type:      EventCrashed,
			PID:       i,
			Name:      "test",
			Timestamp: time.Now(),
		}
		reporter.ReportEvent(event)
	}

	// Flush remaining / 刷新剩余
	reporter.FlushEvents()

	// Wait for async operations / 等待异步操作
	time.Sleep(100 * time.Millisecond)

	// Verify batches / 验证批次
	mu.Lock()
	totalReported := 0
	for _, batchSize := range batches {
		totalReported += batchSize
	}
	mu.Unlock()

	if totalReported != 10 {
		t.Errorf("Expected 10 total reported events, got %d", totalReported)
	}
}

// TestEventReporter_ClearCache tests cache clearing
// TestEventReporter_ClearCache 测试清除缓存
func TestEventReporter_ClearCache(t *testing.T) {
	reporter := NewEventReporter(nil)
	reporter.SetConnected(false)

	// Add events / 添加事件
	for i := 0; i < 5; i++ {
		event := &ProcessEvent{
			Type:      EventCrashed,
			PID:       i,
			Name:      "test",
			Timestamp: time.Now(),
		}
		reporter.ReportEvent(event)
	}

	// Verify events cached / 验证事件已缓存
	if reporter.GetCachedEventCount() != 5 {
		t.Errorf("Expected 5 cached events, got %d", reporter.GetCachedEventCount())
	}

	// Clear cache / 清除缓存
	reporter.ClearCache()

	// Verify cache is empty / 验证缓存为空
	if reporter.GetCachedEventCount() != 0 {
		t.Errorf("Cache should be empty after clear")
	}
}
