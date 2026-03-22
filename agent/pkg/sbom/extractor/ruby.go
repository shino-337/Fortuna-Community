package extractor

import (
	"regexp"
	"strings"
)

// RubyGemsParser parses Bundler Gemfile.lock for gem name/version pairs (Tier 3).
type RubyGemsParser struct{}

// NewRubyGemsParser creates a Gemfile.lock parser.
func NewRubyGemsParser() *RubyGemsParser { return &RubyGemsParser{} }

var gemSpecLine = regexp.MustCompile(`^\s{4}([^\s(]+)\s+\(([^)]+)\)\s*$`)

// Parse finds Gemfile.lock via suffix walk and extracts specs from the GEM section.
func (p *RubyGemsParser) Parse(fs *Filesystem) ([]Package, error) {
	const maxFiles = 15
	paths := fs.FindPathsBySuffix("Gemfile.lock")
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
		for _, pkg := range parseGemfileLockSpecs(data) {
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

func parseGemfileLockSpecs(data []byte) []Package {
	lines := strings.Split(string(data), "\n")
	var out []Package
	inSpecs := false
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		t := strings.TrimSpace(line)
		if t == "specs:" || strings.HasSuffix(t, "specs:") {
			inSpecs = true
			continue
		}
		if !inSpecs {
			continue
		}
		if t == "" {
			continue
		}
		// Top-level section (no 4-space indent) ends specs block
		if !strings.HasPrefix(line, "    ") {
			break
		}
		m := gemSpecLine.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		name := strings.TrimSpace(m[1])
		ver := strings.TrimSpace(m[2])
		if name == "" || ver == "" {
			continue
		}
		purl := "pkg:gem/" + strings.ReplaceAll(name, "/", "%2F") + "@" + ver
		out = append(out, Package{
			Name:       name,
			Version:    ver,
			Type:       "gem",
			PURL:       purl,
			Source:     "parsers",
			Confidence: "high",
		})
	}
	return out
}
