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

package seatunnel

import (
	"fmt"
	"strings"
)

const (
	// DefaultCapabilityProxyVersion is the fallback capability proxy implementation
	// shipped with SeaTunnelX until newer SeaTunnel-specific probe jars are added.
	// DefaultCapabilityProxyVersion 是 SeaTunnelX 当前内置的 capability proxy 回退版本。
	DefaultCapabilityProxyVersion = "2.3.13"

	// CapabilityProxyJarFileNamePattern defines the packaged jar naming convention.
	// CapabilityProxyJarFileNamePattern 定义 capability proxy jar 的统一命名规则。
	CapabilityProxyJarFileNamePattern = "seatunnel-capability-proxy-%s.jar"

	// CapabilityProxyScriptFileName is the shared launcher script name.
	// CapabilityProxyScriptFileName 是统一的 capability proxy 启动脚本名。
	CapabilityProxyScriptFileName = "seatunnel-capability-proxy.sh"
)

// ResolveCapabilityProxyVersion falls back to the packaged default when no
// SeaTunnel-specific capability proxy jar version is provided.
// ResolveCapabilityProxyVersion 在未指定版本时回退到内置默认 capability proxy 版本。
func ResolveCapabilityProxyVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed != "" {
		return trimmed
	}
	return DefaultCapabilityProxyVersion
}

// CapabilityProxyJarFileName returns the packaged capability proxy jar file name
// for a SeaTunnel version.
// CapabilityProxyJarFileName 返回指定 SeaTunnel 版本对应的 capability proxy jar 文件名。
func CapabilityProxyJarFileName(version string) string {
	return fmt.Sprintf(CapabilityProxyJarFileNamePattern, ResolveCapabilityProxyVersion(version))
}
