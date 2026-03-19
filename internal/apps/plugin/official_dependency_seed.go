package plugin

import (
	"context"
	"crypto/sha1"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

//go:embed seed/*.json
var officialDependencySeedFS embed.FS

type officialDependencySeedFile struct {
	SeatunnelVersion   string                              `json:"seatunnel_version"`
	TemplateVersion    string                              `json:"template_version"`
	Notes              []string                            `json:"notes"`
	HiddenPlugins      []string                            `json:"hidden_plugins"`
	Catalog            []officialDependencySeedCatalog     `json:"catalog"`
	DefaultNotRequired []string                            `json:"default_not_required"`
	Profiles           []officialDependencySeedProfileSpec `json:"profiles"`
}

type officialDependencySeedCatalog struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	ArtifactID  string `json:"artifact_id"`
	GroupID     string `json:"group_id"`
	DocURL      string `json:"doc_url"`
}

type officialDependencySeedProfileSpec struct {
	PluginName               string                                  `json:"plugin_name"`
	ArtifactID               string                                  `json:"artifact_id"`
	ProfileKey               string                                  `json:"profile_key"`
	ProfileName              string                                  `json:"profile_name"`
	EngineScope              string                                  `json:"engine_scope"`
	TargetDir                string                                  `json:"target_dir"`
	AppliesTo                string                                  `json:"applies_to"`
	IncludeVersions          []string                                `json:"include_versions"`
	ExcludeVersions          []string                                `json:"exclude_versions"`
	DocSlug                  string                                  `json:"doc_slug"`
	DocSourceURL             string                                  `json:"doc_source_url"`
	Confidence               string                                  `json:"confidence"`
	IsDefault                bool                                    `json:"is_default"`
	NoAdditionalDependencies bool                                    `json:"no_additional_dependencies"`
	ResolutionMode           DependencyResolutionMode                `json:"resolution_mode"`
	BaselineVersionUsed      string                                  `json:"baseline_version_used"`
	Items                    []officialDependencySeedProfileItemSpec `json:"items"`
}

type officialDependencySeedProfileItemSpec struct {
	GroupID    string `json:"group_id"`
	ArtifactID string `json:"artifact_id"`
	Version    string `json:"version"`
	TargetDir  string `json:"target_dir"`
	Required   bool   `json:"required"`
	SourceURL  string `json:"source_url"`
	Note       string `json:"note"`
}

func (s *Service) ensureBundledSeedLoaded(ctx context.Context, version string) {
	if version == "" || s.repo == nil {
		return
	}
	s.seedLoadedMu.RLock()
	if s.seedLoadedVersions[version] {
		s.seedLoadedMu.RUnlock()
		return
	}
	s.seedLoadedMu.RUnlock()

	seed, err := loadOfficialDependencySeed(version)
	if err != nil || seed == nil {
		return
	}

	if err := s.loadOfficialDependencySeedIntoDB(ctx, seed); err != nil {
		return
	}

	s.seedLoadedMu.Lock()
	s.seedLoadedVersions[version] = true
	s.seedLoadedMu.Unlock()
}

func loadOfficialDependencySeed(version string) (*officialDependencySeedFile, error) {
	path := filepath.Join("seed", fmt.Sprintf("seatunnel-%s.json", version))
	content, err := officialDependencySeedFS.ReadFile(path)
	if err != nil {
		fallbackPath := filepath.Join("seed", "seatunnel-2.3.12.json")
		content, err = officialDependencySeedFS.ReadFile(fallbackPath)
		if err != nil {
			return nil, err
		}
	}
	var seed officialDependencySeedFile
	if err := json.Unmarshal(content, &seed); err != nil {
		return nil, err
	}
	if strings.TrimSpace(seed.TemplateVersion) == "" {
		seed.TemplateVersion = seed.SeatunnelVersion
	}
	seed.SeatunnelVersion = version
	return &seed, nil
}

func (s *Service) loadOfficialDependencySeedIntoDB(ctx context.Context, seed *officialDependencySeedFile) error {
	catalogByName := make(map[string]officialDependencySeedCatalog, len(seed.Catalog))
	for _, item := range seed.Catalog {
		catalogByName[item.Name] = item
	}

	for _, item := range seed.DefaultNotRequired {
		catalog, ok := catalogByName[item]
		if !ok {
			continue
		}
		profile := PluginDependencyProfile{
			SeatunnelVersion:         seed.SeatunnelVersion,
			PluginName:               item,
			ArtifactID:               catalog.ArtifactID,
			ProfileKey:               "default",
			ProfileName:              "Default",
			EngineScope:              "zeta",
			SourceKind:               PluginDependencyProfileSourceOfficialSeed,
			BaselineVersionUsed:      seed.TemplateVersion,
			ResolutionMode:           defaultSeedResolutionMode(seed.TemplateVersion, seed.SeatunnelVersion),
			TargetDir:                defaultPluginDependencyTargetDir(seed.SeatunnelVersion, catalog.ArtifactID),
			AppliesTo:                "*",
			DocSourceURL:             catalog.DocURL,
			Confidence:               "manual",
			IsDefault:                true,
			NoAdditionalDependencies: true,
			ContentHash:              hashOfficialSeedValue(seed.SeatunnelVersion, item, "default", "not_required"),
			Items:                    []PluginDependencyProfileItem{},
		}
		if err := s.repo.UpsertDependencyProfile(ctx, &profile); err != nil {
			return err
		}
	}

	for _, spec := range seed.Profiles {
		items := make([]PluginDependencyProfileItem, 0, len(spec.Items))
		for _, item := range spec.Items {
			targetDir := strings.TrimSpace(item.TargetDir)
			if targetDir == "" {
				targetDir = firstNonEmpty(spec.TargetDir, defaultPluginDependencyTargetDir(seed.SeatunnelVersion, spec.ArtifactID))
			}
			targetDir = adjustSeedTargetDirForVersion(targetDir, seed.SeatunnelVersion)
			items = append(items, PluginDependencyProfileItem{
				GroupID:    item.GroupID,
				ArtifactID: item.ArtifactID,
				Version:    item.Version,
				TargetDir:  targetDir,
				Required:   item.Required,
				SourceURL:  item.SourceURL,
				Note:       item.Note,
			})
		}
		if !seedProfileAppliesToRequestedVersion(spec, seed.SeatunnelVersion) {
			continue
		}
		profile := PluginDependencyProfile{
			SeatunnelVersion:         seed.SeatunnelVersion,
			PluginName:               spec.PluginName,
			ArtifactID:               spec.ArtifactID,
			ProfileKey:               spec.ProfileKey,
			ProfileName:              firstNonEmpty(spec.ProfileName, spec.ProfileKey),
			EngineScope:              spec.EngineScope,
			SourceKind:               PluginDependencyProfileSourceOfficialSeed,
			BaselineVersionUsed:      firstNonEmpty(spec.BaselineVersionUsed, seed.TemplateVersion),
			ResolutionMode:           seedProfileResolutionMode(spec, seed.TemplateVersion, seed.SeatunnelVersion),
			TargetDir:                adjustSeedTargetDirForVersion(firstNonEmpty(spec.TargetDir, defaultPluginDependencyTargetDir(seed.SeatunnelVersion, spec.ArtifactID)), seed.SeatunnelVersion),
			AppliesTo:                firstNonEmpty(spec.AppliesTo, "*"),
			IncludeVersions:          strings.Join(spec.IncludeVersions, ","),
			ExcludedVersions:         strings.Join(spec.ExcludeVersions, ","),
			DocSlug:                  spec.DocSlug,
			DocSourceURL:             spec.DocSourceURL,
			Confidence:               spec.Confidence,
			IsDefault:                spec.IsDefault,
			NoAdditionalDependencies: spec.NoAdditionalDependencies,
			ContentHash:              hashOfficialSeedValue(seed.SeatunnelVersion, spec.PluginName, spec.ProfileKey, spec.DocSourceURL),
			Items:                    items,
		}
		if err := s.repo.UpsertDependencyProfile(ctx, &profile); err != nil {
			return err
		}
	}

	return nil
}

func defaultSeedResolutionMode(templateVersion, requestedVersion string) DependencyResolutionMode {
	if strings.TrimSpace(templateVersion) == strings.TrimSpace(requestedVersion) {
		return DependencyResolutionModeExact
	}
	return DependencyResolutionModeFallback
}

func seedProfileResolutionMode(spec officialDependencySeedProfileSpec, templateVersion, requestedVersion string) DependencyResolutionMode {
	if spec.ResolutionMode != "" && strings.TrimSpace(templateVersion) == strings.TrimSpace(requestedVersion) {
		return spec.ResolutionMode
	}
	return defaultSeedResolutionMode(templateVersion, requestedVersion)
}

func seedProfileAppliesToRequestedVersion(spec officialDependencySeedProfileSpec, requestedVersion string) bool {
	if len(spec.ExcludeVersions) > 0 {
		for _, item := range spec.ExcludeVersions {
			if strings.TrimSpace(item) == strings.TrimSpace(requestedVersion) {
				return false
			}
		}
	}
	if len(spec.IncludeVersions) > 0 {
		for _, item := range spec.IncludeVersions {
			if strings.TrimSpace(item) == strings.TrimSpace(requestedVersion) {
				return true
			}
		}
		return false
	}
	appliesTo := strings.TrimSpace(spec.AppliesTo)
	return appliesTo == "" || appliesTo == "*" || appliesTo == requestedVersion
}

func adjustSeedTargetDirForVersion(targetDir, requestedVersion string) string {
	normalized := strings.TrimSpace(targetDir)
	if normalized == "" {
		return normalized
	}
	if !supportsConnectorIsolatedDependency(requestedVersion) && strings.HasPrefix(normalized, "plugins/") {
		return "lib"
	}
	return normalized
}

func hiddenPluginSetFromSeed(seed *officialDependencySeedFile) map[string]struct{} {
	result := make(map[string]struct{}, len(seed.HiddenPlugins))
	for _, item := range seed.HiddenPlugins {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		result[name] = struct{}{}
	}
	return result
}

func hashOfficialSeedValue(parts ...string) string {
	h := sha1.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
