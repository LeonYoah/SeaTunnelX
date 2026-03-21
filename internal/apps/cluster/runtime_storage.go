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

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// RuntimeStorageDetails summarizes checkpoint and IMAP runtime storage.
// RuntimeStorageDetails 汇总 checkpoint 与 IMAP 运行时存储信息。
type RuntimeStorageDetails struct {
	ClusterID   uint                `json:"cluster_id"`
	Checkpoint  *RuntimeStorageSpec `json:"checkpoint,omitempty"`
	IMAP        *RuntimeStorageSpec `json:"imap,omitempty"`
	ConfigSource string             `json:"config_source,omitempty"`
}

// RuntimeStorageSpec describes one runtime storage area.
// RuntimeStorageSpec 描述一类运行时存储。
type RuntimeStorageSpec struct {
	Kind            string                    `json:"kind"`
	Enabled         bool                      `json:"enabled"`
	StorageType     string                    `json:"storage_type"`
	Namespace       string                    `json:"namespace,omitempty"`
	Endpoint        string                    `json:"endpoint,omitempty"`
	Bucket          string                    `json:"bucket,omitempty"`
	External        bool                      `json:"external"`
	SizeAvailable   bool                      `json:"size_available"`
	TotalSizeBytes  int64                     `json:"total_size_bytes"`
	CleanupSupported bool                     `json:"cleanup_supported"`
	Warning         string                    `json:"warning,omitempty"`
	Nodes           []*RuntimeStorageNodeStat `json:"nodes,omitempty"`
}

// RuntimeStorageNodeStat describes one node's runtime storage usage.
// RuntimeStorageNodeStat 描述单个节点的运行时存储使用情况。
type RuntimeStorageNodeStat struct {
	NodeID     uint   `json:"node_id"`
	HostID     uint   `json:"host_id"`
	HostName   string `json:"host_name"`
	Path       string `json:"path,omitempty"`
	Exists     bool   `json:"exists"`
	SizeBytes  int64  `json:"size_bytes"`
	Message    string `json:"message,omitempty"`
}

// RuntimeStorageCleanupResult describes a cleanup operation result.
// RuntimeStorageCleanupResult 描述清理操作结果。
type RuntimeStorageCleanupResult struct {
	ClusterID uint                         `json:"cluster_id"`
	Success   bool                         `json:"success"`
	Message   string                       `json:"message"`
	Nodes     []*RuntimeStorageNodeCleanup `json:"nodes,omitempty"`
}

// RuntimeStorageNodeCleanup describes one node's cleanup result.
// RuntimeStorageNodeCleanup 描述单个节点的清理结果。
type RuntimeStorageNodeCleanup struct {
	HostID   uint   `json:"host_id"`
	HostName string `json:"host_name"`
	Success  bool   `json:"success"`
	Message  string `json:"message"`
}

// GetRuntimeStorageDetails returns runtime storage config and local usage stats.
// GetRuntimeStorageDetails 返回运行时存储配置及本地使用统计。
func (s *Service) GetRuntimeStorageDetails(ctx context.Context, clusterID uint) (*RuntimeStorageDetails, error) {
	clusterObj, err := s.Get(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	nodes, err := s.GetNodes(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	details := &RuntimeStorageDetails{ClusterID: clusterID, ConfigSource: "cluster_config"}
	checkpoint, imap, source := extractRuntimeStorageFromClusterConfig(clusterObj.Config)
	if strings.TrimSpace(source) != "" {
		details.ConfigSource = source
	}

	if liveNode := pickRuntimeStorageConfigNode(nodes); liveNode != nil && s.agentSender != nil {
		if liveCheckpoint, err := s.loadCheckpointFromNode(ctx, liveNode); err == nil && liveCheckpoint != nil {
			checkpoint = liveCheckpoint
			details.ConfigSource = "live"
		}
		if liveIMAP, err := s.loadIMAPFromNode(ctx, liveNode); err == nil && liveIMAP != nil {
			imap = liveIMAP
			details.ConfigSource = "live"
		}
	}

	if checkpoint == nil {
		checkpoint = &RuntimeStorageSpec{Kind: "checkpoint"}
	}
	if imap == nil {
		imap = &RuntimeStorageSpec{Kind: "imap", StorageType: "DISABLED"}
	}

	s.fillLocalRuntimeStorageStats(ctx, nodes, checkpoint)
	s.fillLocalRuntimeStorageStats(ctx, nodes, imap)
	details.Checkpoint = checkpoint
	details.IMAP = imap
	return details, nil
}

// CleanupIMAPStorage cleans local IMAP directories on all online nodes when IMAP uses LOCAL_FILE.
// CleanupIMAPStorage 在 IMAP 使用 LOCAL_FILE 时清理所有在线节点上的本地 IMAP 目录。
func (s *Service) CleanupIMAPStorage(ctx context.Context, clusterID uint) (*RuntimeStorageCleanupResult, error) {
	details, err := s.GetRuntimeStorageDetails(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	if details.IMAP == nil || !details.IMAP.Enabled || !strings.EqualFold(details.IMAP.StorageType, "LOCAL_FILE") {
		return nil, fmt.Errorf("imap cleanup is only supported for LOCAL_FILE mode / 仅支持 LOCAL_FILE 模式的 IMAP 清理")
	}
	nodes, err := s.GetNodes(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	result := &RuntimeStorageCleanupResult{
		ClusterID: clusterID,
		Success:   true,
		Message:   "IMAP cleanup completed",
		Nodes:     make([]*RuntimeStorageNodeCleanup, 0, len(nodes)),
	}
	for _, node := range nodes {
		if node == nil {
			continue
		}
		hostInfo, err := s.hostProvider.GetHostByID(ctx, node.HostID)
		hostName := node.HostName
		if err != nil || hostInfo == nil || !hostInfo.IsOnline(s.heartbeatTimeout) || strings.TrimSpace(hostInfo.AgentID) == "" {
			result.Success = false
			result.Nodes = append(result.Nodes, &RuntimeStorageNodeCleanup{
				HostID:   node.HostID,
				HostName: hostName,
				Success:  false,
				Message:  "host agent is offline",
			})
			continue
		}
		success, message, sendErr := s.agentSender.SendCommand(ctx, hostInfo.AgentID, "cleanup_path", map[string]string{
			"path": details.IMAP.Namespace,
		})
		if sendErr != nil {
			success = false
			message = sendErr.Error()
		}
		nodeResult := &RuntimeStorageNodeCleanup{
			HostID:   node.HostID,
			HostName: hostName,
			Success:  success,
			Message:  parseCommandMessage(message),
		}
		if !success {
			result.Success = false
		}
		result.Nodes = append(result.Nodes, nodeResult)
	}
	if !result.Success {
		result.Message = "IMAP cleanup finished with warnings"
	}
	return result, nil
}

func pickRuntimeStorageConfigNode(nodes []*NodeInfo) *NodeInfo {
	for _, node := range nodes {
		if node == nil {
			continue
		}
		if node.IsOnline && (node.Role == NodeRoleMaster || node.Role == NodeRoleMasterWorker) {
			return node
		}
	}
	for _, node := range nodes {
		if node != nil && node.IsOnline {
			return node
		}
	}
	return nil
}

func (s *Service) loadCheckpointFromNode(ctx context.Context, node *NodeInfo) (*RuntimeStorageSpec, error) {
	content, err := s.pullConfigFromNode(ctx, node, "seatunnel.yaml")
	if err != nil {
		return nil, err
	}
	return parseCheckpointStorageFromYAML(content), nil
}

func (s *Service) loadIMAPFromNode(ctx context.Context, node *NodeInfo) (*RuntimeStorageSpec, error) {
	configType := "hazelcast.yaml"
	if node.Role == NodeRoleMaster {
		configType = "hazelcast-master.yaml"
	} else if node.Role == NodeRoleWorker {
		configType = "hazelcast-worker.yaml"
	}
	content, err := s.pullConfigFromNode(ctx, node, configType)
	if err != nil {
		return nil, err
	}
	return parseIMAPStorageFromYAML(content), nil
}

func (s *Service) pullConfigFromNode(ctx context.Context, node *NodeInfo, configType string) (string, error) {
	if s.agentSender == nil {
		return "", fmt.Errorf("agent sender is not configured")
	}
	hostInfo, err := s.hostProvider.GetHostByID(ctx, node.HostID)
	if err != nil {
		return "", err
	}
	if hostInfo == nil || !hostInfo.IsOnline(s.heartbeatTimeout) || strings.TrimSpace(hostInfo.AgentID) == "" {
		return "", fmt.Errorf("host agent is offline")
	}
	success, message, err := s.agentSender.SendCommand(ctx, hostInfo.AgentID, "pull_config", map[string]string{
		"install_dir": node.InstallDir,
		"config_type": configType,
	})
	if err != nil {
		return "", err
	}
	if !success {
		return "", fmt.Errorf("%s", parseCommandMessage(message))
	}
	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Content string `json:"content"`
	}
	if jsonErr := json.Unmarshal([]byte(message), &payload); jsonErr != nil {
		return "", jsonErr
	}
	if !payload.Success {
		return "", fmt.Errorf("%s", payload.Message)
	}
	return payload.Content, nil
}

func (s *Service) fillLocalRuntimeStorageStats(ctx context.Context, nodes []*NodeInfo, spec *RuntimeStorageSpec) {
	if spec == nil || !spec.Enabled || !strings.EqualFold(spec.StorageType, "LOCAL_FILE") || s.agentSender == nil {
		return
	}
	spec.SizeAvailable = true
	spec.CleanupSupported = spec.Kind == "imap"
	spec.Nodes = make([]*RuntimeStorageNodeStat, 0, len(nodes))
	var total int64
	for _, node := range nodes {
		if node == nil {
			continue
		}
		hostInfo, err := s.hostProvider.GetHostByID(ctx, node.HostID)
		hostName := node.HostName
		nodeStat := &RuntimeStorageNodeStat{
			NodeID:   node.ID,
			HostID:   node.HostID,
			HostName: hostName,
			Path:     spec.Namespace,
		}
		if err != nil || hostInfo == nil || !hostInfo.IsOnline(s.heartbeatTimeout) || strings.TrimSpace(hostInfo.AgentID) == "" {
			nodeStat.Message = "host agent is offline"
			spec.Nodes = append(spec.Nodes, nodeStat)
			continue
		}
		success, message, sendErr := s.agentSender.SendCommand(ctx, hostInfo.AgentID, "stat_path", map[string]string{
			"path": spec.Namespace,
		})
		if sendErr != nil || !success {
			nodeStat.Message = parseCommandMessage(firstNonEmpty(message, errorString(sendErr)))
			spec.Nodes = append(spec.Nodes, nodeStat)
			continue
		}
		var payload struct {
			Success bool              `json:"success"`
			Message string            `json:"message"`
			Details map[string]string `json:"details"`
		}
		if jsonErr := json.Unmarshal([]byte(message), &payload); jsonErr != nil {
			nodeStat.Message = jsonErr.Error()
			spec.Nodes = append(spec.Nodes, nodeStat)
			continue
		}
		nodeStat.Message = payload.Message
		nodeStat.Exists = strings.EqualFold(payload.Details["exists"], "true")
		nodeStat.Path = firstNonEmpty(payload.Details["path"], spec.Namespace)
		sizeBytes, _ := strconv.ParseInt(payload.Details["size_bytes"], 10, 64)
		nodeStat.SizeBytes = sizeBytes
		total += sizeBytes
		spec.Nodes = append(spec.Nodes, nodeStat)
	}
	spec.TotalSizeBytes = total
}

func parseCheckpointStorageFromYAML(content string) *RuntimeStorageSpec {
	var root map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		return nil
	}
	pluginConfig := nestedMap(root, "seatunnel", "engine", "checkpoint", "storage", "plugin-config")
	if len(pluginConfig) == 0 {
		return &RuntimeStorageSpec{Kind: "checkpoint"}
	}
	return runtimeSpecFromPluginConfig("checkpoint", pluginConfig)
}

func parseIMAPStorageFromYAML(content string) *RuntimeStorageSpec {
	var root map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		return nil
	}
	engineMapStore := nestedMap(root, "map", "engine*", "map-store")
	if len(engineMapStore) == 0 {
		engineMapStore = nestedMap(root, "hazelcast", "map", "engine*", "map-store")
	}
	if len(engineMapStore) == 0 {
		return &RuntimeStorageSpec{Kind: "imap", StorageType: "DISABLED"}
	}
	enabled := strings.EqualFold(asString(engineMapStore["enabled"]), "true")
	if !enabled {
		return &RuntimeStorageSpec{
			Kind:        "imap",
			Enabled:     false,
			StorageType: "DISABLED",
			External:    false,
		}
	}
	properties := asMap(engineMapStore["properties"])
	if len(properties) == 0 {
		return &RuntimeStorageSpec{Kind: "imap", Enabled: true}
	}
	spec := runtimeSpecFromPluginConfig("imap", properties)
	if spec != nil {
		spec.Enabled = true
	}
	return spec
}

func runtimeSpecFromPluginConfig(kind string, pluginConfig map[string]interface{}) *RuntimeStorageSpec {
	spec := &RuntimeStorageSpec{
		Kind:    kind,
		Enabled: true,
	}
	storageType := strings.ToUpper(asString(pluginConfig["storage.type"]))
	if storageType == "" {
		if fsDefault := asString(pluginConfig["fs.defaultFS"]); strings.HasPrefix(fsDefault, "file:///") {
			storageType = "LOCAL_FILE"
		}
	}
	switch storageType {
	case "", "HDFS":
		if fsDefault := asString(pluginConfig["fs.defaultFS"]); strings.HasPrefix(fsDefault, "file:///") {
			spec.StorageType = "LOCAL_FILE"
			spec.External = false
		} else {
			spec.StorageType = "HDFS"
			spec.External = true
		}
	case "OSS":
		spec.StorageType = "OSS"
		spec.External = true
	case "S3":
		spec.StorageType = "S3"
		spec.External = true
	default:
		spec.StorageType = storageType
		spec.External = true
	}
	spec.Namespace = asString(pluginConfig["namespace"])
	switch spec.StorageType {
	case "OSS":
		spec.Endpoint = asString(pluginConfig["fs.oss.endpoint"])
		spec.Bucket = asString(pluginConfig["oss.bucket"])
	case "S3":
		spec.Endpoint = asString(pluginConfig["fs.s3a.endpoint"])
		spec.Bucket = runtimeStorageFirstNonEmpty(asString(pluginConfig["s3.bucket"]), asString(pluginConfig["fs.defaultFS"]))
	case "HDFS":
		spec.Endpoint = asString(pluginConfig["fs.defaultFS"])
	}
	return spec
}

func extractRuntimeStorageFromClusterConfig(cfg ClusterConfig) (*RuntimeStorageSpec, *RuntimeStorageSpec, string) {
	if cfg == nil {
		return nil, nil, ""
	}
	var checkpoint *RuntimeStorageSpec
	if checkpointCfg := asMap(cfg["checkpoint"]); len(checkpointCfg) > 0 {
		checkpoint = runtimeSpecFromClusterConfig("checkpoint", checkpointCfg)
	}
	var imap *RuntimeStorageSpec
	if imapCfg := asMap(cfg["imap"]); len(imapCfg) > 0 {
		imap = runtimeSpecFromClusterConfig("imap", imapCfg)
	}
	return checkpoint, imap, "cluster_config"
}

func runtimeSpecFromClusterConfig(kind string, raw map[string]interface{}) *RuntimeStorageSpec {
	storageType := strings.ToUpper(asString(raw["storage_type"]))
	spec := &RuntimeStorageSpec{
		Kind:        kind,
		Enabled:     true,
		StorageType: storageType,
		Namespace:   asString(raw["namespace"]),
	}
	if kind == "imap" && storageType == "DISABLED" {
		spec.Enabled = false
		spec.External = false
		return spec
	}
	switch storageType {
	case "LOCAL_FILE":
		spec.External = false
	case "OSS", "S3", "HDFS":
		spec.External = true
	default:
		spec.External = false
	}
	spec.Endpoint = asString(raw["storage_endpoint"])
	spec.Bucket = asString(raw["storage_bucket"])
	return spec
}

func nestedMap(root map[string]interface{}, path ...string) map[string]interface{} {
	current := root
	for _, key := range path {
		next, ok := current[key]
		if !ok {
			return nil
		}
		current = asMap(next)
		if len(current) == 0 {
			return nil
		}
	}
	return current
}

func asMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	if m, ok := v.(map[interface{}]interface{}); ok {
		out := make(map[string]interface{}, len(m))
		for key, value := range m {
			out[fmt.Sprintf("%v", key)] = value
		}
		return out
	}
	return map[string]interface{}{}
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch typed := v.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func parseCommandMessage(message string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return ""
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil && payload.Message != "" {
		return payload.Message
	}
	return trimmed
}

func runtimeStorageFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
