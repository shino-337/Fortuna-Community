package loader

import (
	"encoding/json"
	"fmt"
	"log"
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
	Type   string      `json:"type"` // SEMVER, ECOSYSTEM, GIT
	Repo   string      `json:"repo,omitempty"`
	Events []OSVEvent  `json:"events,omitempty"`
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
	RangeType             string
	VersionStartIncluding string
	VersionStartExcluding string
	VersionEndIncluding   string
	VersionEndExcluding   string
	FixedVersion          string
	AffectedVersions      []string
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

// ConvertToPackageVulnerabilities converts OSV affected packages to package vulnerabilities
func ConvertToPackageVulnerabilities(osv *OSVVulnerability) ([]*ParsedPackageVulnerability, error) {
	var result []*ParsedPackageVulnerability

	for _, affected := range osv.Affected {
		// Skip if no package info
		if affected.Package.Name == "" || affected.Package.Ecosystem == "" {
			continue
		}

		// Process version ranges
		for _, r := range affected.Ranges {
			// Skip GIT ranges (not useful for version matching)
			if r.Type == "GIT" {
				continue
			}

			for _, event := range r.Events {
				// Create vulnerability entry for each fix event
				if event.Fixed != "" {
					pv := &ParsedPackageVulnerability{
						CVEID:       osv.ID,
						PackageName: affected.Package.Name,
						Ecosystem:   normalizeEcosystem(affected.Package.Ecosystem),
						RangeType:   r.Type,
					}

					// Set version ranges
					if event.Introduced != "" {
						if event.Introduced == "0" {
							pv.VersionStartIncluding = "0"
						} else {
							pv.VersionStartIncluding = event.Introduced
						}
					}

					pv.VersionEndExcluding = event.Fixed
					pv.FixedVersion = event.Fixed

					// Add affected versions if available
					if len(affected.Versions) > 0 {
						pv.AffectedVersions = affected.Versions
					}

					// Add database-specific data
					if affected.DatabaseSpecific != nil {
						dbSpecJSON, _ := json.Marshal(affected.DatabaseSpecific)
						pv.DatabaseSpecific = string(dbSpecJSON)
					}

					result = append(result, pv)
				}

				// Handle last_affected
				if event.LastAffected != "" {
					pv := &ParsedPackageVulnerability{
						CVEID:       osv.ID,
						PackageName: affected.Package.Name,
						Ecosystem:   normalizeEcosystem(affected.Package.Ecosystem),
						RangeType:   r.Type,
					}

					if event.Introduced != "" {
						pv.VersionStartIncluding = event.Introduced
					}
					pv.VersionEndIncluding = event.LastAffected

					result = append(result, pv)
				}
			}
		}

		// If no ranges but has explicit versions, create entries for each
		if len(affected.Ranges) == 0 && len(affected.Versions) > 0 {
			pv := &ParsedPackageVulnerability{
				CVEID:            osv.ID,
				PackageName:      affected.Package.Name,
				Ecosystem:        normalizeEcosystem(affected.Package.Ecosystem),
				RangeType:        "EXPLICIT",
				AffectedVersions: affected.Versions,
			}
			result = append(result, pv)
		}
	}

	return result, nil
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
	TotalFiles            int
	SuccessfullyParsed    int
	FailedToParse         int
	CVEsCreated           int
	PackageVulnsCreated   int
	SkippedNoPackageInfo  int
}

// LogProgress logs parsing progress
func LogProgress(stats *Stats, filename string) {
	if stats.TotalFiles%1000 == 0 {
		log.Printf("Progress: %d/%d files processed, %d CVEs, %d package vulns",
			stats.SuccessfullyParsed, stats.TotalFiles,
			stats.CVEsCreated, stats.PackageVulnsCreated)
	}
}
