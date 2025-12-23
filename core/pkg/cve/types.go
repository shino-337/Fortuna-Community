package cve

import "time"

// CVE represents a Common Vulnerability and Exposure
type CVE struct {
	ID            string
	Description   string
	Severity      string // CRITICAL, HIGH, MEDIUM, LOW
	CVSSScore     float64
	CVSSVector    string
	Constraint    string // Version constraint, e.g., "< 1.1.1l"
	FixedVersion  string
	Published     time.Time
	Modified      time.Time
	References    []string
}



