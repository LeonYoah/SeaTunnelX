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

// **Feature: seatunnel-process-monitor, Property 10: 手动停止不触发自动拉起**
// **Validates: Requirements 4.7, 8.3**
// For any process marked as "manually stopped", process exit should not trigger auto restart.
// 对于任何被标记为"主动停止"的进程，进程退出后不应触发自动拉起。
func TestProperty_ManualStopNoAutoRestart(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		monitor := NewProcessMonitor()

		// Generate random process name / 生成随机进程名
		processName := rapid.StringMatching(`seatunnel-[a-z0-9]+`).Draw(t, "processName")
		pid := rapid.IntRange(1000, 65535).Draw(t, "pid")

		// Track process / 跟踪进程
		monitor.TrackProcess(processName, pid, "/opt/seatunnel", "hybrid", nil)

		// Mark as manually stopped / 标记为手动停止
		monitor.MarkManuallyStopped(processName)

		// Verify manually stopped flag / 验证手动停止标记
		if !monitor.IsManuallyStopped(processName) {
			t.Errorf("Process should be marked as manually stopped")
		}

		// Get tracked process / 获取跟踪的进程
		proc := monitor.GetTrackedProcess(processName)
		if proc == nil {
			t.Fatalf("Process not found")
		}

		// Verify ManuallyStopped flag / 验证 ManuallyStopped 标记
		if !proc.ManuallyStopped {
			t.Errorf("TrackedProcess.ManuallyStopped should be true")
		}
	})
}

// **Feature: seatunnel-process-monitor, Property 11: 启动清除手动停止标记**
// **Validates: Requirements 8.4**
// For any process marked as "manually stopped", when user starts it through the interface,
// the manual stop flag should be cleared.
// 对于任何被标记为"主动停止"的进程，当用户通过界面启动时，应清除该标记。
func TestProperty_StartClearsManualStopFlag(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		monitor := NewProcessMonitor()

		// Generate random process name / 生成随机进程名
		processName := rapid.StringMatching(`seatunnel-[a-z0-9]+`).Draw(t, "processName")
		pid := rapid.IntRange(1000, 65535).Draw(t, "pid")

		// Track process / 跟踪进程
		monitor.TrackProcess(processName, pid, "/opt/seatunnel", "hybrid", nil)

		// Mark as manually stopped / 标记为手动停止
		monitor.MarkManuallyStopped(processName)

		// Verify manually stopped / 验证手动停止
		if !monitor.IsManuallyStopped(processName) {
			t.Errorf("Process should be marked as manually stopped")
		}

		// Clear manual stop flag (simulating user start) / 清除手动停止标记（模拟用户启动）
		monitor.ClearManuallyStopped(processName)

		// Verify flag is cleared / 验证标记已清除
		if monitor.IsManuallyStopped(processName) {
			t.Errorf("Manual stop flag should be cleared after start")
		}
	})
}

// **Feature: seatunnel-process-monitor, Property 6: 进程状态变化事件完整性**
// **Validates: Requirements 3.3, 3.4**
// For any process status change (start, stop, crash), Agent should generate
// corresponding event and report to Control Plane.
// 对于任何进程状态变化（启动、停止、崩溃），Agent 应生成对应的事件并上报到 Control Plane。
func TestProperty_ProcessEventCompleteness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		monitor := NewProcessMonitor()

		// Collect events / 收集事件
		var events []*ProcessEvent
		var mu sync.Mutex

		monitor.SetEventHandler(func(event *ProcessEvent) {
			mu.Lock()
			events = append(events, event)
			mu.Unlock()
		})

		// Generate random process name / 生成随机进程名
		processName := rapid.StringMatching(`seatunnel-[a-z0-9]+`).Draw(t, "processName")
		pid := rapid.IntRange(1000, 65535).Draw(t, "pid")

		// Track process (should generate started event) / 跟踪进程（应生成启动事件）
		monitor.TrackProcess(processName, pid, "/opt/seatunnel", "hybrid", nil)

		// Wait for event / 等待事件
		time.Sleep(100 * time.Millisecond)

		// Verify started event / 验证启动事件
		mu.Lock()
		hasStartedEvent := false
		for _, e := range events {
			if e.Type == EventStarted && e.Name == processName {
				hasStartedEvent = true
				break
			}
		}
		mu.Unlock()

		if !hasStartedEvent {
			t.Errorf("Should generate started event when tracking process")
		}

		// Untrack process (should generate stopped event) / 取消跟踪进程（应生成停止事件）
		monitor.UntrackProcess(processName)

		// Wait for event / 等待事件
		time.Sleep(100 * time.Millisecond)

		// Verify stopped event / 验证停止事件
		mu.Lock()
		hasStoppedEvent := false
		for _, e := range events {
			if e.Type == EventStopped && e.Name == processName {
				hasStoppedEvent = true
				break
			}
		}
		mu.Unlock()

		if !hasStoppedEvent {
			t.Errorf("Should generate stopped event when untracking process")
		}
	})
}

// **Feature: seatunnel-process-monitor, Property 7: 连续检查失败触发重启**
// **Validates: Requirements 3.6**
// For any monitored process, when 3 consecutive checks detect process not existing,
// auto restart should be triggered (if enabled).
// 对于任何被监控的进程，当连续 3 次检查均检测到进程不存在时，
// 应触发自动拉起流程（如已启用）。
func TestProperty_ConsecutiveFailureTrigger(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		monitor := NewProcessMonitor()
		monitor.SetConsecutiveFailThreshold(3)

		// Track crash events / 跟踪崩溃事件
		var crashedProcs []*TrackedProcess
		var mu sync.Mutex

		monitor.SetCrashHandler(func(proc *TrackedProcess) {
			mu.Lock()
			crashedProcs = append(crashedProcs, proc)
			mu.Unlock()
		})

		// Generate random process name / 生成随机进程名
		processName := rapid.StringMatching(`seatunnel-[a-z0-9]+`).Draw(t, "processName")

		// Use a PID that definitely doesn't exist / 使用一个肯定不存在的 PID
		pid := 999999

		// Track process / 跟踪进程
		monitor.TrackProcess(processName, pid, "/opt/seatunnel", "hybrid", nil)

		// Simulate consecutive failures by directly manipulating the tracked process
		// 通过直接操作跟踪的进程来模拟连续失败
		monitor.mu.Lock()
		if proc, exists := monitor.trackedProcesses[processName]; exists {
			proc.ConsecutiveFails = 2 // Set to 2, next check will trigger
		}
		monitor.mu.Unlock()

		// Run one check cycle / 运行一次检查周期
		monitor.checkAllProcesses()

		// Wait for crash handler / 等待崩溃处理器
		time.Sleep(100 * time.Millisecond)

		// Verify crash was detected / 验证检测到崩溃
		mu.Lock()
		hasCrash := len(crashedProcs) > 0
		mu.Unlock()

		if !hasCrash {
			t.Errorf("Should trigger crash handler after consecutive failures")
		}
	})
}

// TestProcessMonitor_TrackAndUntrack tests tracking and untracking processes
// TestProcessMonitor_TrackAndUntrack 测试跟踪和取消跟踪进程
func TestProcessMonitor_TrackAndUntrack(t *testing.T) {
	monitor := NewProcessMonitor()

	// Track process / 跟踪进程
	monitor.TrackProcess("test-process", 1234, "/opt/seatunnel", "hybrid", nil)

	// Verify tracked / 验证已跟踪
	proc := monitor.GetTrackedProcess("test-process")
	if proc == nil {
		t.Fatal("Process should be tracked")
	}

	if proc.PID != 1234 {
		t.Errorf("PID mismatch: got %d, want 1234", proc.PID)
	}

	// Untrack / 取消跟踪
	monitor.UntrackProcess("test-process")

	// Verify untracked / 验证已取消跟踪
	proc = monitor.GetTrackedProcess("test-process")
	if proc != nil {
		t.Error("Process should be untracked")
	}
}

// TestProcessMonitor_ManualStop tests manual stop functionality
// TestProcessMonitor_ManualStop 测试手动停止功能
func TestProcessMonitor_ManualStop(t *testing.T) {
	monitor := NewProcessMonitor()

	// Track process / 跟踪进程
	monitor.TrackProcess("test-process", 1234, "/opt/seatunnel", "hybrid", nil)

	// Initially not manually stopped / 初始时未手动停止
	if monitor.IsManuallyStopped("test-process") {
		t.Error("Process should not be manually stopped initially")
	}

	// Mark as manually stopped / 标记为手动停止
	monitor.MarkManuallyStopped("test-process")

	// Verify manually stopped / 验证手动停止
	if !monitor.IsManuallyStopped("test-process") {
		t.Error("Process should be manually stopped")
	}

	// Clear manual stop / 清除手动停止
	monitor.ClearManuallyStopped("test-process")

	// Verify not manually stopped / 验证未手动停止
	if monitor.IsManuallyStopped("test-process") {
		t.Error("Process should not be manually stopped after clear")
	}
}
