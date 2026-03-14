package extractor

import (
	"path/filepath"
)

// DistrolessParser scans /bin, /usr/bin, /usr/lib (and siblings) for binaries and emits
// generic packages for distroless/minimal images (Finding #8.2 / C1).
type DistrolessParser struct{}

// NewDistrolessParser creates a new distroless parser.
func NewDistrolessParser() *DistrolessParser {
	return &DistrolessParser{}
}

// Binary search prefixes (order matters only for dedup; we merge all).
var distrolessPrefixes = []string{"/bin/", "/usr/bin/", "/usr/lib/", "/usr/local/bin/", "/usr/local/sbin/"}

// Parse walks the virtual FS under common binary directories and creates one Package per
// unique basename (Name, Version=unknown, Type=generic, PURL, Source=distroless-heuristic).
func (p *DistrolessParser) Parse(fs *Filesystem) ([]Package, error) {
	seen := make(map[string]bool)
	packages := make([]Package, 0)

	for _, prefix := range distrolessPrefixes {
		for _, path := range fs.PathsUnder(prefix) {
			base := filepath.Base(path)
			if base == "" || base == "." || base == ".." {
				continue
			}
			if seen[base] {
				continue
			}
			seen[base] = true
			version := "unknown"
			purl := "pkg:generic/" + base + "@" + version
			packages = append(packages, Package{
				Name:       base,
				Version:    version,
				Type:       "generic",
				PURL:       purl,
				Source:     "distroless-heuristic",
				Confidence: "low",
			})
		}
	}

	return packages, nil
}
