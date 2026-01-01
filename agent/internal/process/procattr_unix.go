//go:build !windows
// +build !windows

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

package process

import (
	"os/exec"
	"syscall"
)

// setProcGroupAttr sets process group attributes for Unix systems
// setProcGroupAttr 为 Unix 系统设置进程组属性
// This ensures child processes are in a separate process group
// 这确保子进程在单独的进程组中
// So when Agent is killed, SeaTunnel processes won't be affected
// 这样当 Agent 被杀死时，SeaTunnel 进程不会受影响
func setProcGroupAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group / 创建新进程组
	}
}
