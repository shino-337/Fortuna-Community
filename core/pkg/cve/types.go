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
}
