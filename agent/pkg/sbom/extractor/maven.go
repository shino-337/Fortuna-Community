package extractor

import (
	"encoding/xml"
	"regexp"
	"strings"
)

var (
	mavenGroupRe    = regexp.MustCompile(`<groupId>\s*([^<]+?)\s*</groupId>`)
	mavenArtifactRe = regexp.MustCompile(`<artifactId>\s*([^<]+?)\s*</artifactId>`)
	mavenVersionRe  = regexp.MustCompile(`<version>\s*([^<]+?)\s*</version>`)
)

// MavenParser extracts coordinates from pom.xml files (Phase 4 / Tier 3 — best-effort).
type MavenParser struct{}

// NewMavenParser creates a Maven POM parser.
func NewMavenParser() *MavenParser { return &MavenParser{} }

// mavenProject matches the root element of a Maven POM (default namespace is ignored for fields).
type mavenProject struct {
	XMLName    xml.Name `xml:"project"`
	GroupID    string   `xml:"groupId"`
	ArtifactID string   `xml:"artifactId"`
	Version    string   `xml:"version"`
}

// Parse discovers pom.xml via suffix walk and emits one package per unique GAV.
func (p *MavenParser) Parse(fs *Filesystem) ([]Package, error) {
	const maxFiles = 30
	paths := fs.FindPathsBySuffix("pom.xml")
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
		var proj mavenProject
		_ = xml.Unmarshal(data, &proj)
		artifact := strings.TrimSpace(proj.ArtifactID)
		group := strings.TrimSpace(proj.GroupID)
		version := strings.TrimSpace(proj.Version)
		if artifact == "" {
			if m := mavenArtifactRe.FindSubmatch(data); len(m) > 1 {
				artifact = strings.TrimSpace(string(m[1]))
			}
		}
		if group == "" {
			if m := mavenGroupRe.FindSubmatch(data); len(m) > 1 {
				g := strings.TrimSpace(string(m[1]))
				if !strings.Contains(g, "${") {
					group = g
				}
			}
		}
		if version == "" {
			if m := mavenVersionRe.FindSubmatch(data); len(m) > 1 {
				v := strings.TrimSpace(string(m[1]))
				if !strings.Contains(v, "${") {
					version = v
				}
			}
		}
		if artifact == "" {
			continue
		}
		if version == "" {
			version = "unknown"
		}
		key := group + ":" + artifact + "@" + version
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		name := artifact
		if group != "" {
			name = group + ":" + artifact
		}
		purl := "pkg:maven/"
		if group != "" {
			purl += strings.ReplaceAll(group, "/", "!") + "/" + artifact + "@" + version
		} else {
			purl += artifact + "@" + version
		}

		out = append(out, Package{
			Name:       name,
			Version:    version,
			Type:       "maven",
			PURL:       purl,
			Source:     "parsers",
			Confidence: "medium",
		})
	}
	return out, nil
}
