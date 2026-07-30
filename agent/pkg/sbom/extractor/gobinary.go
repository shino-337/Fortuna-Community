package extractor

import (
	"bytes"
	"debug/buildinfo"
	"fmt"
	"strings"
)

// GoBinaryParser parses Go modules from compiled Go binaries by reading buildinfo.
// It is especially useful for distroless/control-plane images where go.mod/go.sum
// are not present but binaries contain module metadata.
type GoBinaryParser struct{}

// NewGoBinaryParser creates a new Go binary parser.
func NewGoBinaryParser() *GoBinaryParser {
	return &GoBinaryParser{}
}

// Parse scans common binary locations for Go binaries and extracts module path+version.
// It returns Go packages (Type="go") and a main binary package (Type="go-binary") when available.
func (p *GoBinaryParser) Parse(fs *Filesystem) ([]Package, error) {
	pkgs := make([]Package, 0)

	// Candidate directories where compiled binaries usually live.
	// Extended for distroless/control-plane images (P2-1 design).
	candidates := []string{
		"/bin",
		"/usr/bin",
		"/usr/local/bin",
		"/usr/sbin",
		"/sbin",
		"/app",
		"/",
	}

	const maxBinarySize = 200 * 1024 * 1024 // 200MB
	const maxFilesScanned = 2000
	seen := make(map[string]bool)
	filesScanned := 0

	for _, dir := range candidates {
		paths := fs.PathsUnder(dir)
		for _, path := range paths {
			if filesScanned >= maxFilesScanned {
				return pkgs, nil
			}

			// Skip obviously non-binary files by extension (best-effort).
			if strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".py") || strings.HasSuffix(path, ".pl") {
				continue
			}
			content, err := fs.ReadFile(path)
			if err != nil || len(content) == 0 {
				continue
			}
			filesScanned++

			// Guardrail: very large files are unlikely to be Go binaries and are expensive to parse.
			if len(content) > maxBinarySize {
				continue
			}

			r := bytes.NewReader(content)
			bi, err := buildinfo.Read(r)
			if err != nil || bi == nil {
				continue
			}

			// Main module (binary itself)
			mainPath := strings.TrimSpace(bi.Main.Path)
			mainVersion := strings.TrimSpace(bi.Main.Version)
			if mainPath != "" && mainVersion != "" {
				key := "go-binary:" + mainPath + "@" + mainVersion
				if !seen[key] {
					seen[key] = true
					pkgs = append(pkgs, Package{
						Name:       mainPath,
						Version:    mainVersion,
						Type:       "go-binary",
						PURL:       toGoPURL(mainPath, mainVersion),
						Source:     "gobinary-main",
						Confidence: "high",
					})
				}
			}

			// Dependencies
			for _, dep := range bi.Deps {
				if dep == nil {
					continue
				}
				path := strings.TrimSpace(dep.Path)
				version := strings.TrimSpace(dep.Version)
				if path == "" || version == "" {
					continue
				}
				conf := "high"
				if isPseudoVersion(version) {
					conf = "medium"
				}
				key := "go:" + path + "@" + version
				if seen[key] {
					continue
				}
				seen[key] = true
				pkgs = append(pkgs, Package{
					Name:       path,
					Version:    version,
					Type:       "go",
					PURL:       toGoPURL(path, version),
					Source:     "gobinary",
					Confidence: conf,
				})
			}
		}
	}

	return pkgs, nil
}

// ToolchainVersion returns the Go toolchain version (buildinfo.GoVersion, e.g. go1.22.5) from
// the first readable Go binary on the filesystem. Used for SBOM-level stdlib CVE matching in Core.
func (p *GoBinaryParser) ToolchainVersion(fs *Filesystem) string {
	candidates := []string{
		"/bin",
		"/usr/bin",
		"/usr/local/bin",
		"/usr/sbin",
		"/sbin",
		"/app",
		"/",
	}
	const maxBinarySize = 200 * 1024 * 1024
	const maxFilesScanned = 2000
	filesScanned := 0

	for _, dir := range candidates {
		paths := fs.PathsUnder(dir)
		for _, path := range paths {
			if filesScanned >= maxFilesScanned {
				return ""
			}
			if strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".py") || strings.HasSuffix(path, ".pl") {
				continue
			}
			content, err := fs.ReadFile(path)
			if err != nil || len(content) == 0 {
				continue
			}
			filesScanned++
			if len(content) > maxBinarySize {
				continue
			}
			r := bytes.NewReader(content)
			bi, err := buildinfo.Read(r)
			if err != nil || bi == nil {
				continue
			}
			gv := strings.TrimSpace(bi.GoVersion)
			if gv != "" {
				return gv
			}
		}
	}
	return ""
}

func toGoPURL(name, version string) string {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	if name == "" || version == "" {
		return ""
	}
	return fmt.Sprintf("pkg:go/%s@%s", name, version)
}

// isPseudoVersion implements a lightweight check for Go pseudo-versions,
// e.g. v0.0.0-20230912-abcdef or v1.2.3-0.20230912-abcdef.
func isPseudoVersion(v string) bool {
	v = strings.TrimSpace(v)
	// Best-effort: look for a date-like segment and commit hash suffix.
	// Common patterns:
	// - v0.0.0-20230912-abcdef
	// - v1.2.3-0.20230912-abcdef
	if strings.Contains(v, "-0.20") {
		return true
	}
	if strings.Contains(v, "-20") && strings.Count(v, "-") >= 2 {
		return true
	}
	return false
}

