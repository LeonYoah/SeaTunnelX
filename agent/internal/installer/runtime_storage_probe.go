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

package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/seatunnel/seatunnelX/agent/internal/logger"
	seatunnelmeta "github.com/seatunnel/seatunnelX/internal/seatunnel"
	"gopkg.in/yaml.v3"
)

const (
	capabilityProxyJarEnvVar    = "SEATUNNEL_CAPABILITY_PROXY_JAR"
	capabilityProxyScriptEnvVar = "SEATUNNEL_CAPABILITY_PROXY_SCRIPT"
	seatunnelProxyJarEnvVar     = "SEATUNNEL_PROXY_JAR"
	runtimeProbeTimeout         = 20 * time.Second
	runtimeProbeBusinessName    = "imap-probe"
	runtimeProbeClusterName     = "seatunnel-cluster"
)

type runtimeStorageProbeResponse struct {
	OK         bool   `json:"ok"`
	StatusCode int    `json:"statusCode,omitempty"`
	Message    string `json:"message,omitempty"`
	Writable   bool   `json:"writable,omitempty"`
	Readable   bool   `json:"readable,omitempty"`
}

func buildCheckpointPluginConfig(cfg *CheckpointConfig) (map[string]string, error) {
	namespace := normalizeRuntimeStorageNamespace(cfg.Namespace)
	pluginConfig := make(map[string]string)

	switch cfg.StorageType {
	case CheckpointStorageLocalFile:
		pluginConfig["storage.type"] = "hdfs"
		pluginConfig["namespace"] = namespace
		pluginConfig["fs.defaultFS"] = "file:///"

	case CheckpointStorageHDFS:
		pluginConfig["storage.type"] = "hdfs"
		pluginConfig["namespace"] = namespace
		if cfg.HDFSHAEnabled {
			if strings.TrimSpace(cfg.HDFSNameServices) == "" {
				return nil, fmt.Errorf("hdfs_name_services is required for HDFS HA storage")
			}
			haEndpoints, err := seatunnelmeta.ResolveHDFSHARPCAddresses(
				cfg.HDFSHANamenodes,
				cfg.HDFSNamenodeRPCAddress1,
				cfg.HDFSNamenodeRPCAddress2,
			)
			if err != nil {
				return nil, err
			}
			pluginConfig["fs.defaultFS"] = fmt.Sprintf("hdfs://%s", cfg.HDFSNameServices)
			pluginConfig["seatunnel.hadoop.dfs.nameservices"] = cfg.HDFSNameServices
			pluginConfig[fmt.Sprintf("seatunnel.hadoop.dfs.ha.namenodes.%s", cfg.HDFSNameServices)] = cfg.HDFSHANamenodes
			for _, endpoint := range haEndpoints {
				pluginConfig[fmt.Sprintf("seatunnel.hadoop.dfs.namenode.rpc-address.%s.%s", cfg.HDFSNameServices, endpoint.Name)] = endpoint.Address
			}
			failoverProvider := cfg.HDFSFailoverProxyProvider
			if failoverProvider == "" {
				failoverProvider = "org.apache.hadoop.hdfs.server.namenode.ha.ConfiguredFailoverProxyProvider"
			}
			pluginConfig[fmt.Sprintf("seatunnel.hadoop.dfs.client.failover.proxy.provider.%s", cfg.HDFSNameServices)] = failoverProvider
		} else {
			pluginConfig["fs.defaultFS"] = fmt.Sprintf("hdfs://%s:%d", cfg.HDFSNameNodeHost, cfg.HDFSNameNodePort)
		}
		if cfg.KerberosPrincipal != "" {
			pluginConfig["kerberosPrincipal"] = cfg.KerberosPrincipal
		}
		if cfg.KerberosKeytabFilePath != "" {
			pluginConfig["kerberosKeytabFilePath"] = cfg.KerberosKeytabFilePath
		}

	case CheckpointStorageOSS:
		pluginConfig["storage.type"] = "oss"
		pluginConfig["namespace"] = namespace
		if cfg.StorageBucket != "" {
			pluginConfig["oss.bucket"] = cfg.StorageBucket
		}
		if cfg.StorageEndpoint != "" {
			pluginConfig["fs.oss.endpoint"] = cfg.StorageEndpoint
		}
		if cfg.StorageAccessKey != "" {
			pluginConfig["fs.oss.accessKeyId"] = cfg.StorageAccessKey
		}
		if cfg.StorageSecretKey != "" {
			pluginConfig["fs.oss.accessKeySecret"] = cfg.StorageSecretKey
		}

	case CheckpointStorageS3:
		pluginConfig["storage.type"] = "s3"
		pluginConfig["namespace"] = namespace
		if cfg.StorageBucket != "" {
			pluginConfig["s3.bucket"] = cfg.StorageBucket
		}
		if cfg.StorageEndpoint != "" {
			pluginConfig["fs.s3a.endpoint"] = cfg.StorageEndpoint
		}
		if cfg.StorageAccessKey != "" {
			pluginConfig["fs.s3a.access.key"] = cfg.StorageAccessKey
		}
		if cfg.StorageSecretKey != "" {
			pluginConfig["fs.s3a.secret.key"] = cfg.StorageSecretKey
		}
		pluginConfig["fs.s3a.aws.credentials.provider"] = "org.apache.hadoop.fs.s3a.SimpleAWSCredentialsProvider"

	default:
		pluginConfig["storage.type"] = "hdfs"
		pluginConfig["namespace"] = namespace
		pluginConfig["fs.defaultFS"] = "file:///"
	}

	return pluginConfig, nil
}

func normalizeRuntimeStorageNamespace(namespace string) string {
	trimmed := strings.TrimSpace(namespace)
	if trimmed != "" && !strings.HasSuffix(trimmed, "/") {
		trimmed += "/"
	}
	return trimmed
}

func (m *InstallerManager) maybeProbeCheckpointRuntimeStorage(ctx context.Context, params *InstallParams) string {
	if params == nil || params.Checkpoint == nil || !isRemoteCheckpointStorage(params.Checkpoint.StorageType) {
		return ""
	}
	request, err := buildCheckpointRuntimeProbeRequest(params.Checkpoint)
	if err != nil {
		return fmt.Sprintf("failed to build checkpoint probe request: %v", err)
	}
	response, err := m.executeRuntimeStorageProbe(ctx, params.InstallDir, "checkpoint", request)
	if err != nil {
		logger.WarnF(ctx, "[Install] checkpoint runtime probe execution failed: install_dir=%s, error=%v", params.InstallDir, err)
		return err.Error()
	}
	if !response.OK {
		return firstNonBlank(response.Message, "checkpoint runtime probe returned a non-success response")
	}
	if !response.Writable || !response.Readable {
		return fmt.Sprintf(
			"checkpoint runtime probe reported incomplete access (writable=%t, readable=%t)",
			response.Writable,
			response.Readable,
		)
	}
	logger.InfoF(ctx, "[Install] checkpoint runtime probe succeeded: install_dir=%s", params.InstallDir)
	return ""
}

func (m *InstallerManager) maybeProbeIMAPRuntimeStorage(ctx context.Context, params *InstallParams) string {
	if params == nil || params.IMAP == nil || !isRemoteIMAPStorage(params.IMAP.StorageType) {
		return ""
	}
	request, err := buildIMAPRuntimeProbeRequest(params)
	if err != nil {
		return fmt.Sprintf("failed to build IMAP probe request: %v", err)
	}
	response, err := m.executeRuntimeStorageProbe(ctx, params.InstallDir, "imap", request)
	if err != nil {
		logger.WarnF(ctx, "[Install] IMAP runtime probe execution failed: install_dir=%s, error=%v", params.InstallDir, err)
		return err.Error()
	}
	if !response.OK {
		return firstNonBlank(response.Message, "IMAP runtime probe returned a non-success response")
	}
	if !response.Writable || !response.Readable {
		return fmt.Sprintf(
			"IMAP runtime probe reported incomplete access (writable=%t, readable=%t)",
			response.Writable,
			response.Readable,
		)
	}
	logger.InfoF(ctx, "[Install] IMAP runtime probe succeeded: install_dir=%s", params.InstallDir)
	return ""
}

func buildCheckpointRuntimeProbeRequest(cfg *CheckpointConfig) (map[string]interface{}, error) {
	pluginConfig, err := buildCheckpointPluginConfig(cfg)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"plugin":         "hdfs",
		"mode":           "read_write",
		"probeTimeoutMs": runtimeProbeTimeout.Milliseconds(),
		"config":         pluginConfig,
	}, nil
}

func buildIMAPRuntimeProbeRequest(params *InstallParams) (map[string]interface{}, error) {
	clusterName := resolveRuntimeProbeClusterName(params.InstallDir, params.DeploymentMode)
	properties, err := buildIMAPProperties(
		params.IMAP,
		normalizeRuntimeStorageNamespace(params.IMAP.Namespace),
		clusterName,
	)
	if err != nil {
		return nil, err
	}
	config := make(map[string]interface{}, len(properties)+1)
	for key, value := range properties {
		config[key] = value
	}
	config["businessName"] = runtimeProbeBusinessName

	return map[string]interface{}{
		"plugin":             "hdfs",
		"mode":               "read_write",
		"deleteAllOnDestroy": true,
		"probeTimeoutMs":     runtimeProbeTimeout.Milliseconds(),
		"config":             config,
	}, nil
}

func (m *InstallerManager) executeRuntimeStorageProbe(
	ctx context.Context,
	installDir string,
	kind string,
	request map[string]interface{},
) (*runtimeStorageProbeResponse, error) {
	scriptPath, err := resolveCapabilityProxyScriptPath(installDir)
	if err != nil {
		return nil, err
	}
	jarPath, err := resolveCapabilityProxyJarPath(installDir)
	if err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp("", "seatunnel-runtime-probe-*")
	if err != nil {
		return nil, fmt.Errorf("create runtime probe temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	requestPath := filepath.Join(tempDir, "request.json")
	responsePath := filepath.Join(tempDir, "response.json")
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal runtime probe request: %w", err)
	}
	if err := os.WriteFile(requestPath, payload, 0600); err != nil {
		return nil, fmt.Errorf("write runtime probe request file: %w", err)
	}

	probeCtx, cancel := context.WithTimeout(ctx, runtimeProbeTimeout+(5*time.Second))
	defer cancel()

	cmd := exec.CommandContext(
		probeCtx,
		"bash",
		scriptPath,
		"probe-once",
		kind,
		"--request-file",
		requestPath,
		"--response-file",
		responsePath,
	)
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("SEATUNNEL_HOME=%s", installDir),
		fmt.Sprintf("%s=%s", seatunnelProxyJarEnvVar, jarPath),
	)
	output, execErr := cmd.CombinedOutput()

	response, responseErr := readRuntimeStorageProbeResponse(responsePath)
	if responseErr == nil && response != nil {
		return response, nil
	}
	if execErr != nil {
		return nil, fmt.Errorf(
			"run capability proxy probe with script %s and jar %s: %v: %s",
			scriptPath,
			jarPath,
			execErr,
			strings.TrimSpace(string(output)),
		)
	}
	if responseErr != nil {
		return nil, responseErr
	}
	return nil, fmt.Errorf("runtime probe returned no response")
}

func readRuntimeStorageProbeResponse(path string) (*runtimeStorageProbeResponse, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read runtime probe response file: %w", err)
	}
	if len(bytes) == 0 {
		return nil, fmt.Errorf("runtime probe response file is empty")
	}
	var response runtimeStorageProbeResponse
	if err := json.Unmarshal(bytes, &response); err != nil {
		return nil, fmt.Errorf("parse runtime probe response: %w", err)
	}
	return &response, nil
}

func isRemoteCheckpointStorage(storageType CheckpointStorageType) bool {
	return storageType == CheckpointStorageHDFS ||
		storageType == CheckpointStorageOSS ||
		storageType == CheckpointStorageS3
}

func isRemoteIMAPStorage(storageType IMAPStorageType) bool {
	return storageType == IMAPStorageHDFS ||
		storageType == IMAPStorageOSS ||
		storageType == IMAPStorageS3
}

func resolveRuntimeProbeClusterName(installDir string, deploymentMode DeploymentMode) string {
	configFiles := []string{filepath.Join(installDir, "config", "hazelcast.yaml")}
	if deploymentMode == DeploymentModeSeparated {
		configFiles = []string{filepath.Join(installDir, "config", "hazelcast-master.yaml")}
	}

	for _, configFile := range configFiles {
		content, err := os.ReadFile(configFile)
		if err != nil {
			continue
		}
		var root yaml.Node
		if err := yaml.Unmarshal(content, &root); err != nil {
			continue
		}
		documentRoot := ensureDocumentMappingNode(&root)
		hazelcastNode := findMappingChildNode(documentRoot, "hazelcast")
		if hazelcastNode == nil {
			continue
		}
		clusterName := strings.TrimSpace(getMappingString(hazelcastNode, "cluster-name"))
		if clusterName != "" {
			return clusterName
		}
	}

	return runtimeProbeClusterName
}

func resolveCapabilityProxyScriptPath(installDir string) (string, error) {
	if envPath := strings.TrimSpace(os.Getenv(capabilityProxyScriptEnvVar)); envPath != "" {
		if fileExists(envPath) {
			return envPath, nil
		}
		return "", fmt.Errorf("capability proxy script not found at %s", envPath)
	}

	for _, candidate := range capabilityProxyScriptCandidates(installDir) {
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("capability proxy script is unavailable")
}

func resolveCapabilityProxyJarPath(installDir string) (string, error) {
	if envPath := strings.TrimSpace(os.Getenv(capabilityProxyJarEnvVar)); envPath != "" {
		if fileExists(envPath) {
			return envPath, nil
		}
		return "", fmt.Errorf("capability proxy jar not found at %s", envPath)
	}

	for _, candidate := range capabilityProxyJarCandidates(installDir) {
		if strings.Contains(candidate, "*") {
			matches, _ := filepath.Glob(candidate)
			sort.Strings(matches)
			for _, match := range matches {
				if fileExists(match) && !strings.HasSuffix(match, "-bin.jar") {
					return match, nil
				}
			}
			continue
		}
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("capability proxy jar is unavailable")
}

func capabilityProxyScriptCandidates(installDir string) []string {
	candidates := []string{
		filepath.Join(installDir, "bin", "seatunnel-capability-proxy.sh"),
		filepath.Join("tools", "seatunnel-capability-proxy", "bin", "seatunnel-capability-proxy.sh"),
	}
	if executable, err := os.Executable(); err == nil {
		execDir := filepath.Dir(executable)
		candidates = append(
			candidates,
			filepath.Join(execDir, "tools", "seatunnel-capability-proxy", "bin", "seatunnel-capability-proxy.sh"),
			filepath.Join(execDir, "..", "tools", "seatunnel-capability-proxy", "bin", "seatunnel-capability-proxy.sh"),
			filepath.Join(execDir, "..", "..", "tools", "seatunnel-capability-proxy", "bin", "seatunnel-capability-proxy.sh"),
		)
	}
	return dedupeStrings(candidates)
}

func capabilityProxyJarCandidates(installDir string) []string {
	candidates := []string{
		filepath.Join(installDir, "tools", "seatunnel-capability-proxy.jar"),
		filepath.Join("tools", "seatunnel-capability-proxy", "target", "seatunnel-capability-proxy-*.jar"),
	}
	if executable, err := os.Executable(); err == nil {
		execDir := filepath.Dir(executable)
		candidates = append(
			candidates,
			filepath.Join(execDir, "tools", "seatunnel-capability-proxy", "target", "seatunnel-capability-proxy-*.jar"),
			filepath.Join(execDir, "..", "tools", "seatunnel-capability-proxy", "target", "seatunnel-capability-proxy-*.jar"),
			filepath.Join(execDir, "..", "..", "tools", "seatunnel-capability-proxy", "target", "seatunnel-capability-proxy-*.jar"),
		)
	}
	return dedupeStrings(candidates)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		clean := filepath.Clean(value)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	return result
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
