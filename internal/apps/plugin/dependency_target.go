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
	"fmt"
	"path"
	"strconv"
	"strings"
)

const connectorIsolatedDependencySince = "2.3.12"

func supportsConnectorIsolatedDependency(version string) bool {
	version = strings.TrimSpace(version)
	if version == "" {
		return false
	}
	return comparePluginVersions(version, connectorIsolatedDependencySince) >= 0
}

func defaultPluginDependencyTargetDir(version, artifactID string) string {
	artifactID = strings.TrimSpace(artifactID)
	if artifactID == "" {
		return "lib"
	}
	if supportsConnectorIsolatedDependency(version) {
		return path.Join("plugins", artifactID)
	}
	return "lib"
}

func normalizePluginTargetDir(targetDir string) (string, error) {
	targetDir = strings.TrimSpace(targetDir)
	if targetDir == "" {
		return "", fmt.Errorf("target_dir is required")
	}
	cleaned := path.Clean(strings.ReplaceAll(targetDir, "\\", "/"))
	if cleaned == "." || cleaned == "/" || strings.HasPrefix(cleaned, "../") || cleaned == ".." || path.IsAbs(cleaned) {
		return "", fmt.Errorf("invalid target_dir: %s", targetDir)
	}
	return cleaned, nil
}

func comparePluginVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}
	for i := 0; i < maxLen; i++ {
		var p1, p2 string
		if i < len(parts1) {
			p1 = parts1[i]
		}
		if i < len(parts2) {
			p2 = parts2[i]
		}
		n1, s1 := parsePluginVersionPart(p1)
		n2, s2 := parsePluginVersionPart(p2)
		if n1 != n2 {
			return n1 - n2
		}
		if s1 != s2 {
			if s1 == "" {
				return 1
			}
			if s2 == "" {
				return -1
			}
			return strings.Compare(s1, s2)
		}
	}
	return 0
}

func parsePluginVersionPart(part string) (int, string) {
	if part == "" {
		return 0, ""
	}
	idx := strings.Index(part, "-")
	if idx == -1 {
		num, _ := strconv.Atoi(part)
		return num, ""
	}
	num, _ := strconv.Atoi(part[:idx])
	return num, part[idx:]
}
