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

import "errors"

// Error definitions for monitor package.
// 监控包的错误定义。
var (
	// ErrConfigNotFound indicates the monitor config was not found.
	// ErrConfigNotFound 表示监控配置未找到。
	ErrConfigNotFound = errors.New("monitor config not found / 监控配置未找到")

	// ErrInvalidMonitorInterval indicates the monitor interval is invalid.
	// ErrInvalidMonitorInterval 表示监控间隔无效。
	ErrInvalidMonitorInterval = errors.New("monitor interval must be between 1 and 60 seconds / 监控间隔必须在 1-60 秒之间")

	// ErrInvalidRestartDelay indicates the restart delay is invalid.
	// ErrInvalidRestartDelay 表示重启延迟无效。
	ErrInvalidRestartDelay = errors.New("restart delay must be between 1 and 300 seconds / 重启延迟必须在 1-300 秒之间")

	// ErrInvalidMaxRestarts indicates the max restarts is invalid.
	// ErrInvalidMaxRestarts 表示最大重启次数无效。
	ErrInvalidMaxRestarts = errors.New("max restarts must be between 1 and 10 / 最大重启次数必须在 1-10 之间")

	// ErrInvalidTimeWindow indicates the time window is invalid.
	// ErrInvalidTimeWindow 表示时间窗口无效。
	ErrInvalidTimeWindow = errors.New("time window must be between 60 and 3600 seconds / 时间窗口必须在 60-3600 秒之间")

	// ErrInvalidCooldownPeriod indicates the cooldown period is invalid.
	// ErrInvalidCooldownPeriod 表示冷却时间无效。
	ErrInvalidCooldownPeriod = errors.New("cooldown period must be between 60 and 86400 seconds / 冷却时间必须在 60-86400 秒之间")

	// ErrEventNotFound indicates the process event was not found.
	// ErrEventNotFound 表示进程事件未找到。
	ErrEventNotFound = errors.New("process event not found / 进程事件未找到")
)
