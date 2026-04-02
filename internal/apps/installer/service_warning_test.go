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

import "testing"

func TestExtractInstallationWarningStripsStepPrefix(t *testing.T) {
	message := "[configure_checkpoint] Warning: checkpoint runtime probe issue: timeout / 警告：checkpoint 运行时探测存在问题：timeout"

	warning := extractInstallationWarning(message)
	if warning == "" {
		t.Fatal("expected warning to be extracted")
	}
	if warning != "Warning: checkpoint runtime probe issue: timeout / 警告：checkpoint 运行时探测存在问题：timeout" {
		t.Fatalf("unexpected warning: %q", warning)
	}
}

func TestAppendInstallationWarningDeduplicates(t *testing.T) {
	status := &InstallationStatus{}

	appendInstallationWarning(status, "Warning: probe failed")
	appendInstallationWarning(status, "Warning: probe failed")
	appendInstallationWarning(status, "  ")

	if len(status.Warnings) != 1 {
		t.Fatalf("expected 1 warning after dedupe, got %d", len(status.Warnings))
	}
	if status.Warnings[0] != "Warning: probe failed" {
		t.Fatalf("unexpected warning content: %q", status.Warnings[0])
	}
}
