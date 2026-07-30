package osv

// Minimal OSV JSON types for mirror ingestion.
// We intentionally model only the fields needed for mirror tables + matching.

type Document struct {
	ID      string   `json:"id"`
	Summary string   `json:"summary,omitempty"`
	Details string   `json:"details,omitempty"`
	Aliases []string `json:"aliases,omitempty"`

	Published string `json:"published,omitempty"`
	Modified  string `json:"modified,omitempty"`

	Severity []Severity `json:"severity,omitempty"`
	Affected []Affected `json:"affected,omitempty"`
}

type Severity struct {
	Type  string `json:"type"`  // CVSS_V2 / CVSS_V3
	Score string `json:"score"` // vector string
}

type Affected struct {
	Package Package `json:"package,omitempty"`
	Ranges  []Range `json:"ranges,omitempty"`
}

type Package struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
}

type Range struct {
	Type   string  `json:"type"` // SEMVER / ECOSYSTEM / GIT
	Events []Event `json:"events,omitempty"`
}

type Event struct {
	Introduced   string `json:"introduced,omitempty"`
	Fixed        string `json:"fixed,omitempty"`
	LastAffected string `json:"last_affected,omitempty"`
}

