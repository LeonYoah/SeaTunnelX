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
