package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
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
	Type  string `json:"type"`  // CVSS_V2, CVSS_V3
	Score string `json:"score"` // CVSS vector string
}

// OSVReference represents external references
type OSVReference struct {
	Type string `json:"type"` // ADVISORY, ARTICLE, FIX, etc.
	URL  string `json:"url"`
}

// ParsedCVE represents a parsed CVE ready for database insertion
type ParsedCVE struct {
	CVEID            string
	CVSSScore        float64
	CVSSVector       string
	CVSSVersion      string
	Severity         string
	Title            string
	Description      string
	PublishedDate    *time.Time
	LastModifiedDate *time.Time
	References       string // JSON string
	CWEIDs           []string
	Source           string
}

// ParsedPackageVulnerability represents a package vulnerability mapping
type ParsedPackageVulnerability struct {
	CVEID                 string
	PackageName           string
	Ecosystem             string
	EcosystemRelease      string // distro release the range applies to ("12" for Debian:12); "" when unscoped
	RangeType             string
	VersionStartIncluding string
	VersionStartExcluding string
	VersionEndIncluding   string
	VersionEndExcluding   string
	FixedVersion          string
	DatabaseSpecific      string // JSON string
}

// ParseFile parses a single OSV.dev JSON file
func ParseFile(path string) (*OSVVulnerability, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	var vuln OSVVulnerability
	if err := json.Unmarshal(data, &vuln); err != nil {
		return nil, fmt.Errorf("failed to parse JSON in %s: %w", path, err)
	}

	// Validate required fields
	if vuln.ID == "" {
		return nil, fmt.Errorf("missing ID in %s", path)
	}

	return &vuln, nil
}

// ConvertToCVE converts OSV vulnerability to CVE model
func ConvertToCVE(osv *OSVVulnerability) (*ParsedCVE, error) {
	// Parse dates
	var published, modified *time.Time
	if osv.Published != "" {
		t, err := time.Parse(time.RFC3339, osv.Published)
		if err == nil {
			published = &t
		}
	}
	if osv.Modified != "" {
		t, err := time.Parse(time.RFC3339, osv.Modified)
		if err == nil {
			modified = &t
		}
	}

	// Parse CVSS
	cvssScore, cvssVector, cvssVersion, severity := parseCVSS(osv.Severity)

	// Build references JSON
	refsJSON := buildReferencesJSON(osv.References)

	// Extract CWE IDs
	cweIDs := extractCWEIDs(osv.DatabaseSpecific)

	// Determine title
	title := osv.Summary
	if title == "" && osv.Details != "" {
		// Use first line of details as title
		lines := strings.Split(osv.Details, "\n")
		if len(lines) > 0 {
			title = lines[0]
			if len(title) > 200 {
				title = title[:200] + "..."
			}
		}
	}

	return &ParsedCVE{
		CVEID:            osv.ID,
		CVSSScore:        cvssScore,
		CVSSVector:       cvssVector,
		CVSSVersion:      cvssVersion,
		Severity:         severity,
		Title:            sanitizeUTF8(title),
		Description:      sanitizeUTF8(osv.Details),
		PublishedDate:    published,
		LastModifiedDate: modified,
		References:       refsJSON,
		CWEIDs:           cweIDs,
		Source:           "osv",
	}, nil
}

// ConvertToPackageVulnerabilities converts OSV affected packages to package vulnerabilities.
//
// Each OSV range is a sequence of events; an `introduced` event opens an affected interval and
// the next `fixed` or `last_affected` closes it. An interval left open (no fix yet) is kept as
// ">= introduced". Explicit `versions` are used when an entry has no SEMVER/ECOSYSTEM range.
// Withdrawn advisories produce no rows. Distro releases (Debian:12, Alpine:v3.20, …) are kept in
// EcosystemRelease so one release's ranges are not applied to another.
func ConvertToPackageVulnerabilities(osv *OSVVulnerability) ([]*ParsedPackageVulnerability, error) {
	var result []*ParsedPackageVulnerability
	if osv == nil || strings.TrimSpace(osv.Withdrawn) != "" {
		return result, nil
	}

	for _, affected := range osv.Affected {
		// Skip if no package info
		if affected.Package.Name == "" || affected.Package.Ecosystem == "" {
			continue
		}
		base := ParsedPackageVulnerability{
			CVEID:            osv.ID,
			PackageName:      affected.Package.Name,
			Ecosystem:        normalizeEcosystem(affected.Package.Ecosystem),
			EcosystemRelease: OSVEcosystemRelease(affected.Package.Ecosystem),
		}
		if affected.DatabaseSpecific != nil {
			dbSpecJSON, _ := json.Marshal(affected.DatabaseSpecific)
			base.DatabaseSpecific = string(dbSpecJSON)
		}

		versionRanges := 0
		for _, r := range affected.Ranges {
			rt := strings.ToUpper(strings.TrimSpace(r.Type))
			// GIT ranges carry commit hashes, not package versions.
			if rt != "SEMVER" && rt != "ECOSYSTEM" {
				continue
			}
			versionRanges++
			for _, iv := range OSVRangeIntervals(r.Events) {
				pv := base
				pv.RangeType = rt
				pv.VersionStartIncluding = iv.Introduced
				switch {
				case iv.Fixed != "":
					pv.VersionEndExcluding = iv.Fixed
					pv.FixedVersion = iv.Fixed
				case iv.LastAffected != "":
					pv.VersionEndIncluding = iv.LastAffected
				}
				result = append(result, &pv)
			}
		}

		if versionRanges == 0 {
			for _, v := range uniqueVersions(affected.Versions, maxExplicitVersionsPerPackage) {
				pv := base
				pv.RangeType = "EXPLICIT"
				pv.VersionStartIncluding = v
				pv.VersionEndIncluding = v
				result = append(result, &pv)
			}
		}
	}

	return result, nil
}

// maxExplicitVersionsPerPackage bounds rows created from an OSV `versions` list.
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
// "AlmaLinux:9" → "9". Ecosystems without a release, or with a format we do not parse
// (Red Hat CPE-style suffixes), return "".
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

// NormalizeDistroRelease reduces a distro version to the granularity advisories are published
// at: the major version for Debian, Rocky and Alma ("12.5" → "12"), major.minor for Alpine and
// Ubuntu ("v3.20.3" → "3.20", "22.04" → "22.04"). Unknown distros return the trimmed version.
func NormalizeDistroRelease(distro, version string) string {
	v := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(version)), "v")
	if v == "" {
		return ""
	}
	parts := strings.Split(v, ".")
	switch strings.ToLower(strings.TrimSpace(distro)) {
	case "debian", "rocky", "rocky linux", "alma", "almalinux":
		return parts[0]
	case "alpine", "ubuntu":
		if len(parts) >= 2 {
			return parts[0] + "." + parts[1]
		}
		return parts[0]
	}
	return v
}

// parseCVSS extracts CVSS score and severity from OSV severity data
func parseCVSS(severities []OSVSeverity) (score float64, vector, version, severity string) {
	for _, sev := range severities {
		if sev.Type == "CVSS_V3" || sev.Type == "CVSS_V2" {
			vector = sev.Score
			version = strings.Replace(sev.Type, "CVSS_", "", 1)

			// Parse score from CVSS vector
			// Format: CVSS:3.1/AV:N/AC:L/PR:N/UI:R/S:U/C:L/I:L/A:L
			if strings.HasPrefix(vector, "CVSS:") {
				parts := strings.Split(vector, "/")
				if len(parts) > 1 {
					// Calculate approximate score based on metrics
					score = calculateCVSSScore(parts[1:])
					severity = scoreToseverity(score)
				}
			}
			break
		}
	}

	// Default to MEDIUM if no severity found
	if severity == "" {
		severity = "MEDIUM"
		score = 5.0
	}

	return score, vector, version, severity
}

// calculateCVSSScore provides approximate CVSS score calculation
func calculateCVSSScore(metrics []string) float64 {
	// Simplified scoring logic
	// Full CVSS calculation is complex, this is an approximation
	baseScore := 5.0 // Default

	for _, metric := range metrics {
		parts := strings.Split(metric, ":")
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		val := parts[1]

		switch key {
		case "AV": // Attack Vector
			if val == "N" {
				baseScore += 1.5 // Network
			} else if val == "A" {
				baseScore += 1.0 // Adjacent
			}
		case "AC": // Attack Complexity
			if val == "L" {
				baseScore += 1.0 // Low
			}
		case "PR": // Privileges Required
			if val == "N" {
				baseScore += 1.5 // None
			}
		case "UI": // User Interaction
			if val == "N" {
				baseScore += 0.5 // None
			}
		case "C", "I", "A": // Confidentiality, Integrity, Availability
			if val == "H" {
				baseScore += 1.0 // High
			} else if val == "L" {
				baseScore += 0.5 // Low
			}
		}
	}

	// Cap at 10.0
	if baseScore > 10.0 {
		baseScore = 10.0
	}

	return baseScore
}

// scoreToseverity converts CVSS score to severity level
func scoreToseverity(score float64) string {
	if score >= 9.0 {
		return "CRITICAL"
	} else if score >= 7.0 {
		return "HIGH"
	} else if score >= 4.0 {
		return "MEDIUM"
	}
	return "LOW"
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
