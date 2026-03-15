package extractor

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fortuna/agent/pkg/sbom/signatures"
)

// DistrolessParser scans /bin, /usr/bin, /usr/lib (and siblings) for binaries and emits
// generic packages for distroless/minimal images (Finding #8.2 / C1).
// Only keeps real executables or whitelisted names; drops data files (.pl, charset .so, share/locale).
type DistrolessParser struct{}

// NewDistrolessParser creates a new distroless parser.
func NewDistrolessParser() *DistrolessParser {
	return &DistrolessParser{}
}

// Binary search prefixes (order matters only for dedup; we merge all).
var distrolessPrefixes = []string{"/bin/", "/usr/bin/", "/usr/lib/", "/usr/local/bin/", "/usr/local/sbin/"}

// libSoRe matches library sonames like libc.so.6, libssl.so.3 (allow); ISO-IR-197.so does not match.
var libSoRe = regexp.MustCompile(`^lib.+\.so(\.[0-9]+)*$`)

// distrolessSkipPath returns true if path/basename should be skipped (data files, not real binaries).
func distrolessSkipPath(path, base string) bool {
	// Path-based: drop locale/charset data
	if strings.Contains(path, "/share/locale/") || strings.Contains(path, "/share/i18n/") {
		return true
	}
	// Basename skips
	if base == "" || base == "." || base == ".." {
		return true
	}
	if base == "os-release" || base == "build.log" {
		return true
	}
	if strings.HasSuffix(base, ".log") {
		return true
	}
	// Perl/scripts and similar
	if strings.HasSuffix(base, ".pl") || strings.HasSuffix(base, ".pm") {
		return true
	}
	// .so: only keep lib*.so* (real libs); drop charset/data .so (e.g. ISO-IR-197.so)
	if strings.HasSuffix(base, ".so") || strings.HasSuffix(base, ".so.1") || strings.Contains(base, ".so.") {
		if !libSoRe.MatchString(base) {
			return true
		}
	}
	return false
}

// DistrolessSkipBasename is exposed for tests (legacy: now uses path for full check).
func DistrolessSkipBasename(base string) bool {
	return distrolessSkipPath("", base)
}

// Parse walks the virtual FS under common binary directories and creates one Package per
// unique basename (Name, Version=unknown, Type=generic, PURL, Source=distroless-heuristic).
// Skips os-release, build.log, and *.log so SBOM does not list those as components.
func (p *DistrolessParser) Parse(fs *Filesystem) ([]Package, error) {
	seen := make(map[string]bool)
	packages := make([]Package, 0)
	sig := signatures.LoadDistroless()

	for _, prefix := range distrolessPrefixes {
		for _, path := range fs.PathsUnder(prefix) {
			base := filepath.Base(path)
			if distrolessSkipPath(path, base) {
				continue
			}
			if seen[base] {
				continue
			}
			seen[base] = true
			version := "unknown"
			purl := "pkg:generic/" + base + "@" + version
			confidence := "low"

			// If this binary is in signature DB, use its suggested PURL/confidence.
			if sig != nil {
				if meta, ok := sig.Binaries[base]; ok {
					if meta.PURL != "" {
						purl = meta.PURL
					}
					if meta.Confidence != "" {
						confidence = meta.Confidence
					}
				}
			}

			packages = append(packages, Package{
				Name:       base,
				Version:    version,
				Type:       "generic",
				PURL:       purl,
				Source:     "distroless-heuristic",
				Confidence: confidence,
			})
		}
	}

	return packages, nil
}
