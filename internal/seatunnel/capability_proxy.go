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
	// DefaultSeatunnelXJavaProxyVersion is the fallback seatunnelx-java-proxy implementation
	// shipped with SeaTunnelX until newer SeaTunnel-specific probe jars are added.
	// DefaultSeatunnelXJavaProxyVersion 是 SeaTunnelX 当前内置的 seatunnelx-java-proxy 回退版本。
	DefaultSeatunnelXJavaProxyVersion = "2.3.13"

	// SeatunnelXJavaProxyJarFileNamePattern defines the packaged jar naming convention.
	// SeatunnelXJavaProxyJarFileNamePattern 定义 seatunnelx-java-proxy jar 的统一命名规则。
	SeatunnelXJavaProxyJarFileNamePattern = "seatunnelx-java-proxy-%s.jar"

	// SeatunnelXJavaProxyScriptFileName is the shared launcher script name.
	// SeatunnelXJavaProxyScriptFileName 是统一的 seatunnelx-java-proxy 启动脚本名。
	SeatunnelXJavaProxyScriptFileName = "seatunnelx-java-proxy.sh"
)

// ResolveSeatunnelXJavaProxyVersion falls back to the packaged default when no
// SeaTunnel-specific seatunnelx-java-proxy jar version is provided.
// ResolveSeatunnelXJavaProxyVersion 在未指定版本时回退到内置默认 seatunnelx-java-proxy 版本。
func ResolveSeatunnelXJavaProxyVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed != "" {
		return trimmed
	}
	return DefaultSeatunnelXJavaProxyVersion
}

// SeatunnelXJavaProxyJarFileName returns the packaged seatunnelx-java-proxy jar file name
// for a SeaTunnel version.
// SeatunnelXJavaProxyJarFileName 返回指定 SeaTunnel 版本对应的 seatunnelx-java-proxy jar 文件名。
func SeatunnelXJavaProxyJarFileName(version string) string {
	return fmt.Sprintf(SeatunnelXJavaProxyJarFileNamePattern, ResolveSeatunnelXJavaProxyVersion(version))
}
