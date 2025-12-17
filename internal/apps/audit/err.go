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

package audit

import "errors"

// Error definitions for audit and command log operations.
// 审计和命令日志操作的错误定义。
var (
	// ErrCommandLogNotFound indicates the requested command log does not exist.
	// ErrCommandLogNotFound 表示请求的命令日志不存在。
	ErrCommandLogNotFound = errors.New("audit: command log not found")
	// ErrCommandIDDuplicate indicates a command log with the same command ID already exists.
	// ErrCommandIDDuplicate 表示具有相同命令 ID 的命令日志已存在。
	ErrCommandIDDuplicate = errors.New("audit: command ID already exists")
	// ErrCommandIDEmpty indicates the command ID is empty.
	// ErrCommandIDEmpty 表示命令 ID 为空。
	ErrCommandIDEmpty = errors.New("audit: command ID cannot be empty")
	// ErrAgentIDEmpty indicates the agent ID is empty.
	// ErrAgentIDEmpty 表示代理 ID 为空。
	ErrAgentIDEmpty = errors.New("audit: agent ID cannot be empty")
	// ErrCommandTypeEmpty indicates the command type is empty.
	// ErrCommandTypeEmpty 表示命令类型为空。
	ErrCommandTypeEmpty = errors.New("audit: command type cannot be empty")
	// ErrAuditLogNotFound indicates the requested audit log does not exist.
	// ErrAuditLogNotFound 表示请求的审计日志不存在。
	ErrAuditLogNotFound = errors.New("audit: audit log not found")
	// ErrActionEmpty indicates the action is empty.
	// ErrActionEmpty 表示操作为空。
	ErrActionEmpty = errors.New("audit: action cannot be empty")
	// ErrResourceTypeEmpty indicates the resource type is empty.
	// ErrResourceTypeEmpty 表示资源类型为空。
	ErrResourceTypeEmpty = errors.New("audit: resource type cannot be empty")
)

// Error codes for audit and command log operations.
// 审计和命令日志操作的错误代码。
const (
	ErrCodeCommandLogNotFound = 4001
	ErrCodeCommandIDDuplicate = 4002
	ErrCodeCommandIDEmpty     = 4003
	ErrCodeAgentIDEmpty       = 4004
	ErrCodeCommandTypeEmpty   = 4005
	ErrCodeAuditLogNotFound   = 4006
	ErrCodeActionEmpty        = 4007
	ErrCodeResourceTypeEmpty  = 4008
)
