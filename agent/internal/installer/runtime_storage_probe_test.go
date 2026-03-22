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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recordingProgressReporter struct {
	messages []string
}

func (r *recordingProgressReporter) Report(_ InstallStep, _ int, message string) error {
	r.messages = append(r.messages, message)
	return nil
}

func (r *recordingProgressReporter) ReportStepStart(step InstallStep) error {
	return r.Report(step, 0, "")
}

func (r *recordingProgressReporter) ReportStepComplete(step InstallStep) error {
	return r.Report(step, 100, "")
}

func (r *recordingProgressReporter) ReportStepFailed(step InstallStep, err error) error {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return r.Report(step, 0, message)
}

func (r *recordingProgressReporter) ReportStepSkipped(step InstallStep, reason string) error {
	return r.Report(step, 100, reason)
}

func TestBuildCheckpointRuntimeProbeRequestNormalizesNamespace(t *testing.T) {
	request, err := buildCheckpointRuntimeProbeRequest(&CheckpointConfig{
		StorageType:      CheckpointStorageS3,
		Namespace:        "/seatunnel/checkpoint",
		StorageBucket:    "s3a://checkpoint-bucket",
		StorageEndpoint:  "http://127.0.0.1:9000",
		StorageAccessKey: "minioadmin",
		StorageSecretKey: "minioadmin",
	})
	if err != nil {
		t.Fatalf("buildCheckpointRuntimeProbeRequest returned error: %v", err)
	}

	if request["plugin"] != "hdfs" {
		t.Fatalf("expected plugin=hdfs, got %#v", request["plugin"])
	}
	if request["mode"] != "read_write" {
		t.Fatalf("expected mode=read_write, got %#v", request["mode"])
	}

	config, ok := request["config"].(map[string]string)
	if !ok {
		t.Fatalf("expected config map[string]string, got %#v", request["config"])
	}
	if config["storage.type"] != "s3" {
		t.Fatalf("expected storage.type=s3, got %#v", config["storage.type"])
	}
	if config["namespace"] != "/seatunnel/checkpoint/" {
		t.Fatalf("expected normalized namespace, got %#v", config["namespace"])
	}
}

func TestBuildIMAPRuntimeProbeRequestIncludesBusinessAndClusterName(t *testing.T) {
	installDir := t.TempDir()
	configDir := filepath.Join(installDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(configDir, "hazelcast-master.yaml"),
		[]byte("hazelcast:\n  cluster-name: seatunnel-it\n"),
		0o644,
	); err != nil {
		t.Fatalf("failed to write hazelcast config: %v", err)
	}

	request, err := buildIMAPRuntimeProbeRequest(&InstallParams{
		InstallDir:     installDir,
		DeploymentMode: DeploymentModeSeparated,
		IMAP: &IMAPConfig{
			StorageType:      IMAPStorageS3,
			Namespace:        "/seatunnel/imap",
			StorageBucket:    "s3a://imap-bucket",
			StorageEndpoint:  "http://127.0.0.1:9000",
			StorageAccessKey: "minioadmin",
			StorageSecretKey: "minioadmin",
		},
	})
	if err != nil {
		t.Fatalf("buildIMAPRuntimeProbeRequest returned error: %v", err)
	}

	config, ok := request["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected config map[string]interface{}, got %#v", request["config"])
	}
	if config["businessName"] != runtimeProbeBusinessName {
		t.Fatalf("expected businessName=%s, got %#v", runtimeProbeBusinessName, config["businessName"])
	}
	if config["clusterName"] != "seatunnel-it" {
		t.Fatalf("expected clusterName=seatunnel-it, got %#v", config["clusterName"])
	}
	if config["namespace"] != "/seatunnel/imap/" {
		t.Fatalf("expected normalized namespace, got %#v", config["namespace"])
	}
}

func TestExecuteRuntimeStorageProbeSuccess(t *testing.T) {
	manager := NewInstallerManager()
	scriptPath := filepath.Join(t.TempDir(), "seatunnel-capability-proxy.sh")
	jarPath := filepath.Join(t.TempDir(), "seatunnel-capability-proxy.jar")

	if err := os.WriteFile(
		scriptPath,
		[]byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf '{\"ok\":true,\"statusCode\":200,\"message\":\"probe ok\",\"writable\":true,\"readable\":true}' > \"$6\"\n"),
		0o755,
	); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}
	if err := os.WriteFile(jarPath, []byte("jar"), 0o644); err != nil {
		t.Fatalf("failed to write jar: %v", err)
	}

	t.Setenv(capabilityProxyScriptEnvVar, scriptPath)
	t.Setenv(capabilityProxyJarEnvVar, jarPath)

	response, err := manager.executeRuntimeStorageProbe(context.Background(), t.TempDir(), "checkpoint", map[string]interface{}{
		"plugin": "hdfs",
	})
	if err != nil {
		t.Fatalf("executeRuntimeStorageProbe returned error: %v", err)
	}
	if !response.OK || !response.Writable || !response.Readable {
		t.Fatalf("expected successful probe response, got %#v", response)
	}
}

func TestExecuteRuntimeStorageProbeReturnsResponseOnFailureExit(t *testing.T) {
	manager := NewInstallerManager()
	scriptPath := filepath.Join(t.TempDir(), "seatunnel-capability-proxy.sh")
	jarPath := filepath.Join(t.TempDir(), "seatunnel-capability-proxy.jar")

	if err := os.WriteFile(
		scriptPath,
		[]byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf '{\"ok\":false,\"statusCode\":504,\"message\":\"probe failed\"}' > \"$6\"\nexit 1\n"),
		0o755,
	); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}
	if err := os.WriteFile(jarPath, []byte("jar"), 0o644); err != nil {
		t.Fatalf("failed to write jar: %v", err)
	}

	t.Setenv(capabilityProxyScriptEnvVar, scriptPath)
	t.Setenv(capabilityProxyJarEnvVar, jarPath)

	response, err := manager.executeRuntimeStorageProbe(context.Background(), t.TempDir(), "imap", map[string]interface{}{
		"plugin": "hdfs",
	})
	if err != nil {
		t.Fatalf("expected parsed response instead of error, got: %v", err)
	}
	if response.OK {
		t.Fatalf("expected non-success probe response, got %#v", response)
	}
	if response.Message != "probe failed" {
		t.Fatalf("expected failure message, got %#v", response.Message)
	}
}

func TestExecuteRuntimeStorageProbeFailsWhenAssetsMissing(t *testing.T) {
	manager := NewInstallerManager()
	t.Setenv(capabilityProxyScriptEnvVar, filepath.Join(t.TempDir(), "missing.sh"))
	t.Setenv(capabilityProxyJarEnvVar, filepath.Join(t.TempDir(), "missing.jar"))

	_, err := manager.executeRuntimeStorageProbe(context.Background(), t.TempDir(), "checkpoint", map[string]interface{}{
		"plugin": "hdfs",
	})
	if err == nil {
		t.Fatal("expected missing asset error")
	}
}

func TestExecuteStepConfigureCheckpointKeepsInstallRunningOnProbeWarning(t *testing.T) {
	manager := NewInstallerManager()
	installDir := t.TempDir()
	configDir := filepath.Join(installDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(configDir, "seatunnel.yaml"),
		[]byte("seatunnel:\n  engine:\n    checkpoint:\n      storage:\n        plugin-config: {}\n"),
		0o644,
	); err != nil {
		t.Fatalf("failed to write seatunnel.yaml: %v", err)
	}

	scriptPath := filepath.Join(t.TempDir(), "seatunnel-capability-proxy.sh")
	jarPath := filepath.Join(t.TempDir(), "seatunnel-capability-proxy.jar")
	if err := os.WriteFile(
		scriptPath,
		[]byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf '{\"ok\":false,\"statusCode\":504,\"message\":\"probe failed\"}' > \"$6\"\nexit 1\n"),
		0o755,
	); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}
	if err := os.WriteFile(jarPath, []byte("jar"), 0o644); err != nil {
		t.Fatalf("failed to write jar: %v", err)
	}
	t.Setenv(capabilityProxyScriptEnvVar, scriptPath)
	t.Setenv(capabilityProxyJarEnvVar, jarPath)

	reporter := &recordingProgressReporter{}
	err := manager.executeStepConfigureCheckpoint(context.Background(), &InstallParams{
		InstallDir: installDir,
		Checkpoint: &CheckpointConfig{
			StorageType:      CheckpointStorageS3,
			Namespace:        "/seatunnel/checkpoint",
			StorageBucket:    "s3a://checkpoint-bucket",
			StorageEndpoint:  "http://127.0.0.1:9000",
			StorageAccessKey: "minioadmin",
			StorageSecretKey: "minioadmin",
		},
	}, reporter)
	if err != nil {
		t.Fatalf("expected warning-only behavior, got error: %v", err)
	}

	if len(reporter.messages) == 0 || !strings.Contains(reporter.messages[len(reporter.messages)-1], "Warning: checkpoint runtime probe issue") {
		t.Fatalf("expected warning report message, got %#v", reporter.messages)
	}

	content, err := os.ReadFile(filepath.Join(configDir, "seatunnel.yaml"))
	if err != nil {
		t.Fatalf("failed to read seatunnel.yaml: %v", err)
	}
	if !strings.Contains(string(content), "storage.type: s3") {
		t.Fatalf("expected checkpoint config to be applied, got %s", string(content))
	}
}

func TestExecuteStepConfigureIMAPKeepsInstallRunningOnProbeWarning(t *testing.T) {
	manager := NewInstallerManager()
	installDir := t.TempDir()
	configDir := filepath.Join(installDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(configDir, "hazelcast.yaml"),
		[]byte("hazelcast:\n  cluster-name: seatunnel\n"),
		0o644,
	); err != nil {
		t.Fatalf("failed to write hazelcast.yaml: %v", err)
	}

	scriptPath := filepath.Join(t.TempDir(), "seatunnel-capability-proxy.sh")
	jarPath := filepath.Join(t.TempDir(), "seatunnel-capability-proxy.jar")
	if err := os.WriteFile(
		scriptPath,
		[]byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf '{\"ok\":false,\"statusCode\":504,\"message\":\"probe failed\"}' > \"$6\"\nexit 1\n"),
		0o755,
	); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}
	if err := os.WriteFile(jarPath, []byte("jar"), 0o644); err != nil {
		t.Fatalf("failed to write jar: %v", err)
	}
	t.Setenv(capabilityProxyScriptEnvVar, scriptPath)
	t.Setenv(capabilityProxyJarEnvVar, jarPath)

	reporter := &recordingProgressReporter{}
	err := manager.executeStepConfigureIMAP(context.Background(), &InstallParams{
		InstallDir:     installDir,
		DeploymentMode: DeploymentModeHybrid,
		IMAP: &IMAPConfig{
			StorageType:      IMAPStorageS3,
			Namespace:        "/seatunnel/imap",
			StorageBucket:    "s3a://imap-bucket",
			StorageEndpoint:  "http://127.0.0.1:9000",
			StorageAccessKey: "minioadmin",
			StorageSecretKey: "minioadmin",
		},
	}, reporter)
	if err != nil {
		t.Fatalf("expected warning-only behavior, got error: %v", err)
	}

	if len(reporter.messages) == 0 || !strings.Contains(reporter.messages[len(reporter.messages)-1], "Warning: IMAP runtime probe issue") {
		t.Fatalf("expected warning report message, got %#v", reporter.messages)
	}

	content, err := os.ReadFile(filepath.Join(configDir, "hazelcast.yaml"))
	if err != nil {
		t.Fatalf("failed to read hazelcast.yaml: %v", err)
	}
	if !strings.Contains(string(content), "storage.type: s3") {
		t.Fatalf("expected IMAP config to be applied, got %s", string(content))
	}
}
