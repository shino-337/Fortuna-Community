package nvd

import "time"

// APIResponse represents the NVD API 2.0 response envelope.
type APIResponse struct {
	ResultsPerPage  int              `json:"resultsPerPage"`
	StartIndex      int              `json:"startIndex"`
	TotalResults    int              `json:"totalResults"`
	Format          string           `json:"format"`
	Version         string           `json:"version"`
	Timestamp       string           `json:"timestamp"`
	Vulnerabilities []VulnWrapper    `json:"vulnerabilities"`
}

type VulnWrapper struct {
	CVE CVEItem `json:"cve"`
}

type CVEItem struct {
	ID              string       `json:"id"`
	SourceID        string       `json:"sourceIdentifier"`
	Published       string       `json:"published"`
	LastModified    string       `json:"lastModified"`
	VulnStatus      string       `json:"vulnStatus"`
	Descriptions    []LangString `json:"descriptions"`
	Metrics         *Metrics     `json:"metrics,omitempty"`
	Weaknesses      []Weakness   `json:"weaknesses,omitempty"`
	Configurations  []Config     `json:"configurations,omitempty"`
	References      []Reference  `json:"references,omitempty"`
}

type LangString struct {
	Lang  string `json:"lang"`
	Value string `json:"value"`
}

type Metrics struct {
	CvssMetricV31 []CVSSMetric `json:"cvssMetricV31,omitempty"`
	CvssMetricV30 []CVSSMetric `json:"cvssMetricV30,omitempty"`
	CvssMetricV2  []CVSSMetric `json:"cvssMetricV2,omitempty"`
}

type CVSSMetric struct {
	Source   string   `json:"source"`
	Type     string   `json:"type"`
	CVSSData CVSSData `json:"cvssData"`
}

type CVSSData struct {
	Version      string  `json:"version"`
	VectorString string  `json:"vectorString"`
	BaseScore    float64 `json:"baseScore"`
	BaseSeverity string  `json:"baseSeverity,omitempty"`
}

type Weakness struct {
	Source      string       `json:"source"`
	Type        string       `json:"type"`
	Description []LangString `json:"description"`
}

type Config struct {
	Operator string      `json:"operator,omitempty"`
	Negate   bool        `json:"negate,omitempty"`
	Nodes    []MatchNode `json:"nodes"`
}

type MatchNode struct {
	Operator string     `json:"operator,omitempty"`
	Negate   bool       `json:"negate,omitempty"`
	CPEMatch []CPEMatch `json:"cpeMatch"`
}

type CPEMatch struct {
	Vulnerable            bool   `json:"vulnerable"`
	Criteria              string `json:"criteria"`
	MatchCriteriaID       string `json:"matchCriteriaId,omitempty"`
	VersionStartIncluding string `json:"versionStartIncluding,omitempty"`
	VersionStartExcluding string `json:"versionStartExcluding,omitempty"`
	VersionEndIncluding   string `json:"versionEndIncluding,omitempty"`
	VersionEndExcluding   string `json:"versionEndExcluding,omitempty"`
}

type Reference struct {
	URL    string   `json:"url"`
	Source string   `json:"source,omitempty"`
	Tags   []string `json:"tags,omitempty"`
}

// SyncState persists incremental sync progress.
type SyncState struct {
	LastModified time.Time
	TotalSynced  int64
}
