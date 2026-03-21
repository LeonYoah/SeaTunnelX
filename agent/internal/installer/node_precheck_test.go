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
	"os"
	"path/filepath"
	"testing"
)

func TestCheckPathReadyAllowsCreatablePath(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "checkpoint")

	result := CheckPathReady(target)
	if !result.Success {
		t.Fatalf("expected path to be creatable, got %+v", result)
	}
	if got := result.Details["parent"]; got != root {
		t.Fatalf("unexpected parent directory: %s", got)
	}
}

func TestCheckPathReadyAllowsCreatablePathWithTrailingSlash(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "checkpoint") + string(os.PathSeparator)

	result := CheckPathReady(target)
	if !result.Success {
		t.Fatalf("expected path with trailing slash to be creatable, got %+v", result)
	}
	if got := result.Details["parent"]; got != root {
		t.Fatalf("unexpected parent directory: %s", got)
	}
}

func TestCleanupDirectoryContentsKeepsDirectory(t *testing.T) {
	root := t.TempDir()
	nestedDir := filepath.Join(root, "imap", "nested")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "state.bin"), []byte("abc"), 0o644); err != nil {
		t.Fatalf("failed to create state file: %v", err)
	}

	result := CleanupDirectoryContents(filepath.Join(root, "imap"))
	if !result.Success {
		t.Fatalf("expected cleanup to succeed, got %+v", result)
	}
	entries, err := os.ReadDir(filepath.Join(root, "imap"))
	if err != nil {
		t.Fatalf("failed to read cleaned dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected cleaned directory to be empty, got %d entries", len(entries))
	}
	if _, err := os.Stat(filepath.Join(root, "imap")); err != nil {
		t.Fatalf("expected parent directory to remain, got error %v", err)
	}
}
