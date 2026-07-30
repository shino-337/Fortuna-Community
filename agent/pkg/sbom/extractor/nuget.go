package extractor

import (
	"encoding/json"
	"strings"
)

// NuGetParser parses NuGet packages.lock.json (SDK lock files; Tier 3).
type NuGetParser struct{}

// NewNuGetParser creates a NuGet lockfile parser.
func NewNuGetParser() *NuGetParser { return &NuGetParser{} }

// Parse discovers packages.lock.json and project.assets.json (restore graph).
func (p *NuGetParser) Parse(fs *Filesystem) ([]Package, error) {
	const maxFiles = 15
	out := make([]Package, 0)
	seen := make(map[string]struct{})

	add := func(pkgs []Package) {
		for _, pkg := range pkgs {
			key := strings.ToLower(pkg.Name) + "@" + pkg.Version
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, pkg)
		}
	}

	pathsLock := fs.FindPathsBySuffix("packages.lock.json")
	for i, path := range pathsLock {
		if i >= maxFiles {
			break
		}
		data, err := fs.ReadFile(path)
		if err != nil || len(data) == 0 {
			continue
		}
		add(parseNuGetPackagesLock(data))
	}

	pathsAssets := fs.FindPathsBySuffix("project.assets.json")
	for i, path := range pathsAssets {
		if i >= maxFiles {
			break
		}
		data, err := fs.ReadFile(path)
		if err != nil || len(data) == 0 {
			continue
		}
		add(parseNuGetProjectAssets(data))
	}

	return out, nil
}

func parseNuGetPackagesLock(data []byte) []Package {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	rawDeps, ok := root["dependencies"]
	if !ok || len(rawDeps) == 0 {
		return nil
	}

	var tfMap map[string]json.RawMessage
	if err := json.Unmarshal(rawDeps, &tfMap); err != nil {
		return nil
	}

	var out []Package
	for tfName, rawPkgs := range tfMap {
		_ = tfName
		var pkgMap map[string]json.RawMessage
		if err := json.Unmarshal(rawPkgs, &pkgMap); err != nil {
			continue
		}
		for pkgName, rawMeta := range pkgMap {
			var meta struct {
				Resolved string `json:"resolved"`
				Type     string `json:"type"`
			}
			if err := json.Unmarshal(rawMeta, &meta); err != nil {
				continue
			}
			ver := strings.TrimSpace(meta.Resolved)
			if ver == "" {
				continue
			}
			name := strings.TrimSpace(pkgName)
			if name == "" {
				continue
			}
			// pkg:nuget/packagename@version
			purl := "pkg:nuget/" + strings.ToLower(name) + "@" + ver
			out = append(out, Package{
				Name:       name,
				Version:    ver,
				Type:       "nuget",
				PURL:       purl,
				Source:     "parsers",
				Confidence: "high",
			})
		}
	}
	return out
}

// parseNuGetProjectAssets walks targets[tf]["PackageId/Version"] from project.assets.json (NuGet restore v3).
func parseNuGetProjectAssets(data []byte) []Package {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	rawTargets, ok := root["targets"]
	if !ok || len(rawTargets) == 0 {
		return nil
	}
	var tfMap map[string]json.RawMessage
	if err := json.Unmarshal(rawTargets, &tfMap); err != nil {
		return nil
	}
	var out []Package
	for _, rawPkgs := range tfMap {
		var pkgMap map[string]json.RawMessage
		if err := json.Unmarshal(rawPkgs, &pkgMap); err != nil {
			continue
		}
		for idKey := range pkgMap {
			name, ver := splitNuGetPackageKey(idKey)
			if name == "" || ver == "" {
				continue
			}
			purl := "pkg:nuget/" + strings.ToLower(name) + "@" + ver
			out = append(out, Package{
				Name:       name,
				Version:    ver,
				Type:       "nuget",
				PURL:       purl,
				Source:     "parsers",
				Confidence: "medium",
			})
		}
	}
	return out
}

// splitNuGetPackageKey splits "Some.Package/1.2.3" → name, version (last '/').
func splitNuGetPackageKey(idKey string) (name, ver string) {
	idKey = strings.TrimSpace(idKey)
	if idKey == "" {
		return "", ""
	}
	i := strings.LastIndex(idKey, "/")
	if i <= 0 || i >= len(idKey)-1 {
		return "", ""
	}
	return strings.TrimSpace(idKey[:i]), strings.TrimSpace(idKey[i+1:])
}
