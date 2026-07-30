package extractor

import (
	"bufio"
	"bytes"
	"strings"
)

// CargoParser parses Cargo.lock for crate names/versions (Phase 4 / Tier 3 — best-effort).
type CargoParser struct{}

// NewCargoParser creates a Cargo.lock parser.
func NewCargoParser() *CargoParser { return &CargoParser{} }

// Parse finds Cargo.lock files and extracts [[package]] name/version entries.
func (p *CargoParser) Parse(fs *Filesystem) ([]Package, error) {
	const maxFiles = 5
	paths := fs.FindPathsBySuffix("Cargo.lock")
	out := make([]Package, 0)
	seen := make(map[string]struct{})

	for i, path := range paths {
		if i >= maxFiles {
			break
		}
		data, err := fs.ReadFile(path)
		if err != nil || len(data) == 0 {
			continue
		}
		for _, pkg := range parseCargoLockPackages(data) {
			key := pkg.Name + "@" + pkg.Version
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, pkg)
		}
	}
	return out, nil
}

func parseCargoLockPackages(data []byte) []Package {
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var pkgs []Package
	inPackage := false
	var name, version string

	flush := func() {
		if name == "" || version == "" {
			return
		}
		n := strings.TrimSpace(name)
		v := strings.TrimSpace(version)
		if n == "" || v == "" {
			return
		}
		pkgs = append(pkgs, Package{
			Name:       n,
			Version:    v,
			Type:       "cargo",
			PURL:       "pkg:cargo/" + strings.ReplaceAll(n, "/", "%2F") + "@" + v,
			Source:     "parsers",
			Confidence: "high",
		})
	}

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "[[package]]" {
			if inPackage {
				flush()
			}
			inPackage = true
			name, version = "", ""
			continue
		}
		if !inPackage {
			continue
		}
		if strings.HasPrefix(line, "name = ") {
			name = strings.Trim(strings.TrimPrefix(line, "name = "), `"`)
			continue
		}
		if strings.HasPrefix(line, "version = ") {
			version = strings.Trim(strings.TrimPrefix(line, "version = "), `"`)
			continue
		}
	}
	if inPackage {
		flush()
	}
	return pkgs
}
