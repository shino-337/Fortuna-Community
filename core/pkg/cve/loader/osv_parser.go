package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fortuna/core/pkg/cve/cvss"
)

// OSVVulnerability represents the OSV.dev JSON schema 1.7.3
type OSVVulnerability struct {
	SchemaVersion string   `json:"schema_version"`
	ID            string   `json:"id"`
	Published     string   `json:"published"`
	Modified      string   `json:"modified"`
	Withdrawn     string   `json:"withdrawn,omitempty"`
	Summary       string   `json:"summary,omitempty"`
	Details       string   `json:"details,omitempty"`
	Aliases       []string `json:"aliases,omitempty"`
	Upstream      []string `json:"upstream,omitempty"`
	Related       []string `json:"related,omitempty"`

	Affected []OSVAffected `json:"affected,omitempty"`

	Severity []OSVSeverity `json:"severity,omitempty"`

	References []OSVReference `json:"references,omitempty"`

	DatabaseSpecific map[string]interface{} `json:"database_specific,omitempty"`
}

// OSVAffected represents affected packages and versions
type OSVAffected struct {
	Package OSVPackage `json:"package,omitempty"`

	Ranges []OSVRange `json:"ranges,omitempty"`

	Versions []string `json:"versions,omitempty"`

	Severity []OSVSeverity `json:"severity,omitempty"`

	EcosystemSpecific map[string]interface{} `json:"ecosystem_specific,omitempty"`
	DatabaseSpecific  map[string]interface{} `json:"database_specific,omitempty"`
}

// OSVPackage represents package identification
type OSVPackage struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
	PURL      string `json:"purl,omitempty"`
}

// OSVRange represents version ranges
type OSVRange struct {
	Type   string     `json:"type"` // SEMVER, ECOSYSTEM, GIT
	Repo   string     `json:"repo,omitempty"`
	Events []OSVEvent `json:"events,omitempty"`
}

// OSVEvent represents a version event
type OSVEvent struct {
	Introduced   string `json:"introduced,omitempty"`
	Fixed        string `json:"fixed,omitempty"`
	LastAffected string `json:"last_affected,omitempty"`
	Limit        string `json:"limit,omitempty"`
}

// OSVSeverity represents severity information
type OSVSeverity struct {
	Type  string `json:"type"`  // CVSS_V2, CVSS_V3, CVSS_V4, Ubuntu
	Score string `json:"score"` // CVSS vector, or the rating for Ubuntu
}

// OSVReference represents external references
type OSVReference struct {
	Type string `json:"type"` // ADVISORY, ARTICLE, FIX, etc.
	URL  string `json:"url"`
}

// ParseFile parses a single OSV.dev JSON file
func ParseFile(path string) (*OSVVulnerability, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return ParseBytes(data, path)
}

// ParseBytes parses one OSV advisory; name is only used in errors.
func ParseBytes(data []byte, name string) (*OSVVulnerability, error) {
	var vuln OSVVulnerability
	if err := json.Unmarshal(data, &vuln); err != nil {
		return nil, fmt.Errorf("failed to parse JSON in %s: %w", name, err)
	}

	// Validate required fields
	if vuln.ID == "" {
		return nil, fmt.Errorf("missing ID in %s", name)
	}

	return &vuln, nil
}

// maxExplicitVersionsPerPackage bounds the ranges created from an OSV `versions` list.
const maxExplicitVersionsPerPackage = 2000

// OSVInterval is one affected interval of an OSV range.
type OSVInterval struct {
	Introduced   string // "0" when affected from the first version
	Fixed        string
	LastAffected string
}

// OSVRangeIntervals turns OSV range events into intervals. Events are applied in order: an
// `introduced` opens an interval and the next `fixed` or `last_affected` closes it. A trailing
// open interval (no fix yet) is returned with Fixed and LastAffected empty.
func OSVRangeIntervals(events []OSVEvent) []OSVInterval {
	var out []OSVInterval
	open := false
	introduced := ""
	for _, ev := range events {
		switch {
		case strings.TrimSpace(ev.Introduced) != "":
			if open {
				// Two introduced events in a row: the first interval never closed.
				out = append(out, OSVInterval{Introduced: introduced})
			}
			introduced = strings.TrimSpace(ev.Introduced)
			open = true
		case strings.TrimSpace(ev.Fixed) != "":
			out = append(out, OSVInterval{Introduced: introducedOrZero(introduced, open), Fixed: strings.TrimSpace(ev.Fixed)})
			open, introduced = false, ""
		case strings.TrimSpace(ev.LastAffected) != "":
			out = append(out, OSVInterval{Introduced: introducedOrZero(introduced, open), LastAffected: strings.TrimSpace(ev.LastAffected)})
			open, introduced = false, ""
		}
	}
	if open {
		out = append(out, OSVInterval{Introduced: introduced})
	}
	return out
}

func introducedOrZero(introduced string, open bool) string {
	if open && introduced != "" {
		return introduced
	}
	return "0"
}

func uniqueVersions(versions []string, max int) []string {
	seen := make(map[string]struct{}, len(versions))
	out := make([]string, 0, len(versions))
	for _, v := range versions {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
		if len(out) >= max {
			break
		}
	}
	return out
}

// OSVEcosystemRelease returns the distro release an OSV ecosystem string is scoped to, normalized
// the way SBOM components report it: "Debian:12" → "12", "Alpine:v3.20" → "3.20",
// "Ubuntu:22.04:LTS" and "Ubuntu:Pro:22.04:LTS" → "22.04", "Rocky Linux:8" → "8",
// "AlmaLinux:9" → "9", "Red Hat:enterprise_linux:9::appstream", "Red Hat:rhel_eus:9.4::baseos" and
// "Red Hat:openshift:4.14::el9" → "9". Ecosystems without a release, or with a format we do not
// parse (Red Hat products not tied to a RHEL major), return "".
func OSVEcosystemRelease(ecosystem string) string {
	e := strings.TrimSpace(ecosystem)
	name, rest, ok := strings.Cut(e, ":")
	if !ok {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debian", "rocky linux", "almalinux":
		return NormalizeDistroRelease(strings.ToLower(name), rest)
	case "alpine":
		return NormalizeDistroRelease("alpine", rest)
	case "red hat":
		return redHatRelease(rest)
	case "ubuntu":
		for _, part := range strings.Split(rest, ":") {
			part = strings.TrimSpace(part)
			if part != "" && part[0] >= '0' && part[0] <= '9' {
				return NormalizeDistroRelease("ubuntu", part)
			}
		}
	}
	return ""
}

// redHatRelease returns the RHEL major of a Red Hat CPE-style stream "product:version::variant":
// the "elN" variant of layered products, else the version major of RHEL streams
// (enterprise_linux, enterprise_linux_eus, rhel_eus, rhel_aus, rhel_tus, rhel_e4s).
func redHatRelease(stream string) string {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(stream)), ":")
	if len(parts) < 2 {
		return ""
	}
	if v := parts[len(parts)-1]; strings.HasPrefix(v, "el") && isDigits(v[2:]) {
		return v[2:]
	}
	product := parts[0]
	if product != "enterprise_linux" && product != "enterprise_linux_eus" && !strings.HasPrefix(product, "rhel_") {
		return ""
	}
	major, _, _ := strings.Cut(parts[1], ".")
	if !isDigits(major) {
		return ""
	}
	return major
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// NormalizeDistroRelease reduces a distro version to the granularity advisories are published
// at: the major version for Debian, Rocky, Alma and RHEL ("12.5" → "12"), major.minor for Alpine and
// Ubuntu ("v3.20.3" → "3.20", "22.04" → "22.04"). Unknown distros return the trimmed version.
func NormalizeDistroRelease(distro, version string) string {
	v := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(version)), "v")
	if v == "" {
		return ""
	}
	parts := strings.Split(v, ".")
	switch strings.ToLower(strings.TrimSpace(distro)) {
	case "debian", "rocky", "rocky linux", "alma", "almalinux", "redhat", "rhel":
		return parts[0]
	case "alpine", "ubuntu":
		if len(parts) >= 2 {
			return parts[0] + "." + parts[1]
		}
		return parts[0]
	}
	return v
}

// AdvisoryRating returns the qualitative rating the advisory's own source gave it: GitHub's
// database_specific.severity, the Ubuntu priority, or the highest Debian urgency of its
// packages. "" when the source gave none.
func AdvisoryRating(osv *OSVVulnerability) string {
	if osv == nil {
		return ""
	}
	if s, ok := osv.DatabaseSpecific["severity"].(string); ok {
		if r := cvss.NormalizeRating(s); r != "" {
			return r
		}
	}
	if r := ratingOf(osv.Severity); r != "" {
		return r
	}
	best := ""
	for i := range osv.Affected {
		if r := AffectedRating(&osv.Affected[i]); cvss.Rank(r) > cvss.Rank(best) {
			best = r
		}
	}
	return best
}

// AffectedRating returns the vendor rating of one affected package: Debian's urgency or an
// Ubuntu priority attached to the package.
func AffectedRating(a *OSVAffected) string {
	if a == nil {
		return ""
	}
	if u, ok := a.EcosystemSpecific["urgency"].(string); ok {
		if r := cvss.NormalizeRating(u); r != "" {
			return r
		}
	}
	return ratingOf(a.Severity)
}

func ratingOf(severities []OSVSeverity) string {
	for _, s := range severities {
		if strings.EqualFold(s.Type, "Ubuntu") {
			if r := cvss.NormalizeRating(s.Score); r != "" {
				return r
			}
		}
	}
	return ""
}

// buildReferencesJSON converts OSV references to JSON string
func buildReferencesJSON(refs []OSVReference) string {
	if len(refs) == 0 {
		return "[]"
	}

	type Ref struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	}

	refList := make([]Ref, len(refs))
	for i, r := range refs {
		refList[i] = Ref{Type: r.Type, URL: r.URL}
	}

	data, _ := json.Marshal(refList)
	return string(data)
}

// extractCWEIDs extracts CWE IDs from database_specific field
func extractCWEIDs(dbSpecific map[string]interface{}) []string {
	if dbSpecific == nil {
		return nil
	}

	// Try to extract cwe_ids field
	if cweField, ok := dbSpecific["cwe_ids"]; ok {
		switch v := cweField.(type) {
		case []interface{}:
			cweIDs := make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					cweIDs = append(cweIDs, str)
				}
			}
			return cweIDs
		case []string:
			return v
		}
	}

	return nil
}

// normalizeEcosystem normalizes ecosystem names for consistency
func normalizeEcosystem(ecosystem string) string {
	// Normalize to lowercase
	ecosystem = strings.ToLower(ecosystem)

	// Handle common OSV ecosystem variants that include distro versions, e.g. "debian:10"
	if strings.HasPrefix(ecosystem, "debian:") {
		return "debian"
	}
	if strings.HasPrefix(ecosystem, "ubuntu:") {
		return "ubuntu"
	}
	if strings.HasPrefix(ecosystem, "alpine:") {
		return "alpine"
	}

	// OSV names these "Rocky Linux:8" / "AlmaLinux:9"; the matcher queries "rocky" / "alma".
	if strings.HasPrefix(ecosystem, "rocky linux") {
		return "rocky"
	}
	if strings.HasPrefix(ecosystem, "almalinux") {
		return "alma"
	}
	// "Red Hat:enterprise_linux:9::appstream" and other Red Hat product streams.
	if ecosystem == "red hat" || strings.HasPrefix(ecosystem, "red hat:") {
		return "redhat"
	}
	// Map variations to standard names
	switch ecosystem {
	case "debian", "debian:*":
		return "debian"
	case "ubuntu":
		return "ubuntu"
	case "alpine":
		return "alpine"
	case "pypi":
		return "pypi"
	case "npm":
		return "npm"
	case "go", "golang":
		return "go"
	case "maven":
		return "maven"
	case "nuget":
		return "nuget"
	case "crates.io", "cargo":
		return "cargo"
	case "rubygems":
		return "rubygems"
	default:
		return ecosystem
	}
}

// sanitizeUTF8 replaces invalid UTF-8 sequences with Unicode replacement character
func sanitizeUTF8(s string) string {
	// Convert to runes and back - this replaces invalid UTF-8 with U+FFFD
	return strings.ToValidUTF8(s, "\uFFFD")
}

// Stats tracks parsing statistics
type Stats struct {
	TotalFiles           int
	SuccessfullyParsed   int
	FailedToParse        int
	CVEsCreated          int
	PackageVulnsCreated  int
	SkippedNoPackageInfo int
}
