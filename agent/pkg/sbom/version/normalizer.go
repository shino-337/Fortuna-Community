package version

import (
	"fmt"
	"strings"
)

// NormalizeVersionForPURL updates a raw extracted version to the most OSV-friendly form,
// depending on package type/PURL.
func NormalizeVersionForPURL(purl, pkgType, rawVersion string) string {
	v := strings.TrimSpace(rawVersion)
	if v == "" || v == "unknown" || v == "(devel)" {
		return rawVersion
	}

	// Go versions: typically "v1.28.0" => "1.28.0"
	if pkgType == "go" || strings.Contains(purl, "pkg:golang/") {
		if len(v) > 1 && (v[0] == 'v' || v[0] == 'V') && v[1] >= '0' && v[1] <= '9' {
			v = v[1:]
		}
		// Strip go build metadata after '+' to reduce OSV exact-string misses.
		if idx := strings.Index(v, "+"); idx >= 0 {
			v = v[:idx]
		}
		return v
	}

	// Debian-like: "2:3.0.8-1~deb12u2+..." => "3.0.8-1"
	if pkgType == "deb" || strings.HasPrefix(purl, "pkg:deb/") {
		return normalizeDebVersion(v)
	}

	// APK: keep as-is (OSV data varies), but strip "v" prefix if any.
	if pkgType == "apk" || strings.HasPrefix(purl, "pkg:apk/") {
		if len(v) > 1 && (v[0] == 'v' || v[0] == 'V') && v[1] >= '0' && v[1] <= '9' {
			v = v[1:]
		}
		return v
	}

	// RPM: best-effort stripping epoch.
	if pkgType == "rpm" || strings.HasPrefix(purl, "pkg:rpm/") {
		if idx := strings.Index(v, ":"); idx >= 0 {
			v = v[idx+1:]
		}
		return v
	}

	return v
}

func normalizeDebVersion(v string) string {
	// Remove epoch if present.
	if idx := strings.Index(v, ":"); idx >= 0 {
		v = v[idx+1:]
	}
	// Strip Debian build suffixes that OSV usually doesn't include.
	if idx := strings.Index(v, "~"); idx >= 0 {
		v = v[:idx]
	}
	if idx := strings.Index(v, "+"); idx >= 0 {
		v = v[:idx]
	}
	return v
}

// UpdatePURLVersion replaces the version segment inside a PURL while preserving query parameters.
// Example:
//   pkg:deb/debian/openssl@3.0~deb12u2?arch=amd64
//   => pkg:deb/debian/openssl@3.0?arch=amd64
func UpdatePURLVersion(purl, newVersion string) string {
	p := strings.TrimSpace(purl)
	if p == "" {
		return purl
	}
	qidx := strings.Index(p, "?")
	body := p
	query := ""
	if qidx >= 0 {
		body = p[:qidx]
		query = p[qidx:]
	}

	at := strings.LastIndex(body, "@")
	if at < 0 {
		return p
	}

	prefix := body[:at]
	return prefix + "@" + strings.TrimSpace(newVersion) + query
}

func FormatConfidence(oldConf, newConf string) string {
	// low < medium < high
	rank := func(c string) int {
		switch strings.ToLower(strings.TrimSpace(c)) {
		case "low":
			return 1
		case "medium":
			return 2
		case "high":
			return 3
		default:
			return 0
		}
	}
	if rank(newConf) > rank(oldConf) {
		return newConf
	}
	return oldConf
}

func debugVersionHint(purl string, pkgType string, raw string) string {
	return fmt.Sprintf("purl=%s pkgType=%s raw=%s", purl, pkgType, raw)
}

