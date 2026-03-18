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
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUninstallPluginRejectsPathTraversalPluginName(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewManager(baseDir)

	err := manager.UninstallPlugin("../../etc/passwd", "1.0.0", "", true)
	if err == nil {
		t.Fatalf("expected error for invalid plugin name")
	}
	if !errors.Is(err, ErrInvalidPluginIdentifier) {
		t.Fatalf("expected ErrInvalidPluginIdentifier, got %v", err)
	}
}

func TestUninstallPluginRejectsPathTraversalVersion(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewManager(baseDir)

	err := manager.UninstallPlugin("jdbc", "../1.0.0", "", true)
	if err == nil {
		t.Fatalf("expected error for invalid version")
	}
	if !errors.Is(err, ErrInvalidPluginIdentifier) {
		t.Fatalf("expected ErrInvalidPluginIdentifier, got %v", err)
	}
}

func TestUninstallPluginRemovesConnectorWithinConnectorsDir(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewManager(baseDir)

	connectorsDir := manager.GetConnectorsDir()
	if err := os.MkdirAll(connectorsDir, 0755); err != nil {
		t.Fatalf("mkdir connectors: %v", err)
	}

	pluginName := "jdbc"
	version := "1.0.0"
	connectorPath := filepath.Join(connectorsDir, "connector-"+pluginName+"-"+version+".jar")
	if err := os.WriteFile(connectorPath, []byte("test"), 0644); err != nil {
		t.Fatalf("write connector: %v", err)
	}

	if err := manager.UninstallPlugin(pluginName, version, "", true); err != nil {
		t.Fatalf("uninstall plugin: %v", err)
	}

	if _, err := os.Stat(connectorPath); !os.IsNotExist(err) {
		t.Fatalf("expected connector file removed, stat err: %v", err)
	}
}
