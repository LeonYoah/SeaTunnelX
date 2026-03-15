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
	"testing"
)

func TestInstallerManager_BackupInstallDir_skipsLogs(t *testing.T) {
	manager := NewInstallerManager()
	sourceDir := t.TempDir()
	backupDir := filepath.Join(t.TempDir(), "backup")
	mustWriteFile(t, filepath.Join(sourceDir, "bin", "seatunnel-cluster.sh"), "echo ok")
	mustWriteFile(t, filepath.Join(sourceDir, "logs", "seatunnel.log"), "log")

	createdBackupDir, err := manager.BackupInstallDir(context.Background(), sourceDir, backupDir)
	if err != nil {
		t.Fatalf("BackupInstallDir returned error: %v", err)
	}
	if createdBackupDir != backupDir {
		t.Fatalf("expected backup dir %s, got %s", backupDir, createdBackupDir)
	}
	if _, err := os.Stat(filepath.Join(backupDir, "bin", "seatunnel-cluster.sh")); err != nil {
		t.Fatalf("expected binary to be backed up: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backupDir, "logs")); !os.IsNotExist(err) {
		t.Fatalf("expected logs directory to be skipped, stat err=%v", err)
	}
}

func TestInstallerManager_SyncConnectorsManifest_preservesExistingFiles(t *testing.T) {
	manager := NewInstallerManager()
	installDir := t.TempDir()
	mustWriteFile(t, filepath.Join(installDir, "connectors", "keep.jar"), "keep")
	mustWriteFile(t, filepath.Join(installDir, "connectors", "bundled-default.jar"), "default")

	if err := manager.SyncConnectorsManifest(installDir, []string{"keep.jar"}); err != nil {
		t.Fatalf("SyncConnectorsManifest returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "connectors", "keep.jar")); err != nil {
		t.Fatalf("expected keep.jar to remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "connectors", "bundled-default.jar")); err != nil {
		t.Fatalf("expected bundled-default.jar to remain: %v", err)
	}
}

func TestInstallerManager_SyncLibManifest_preservesExistingFiles(t *testing.T) {
	manager := NewInstallerManager()
	installDir := t.TempDir()
	mustWriteFile(t, filepath.Join(installDir, "lib", "seatunnel-core.jar"), "core")
	mustWriteFile(t, filepath.Join(installDir, "lib", "mysql-cdc-dependency.jar"), "dependency")

	if err := manager.SyncLibManifest(installDir, []string{"mysql-cdc-dependency.jar"}); err != nil {
		t.Fatalf("SyncLibManifest returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "lib", "seatunnel-core.jar")); err != nil {
		t.Fatalf("expected seatunnel-core.jar to remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "lib", "mysql-cdc-dependency.jar")); err != nil {
		t.Fatalf("expected mysql-cdc-dependency.jar to remain: %v", err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}
