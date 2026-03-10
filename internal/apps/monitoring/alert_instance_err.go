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

package monitoring

import "errors"

var (
	// ErrAlertInstanceInvalidID indicates the alert instance identifier is invalid.
	// ErrAlertInstanceInvalidID 表示告警实例标识非法。
	ErrAlertInstanceInvalidID = errors.New("monitoring: invalid alert instance id")
	// ErrAlertInstanceNotFound indicates the alert instance does not exist.
	// ErrAlertInstanceNotFound 表示告警实例不存在。
	ErrAlertInstanceNotFound = errors.New("monitoring: alert instance not found")
	// ErrAlertInstanceAlreadyClosed indicates the alert incident is already closed.
	// ErrAlertInstanceAlreadyClosed 表示该告警事件已经关闭。
	ErrAlertInstanceAlreadyClosed = errors.New("monitoring: alert instance already closed")
	// ErrAlertInstanceNotClosable indicates the alert incident is not yet recoverd and cannot be closed.
	// ErrAlertInstanceNotClosable 表示告警尚未恢复，当前不可关闭。
	ErrAlertInstanceNotClosable = errors.New("monitoring: alert instance is not closable")
)
