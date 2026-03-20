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

import "testing"

func TestLoadOfficialDependencySeed_UsesReviewedVersionsForExactAndFallback(t *testing.T) {
	seedExact, err := loadOfficialDependencySeed("2.3.13")
	if err != nil {
		t.Fatalf("loadOfficialDependencySeed returned error: %v", err)
	}
	if seedExact.RequestedVersion != "2.3.13" {
		t.Fatalf("expected requested version 2.3.13, got %q", seedExact.RequestedVersion)
	}
	if !seedHasExactVersion(seedExact, "2.3.13") {
		t.Fatal("expected 2.3.13 to be treated as reviewed exact version")
	}
	if mode := defaultSeedResolutionMode(seedExact, "2.3.13"); mode != DependencyResolutionModeExact {
		t.Fatalf("expected exact mode for 2.3.13, got %q", mode)
	}
	if baseline := resolvedSeedBaselineVersion(seedExact, "2.3.13"); baseline != "2.3.13" {
		t.Fatalf("expected exact baseline 2.3.13, got %q", baseline)
	}

	seedFallback, err := loadOfficialDependencySeed("2.3.14")
	if err != nil {
		t.Fatalf("loadOfficialDependencySeed returned error: %v", err)
	}
	if mode := defaultSeedResolutionMode(seedFallback, "2.3.14"); mode != DependencyResolutionModeFallback {
		t.Fatalf("expected fallback mode for 2.3.14, got %q", mode)
	}
	if baseline := resolvedSeedBaselineVersion(seedFallback, "2.3.14"); baseline != "2.3.13" {
		t.Fatalf("expected fallback baseline 2.3.13, got %q", baseline)
	}
}

func TestResolvedSeedProfileBaselineVersion_PrefersSeedReviewedVersionOverLegacyTemplateMarker(t *testing.T) {
	seed := &officialDependencySeedFile{
		TemplateVersion:  "2.3.12",
		ReviewedVersions: []string{"2.3.12", "2.3.13"},
	}
	spec := officialDependencySeedProfileSpec{
		BaselineVersionUsed: "2.3.12",
	}

	baseline := resolvedSeedProfileBaselineVersion(spec, seed, "2.3.13")
	if baseline != "2.3.13" {
		t.Fatalf("expected profile baseline 2.3.13 for exact reviewed version, got %q", baseline)
	}
}
