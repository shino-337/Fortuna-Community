package cve

import "time"

// CVE represents a Common Vulnerability and Exposure
type CVE struct {
	ID           string
	Description  string
	Severity     string // CRITICAL, HIGH, MEDIUM, LOW
	CVSSScore    float64
	CVSSVector   string
	Constraint   string // Version constraint, e.g., "< 1.1.1l"
	FixedVersion string
	// Release is the distro release the range applies to ("12" for Debian:12); "" when the
	// advisory is not scoped to a release.
	Release    string
	Published  time.Time
	Modified   time.Time
	References []string

	// AdvisoryID is the advisory the range comes from when ID is the canonical CVE it was
	// grouped under ("" when ID is the advisory itself).
	AdvisoryID string
	// SeverityTier orders where Severity came from (lower is better, 0 = not ranked);
	// SeveritySource names it (vendor, advisory_cvss, nvd, cve_cvss, errata_cvss, default).
	SeverityTier   int
	SeveritySource string
}
