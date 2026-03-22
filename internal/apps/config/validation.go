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

package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidationError represents one user-correctable config validation failure.
type ValidationError struct {
	ConfigType ConfigType
	Message    string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func shouldValidateYAML(configType ConfigType) bool {
	switch configType {
	case ConfigTypeSeatunnel,
		ConfigTypeHazelcast,
		ConfigTypeHazelcastClient,
		ConfigTypeHazelcastMaster,
		ConfigTypeHazelcastWorker:
		return true
	default:
		return false
	}
}

func validateConfigContent(configType ConfigType, content string) error {
	if !shouldValidateYAML(configType) {
		return nil
	}

	_, _, err := parseAndValidateYAML(configType, content)
	return err
}

func normalizeConfigContent(configType ConfigType, content string) (string, error) {
	if !shouldValidateYAML(configType) {
		return content, nil
	}

	root, _, err := parseAndValidateYAML(configType, content)
	if err != nil {
		if isHazelcastConfigType(configType) {
			if repaired, ok := tryRepairHazelcastMapBlockIndentation(content); ok {
				root, _, err = parseAndValidateYAML(configType, repaired)
				if err == nil {
					content = repaired
				}
			}
		} else if configType == ConfigTypeSeatunnel {
			if repaired, ok := tryRepairSeatunnelCheckpointIndentation(content); ok {
				root, _, err = parseAndValidateYAML(configType, repaired)
				if err == nil {
					content = repaired
				}
			}
		}
		if err != nil {
			return "", err
		}
	}
	normalizeYAMLNodeStyles(root)

	normalized, err := yaml.Marshal(root)
	if err != nil {
		return "", &ValidationError{
			ConfigType: configType,
			Message:    fmt.Sprintf("Invalid YAML in %s: %v", configType, err),
		}
	}
	return string(normalized), nil
}

// tryRepairHazelcastMapBlockIndentation attempts one safe repair only:
// move a malformed `map:` block to the same indentation level as other hazelcast child sections.
// tryRepairHazelcastMapBlockIndentation 只尝试一种安全修复：
// 把错误缩进的 `map:` 配置块调整到 hazelcast 根下其他子节点的同级缩进。
func tryRepairHazelcastMapBlockIndentation(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	hazelcastIndex := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "hazelcast:" {
			hazelcastIndex = i
			break
		}
	}
	if hazelcastIndex < 0 {
		return "", false
	}

	rootIndent := indentationWidth(lines[hazelcastIndex])
	expectedChildIndent := -1
	for i := hazelcastIndex + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := indentationWidth(lines[i])
		if indent > rootIndent {
			expectedChildIndent = indent
			break
		}
	}
	if expectedChildIndent <= rootIndent {
		return "", false
	}

	mapIndex := -1
	mapIndent := -1
	for i := hazelcastIndex + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "map:" {
			indent := indentationWidth(lines[i])
			if indent != expectedChildIndent {
				mapIndex = i
				mapIndent = indent
			}
			break
		}
	}
	if mapIndex < 0 || mapIndent >= expectedChildIndent {
		return "", false
	}

	blockEnd := len(lines)
	for i := mapIndex + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		indent := indentationWidth(lines[i])
		if !strings.HasPrefix(trimmed, "#") && indent <= mapIndent {
			blockEnd = i
			break
		}
	}

	shift := expectedChildIndent - mapIndent
	if shift <= 0 {
		return "", false
	}

	repaired := append([]string(nil), lines...)
	for i := mapIndex; i < blockEnd; i++ {
		if strings.TrimSpace(repaired[i]) == "" {
			continue
		}
		repaired[i] = strings.Repeat(" ", shift) + repaired[i]
	}

	return strings.Join(repaired, "\n"), true
}

// tryRepairSeatunnelCheckpointIndentation repairs common indentation mistakes in
// seatunnel.yaml around engine/checkpoint/storage/plugin-config blocks.
// tryRepairSeatunnelCheckpointIndentation 修复 seatunnel.yaml 中
// engine/checkpoint/storage/plugin-config 配置块的常见缩进问题。
func tryRepairSeatunnelCheckpointIndentation(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	changed := false

	seatunnelIndex := findLineWithTrimmedValue(lines, 0, "seatunnel:")
	if seatunnelIndex < 0 {
		return "", false
	}
	if repairDirectChildBlockIndentation(lines, seatunnelIndex, "engine:") {
		changed = true
	}

	engineIndex := findDirectChildLine(lines, seatunnelIndex, "engine:")
	if engineIndex < 0 {
		if changed {
			return strings.Join(lines, "\n"), true
		}
		return "", false
	}
	if repairDirectChildBlockIndentation(lines, engineIndex, "checkpoint:") {
		changed = true
	}

	checkpointIndex := findDirectChildLine(lines, engineIndex, "checkpoint:")
	if checkpointIndex < 0 {
		if changed {
			return strings.Join(lines, "\n"), true
		}
		return "", false
	}
	if repairDirectChildBlockIndentation(lines, checkpointIndex, "storage:") {
		changed = true
	}

	storageIndex := findDirectChildLine(lines, checkpointIndex, "storage:")
	if storageIndex < 0 {
		if changed {
			return strings.Join(lines, "\n"), true
		}
		return "", false
	}
	if repairDirectChildBlockIndentation(lines, storageIndex, "plugin-config:") {
		changed = true
	}

	if !changed {
		return "", false
	}
	return strings.Join(lines, "\n"), true
}

func repairDirectChildBlockIndentation(lines []string, anchorIndex int, targetKey string) bool {
	if anchorIndex < 0 || anchorIndex >= len(lines) {
		return false
	}

	anchorIndent := indentationWidth(lines[anchorIndex])
	expectedChildIndent := -1
	logicalEnd := len(lines)

	for i := anchorIndex + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := indentationWidth(lines[i])
		if indent <= anchorIndent {
			logicalEnd = i
			break
		}
		if expectedChildIndent == -1 {
			expectedChildIndent = indent
		}
	}

	if expectedChildIndent == -1 {
		expectedChildIndent = anchorIndent + 2
	}

	targetIndex := -1
	targetIndent := -1
	for i := anchorIndex + 1; i < logicalEnd; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == targetKey {
			indent := indentationWidth(lines[i])
			if indent != expectedChildIndent {
				targetIndex = i
				targetIndent = indent
			}
			break
		}
	}

	if targetIndex < 0 {
		for i := anchorIndex + 1; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if trimmed == targetKey {
				targetIndex = i
				targetIndent = indentationWidth(lines[i])
				break
			}
		}
	}

	if targetIndex < 0 || targetIndent >= expectedChildIndent {
		return false
	}

	targetBlockEnd := len(lines)
	for i := targetIndex + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		indent := indentationWidth(lines[i])
		if !strings.HasPrefix(trimmed, "#") && indent <= targetIndent {
			targetBlockEnd = i
			break
		}
	}

	shift := expectedChildIndent - targetIndent
	if shift <= 0 {
		return false
	}

	for i := targetIndex; i < targetBlockEnd; i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		lines[i] = strings.Repeat(" ", shift) + lines[i]
	}
	return true
}

func findDirectChildLine(lines []string, anchorIndex int, targetKey string) int {
	if anchorIndex < 0 || anchorIndex >= len(lines) {
		return -1
	}
	anchorIndent := indentationWidth(lines[anchorIndex])
	for i := anchorIndex + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := indentationWidth(lines[i])
		if indent <= anchorIndent {
			return -1
		}
		if trimmed == targetKey {
			return i
		}
	}
	return -1
}

func findLineWithTrimmedValue(lines []string, start int, target string) int {
	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == target {
			return i
		}
	}
	return -1
}

func normalizeYAMLNodeStyles(node *yaml.Node) {
	if node == nil {
		return
	}
	node.Style = 0
	for _, child := range node.Content {
		normalizeYAMLNodeStyles(child)
	}
}

func indentationWidth(line string) int {
	width := 0
	for _, ch := range line {
		if ch == ' ' {
			width++
			continue
		}
		if ch == '\t' {
			width += 2
			continue
		}
		break
	}
	return width
}

func parseAndValidateYAML(configType ConfigType, content string) (*yaml.Node, *yaml.Node, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, nil, &ValidationError{
			ConfigType: configType,
			Message:    fmt.Sprintf("Invalid YAML in %s: content is empty", configType),
		}
	}

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		return nil, nil, &ValidationError{
			ConfigType: configType,
			Message:    fmt.Sprintf("Invalid YAML in %s: %v", configType, err),
		}
	}

	mapping := &root
	if mapping.Kind == yaml.DocumentNode {
		if len(mapping.Content) == 0 {
			return nil, nil, &ValidationError{
				ConfigType: configType,
				Message:    fmt.Sprintf("Invalid YAML in %s: document is empty", configType),
			}
		}
		mapping = mapping.Content[0]
	}

	if mapping.Kind != yaml.MappingNode {
		return nil, nil, &ValidationError{
			ConfigType: configType,
			Message:    fmt.Sprintf("Invalid YAML in %s: root must be a mapping object", configType),
		}
	}

	expectedRootKey := expectedTopLevelKeyForConfigType(configType)
	if expectedRootKey != "" && findTopLevelKey(mapping, expectedRootKey) == nil {
		return nil, nil, &ValidationError{
			ConfigType: configType,
			Message:    fmt.Sprintf("Invalid %s: expected top-level key '%s'", configType, expectedRootKey),
		}
	}

	return &root, mapping, nil
}

func expectedTopLevelKeyForConfigType(configType ConfigType) string {
	switch configType {
	case ConfigTypeSeatunnel:
		return "seatunnel"
	case ConfigTypeHazelcastClient:
		return "hazelcast-client"
	case ConfigTypeHazelcast,
		ConfigTypeHazelcastMaster,
		ConfigTypeHazelcastWorker:
		return "hazelcast"
	default:
		return ""
	}
}

func isHazelcastConfigType(configType ConfigType) bool {
	switch configType {
	case ConfigTypeHazelcast,
		ConfigTypeHazelcastClient,
		ConfigTypeHazelcastMaster,
		ConfigTypeHazelcastWorker:
		return true
	default:
		return false
	}
}

func findTopLevelKey(root *yaml.Node, key string) *yaml.Node {
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	for idx := 0; idx+1 < len(root.Content); idx += 2 {
		if strings.TrimSpace(root.Content[idx].Value) == key {
			return root.Content[idx+1]
		}
	}
	return nil
}
